package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/unkeyed/unkey/internal/services/auditlogs"
	"github.com/unkeyed/unkey/pkg/auditlog"
	"github.com/unkeyed/unkey/pkg/codes"
	"github.com/unkeyed/unkey/pkg/db"
	"github.com/unkeyed/unkey/pkg/fault"
	"github.com/unkeyed/unkey/pkg/ptr"
	"github.com/unkeyed/unkey/pkg/rbac"
	"github.com/unkeyed/unkey/pkg/zen"
	"github.com/unkeyed/unkey/svc/api/openapi"
)

// maxPoliciesPerEnvironment mirrors the dashboard's SENTINEL_LIMITS.maxPolicies;
// every stored variant counts toward it.
const maxPoliciesPerEnvironment = 10

type (
	Request  = openapi.V2PoliciesSetPoliciesRequestBody
	Response = openapi.V2PoliciesSetPoliciesResponseBody
)

type Handler struct {
	DB        db.Database
	Auditlogs auditlogs.AuditLogService
}

func (h *Handler) Method() string {
	return "POST"
}

func (h *Handler) Path() string {
	return "/v2/policies.setPolicies"
}

func (h *Handler) Handle(ctx context.Context, s *zen.Session) error {
	principal, err := s.GetPrincipal()
	if err != nil {
		return err
	}

	req, err := zen.BindBody[Request](s)
	if err != nil {
		return err
	}

	env, err := db.Query.FindEnvironmentByIdentifiers(ctx, h.DB.RO(), db.FindEnvironmentByIdentifiersParams{
		WorkspaceID: principal.WorkspaceID,
		Project:     req.Project,
		App:         req.App,
		Environment: req.Environment,
	})
	if err != nil {
		if db.IsNotFound(err) {
			return fault.New(
				"environment not found",
				fault.Code(codes.Data.Environment.NotFound.URN()),
				fault.Internal("environment not found"),
				fault.Public("The requested environment does not exist."),
			)
		}
		return fault.Wrap(
			err,
			fault.Code(codes.App.Internal.ServiceUnavailable.URN()),
			fault.Internal("database error"),
			fault.Public("Failed to retrieve environment."),
		)
	}

	err = principal.Authorize(rbac.Or(
		rbac.T(rbac.Tuple{
			ResourceType: rbac.Environment,
			ResourceID:   "*",
			Action:       rbac.SetPolicies,
		}),
		rbac.T(rbac.Tuple{
			ResourceType: rbac.Environment,
			ResourceID:   env.ID,
			Action:       rbac.SetPolicies,
		}),
	))
	if err != nil {
		return err
	}

	updateIDs := make(map[string]struct{}, len(req.Policies))
	for _, p := range req.Policies {
		if p.Id == nil {
			continue
		}
		if _, dup := updateIDs[*p.Id]; dup {
			return fault.New(
				"duplicate policy id",
				fault.Code(codes.App.Validation.InvalidInput.URN()),
				fault.Internal("duplicate policy id in request"),
				fault.Public(fmt.Sprintf("Policy id %q is listed more than once. Each id may appear at most once.", *p.Id)),
			)
		}
		updateIDs[*p.Id] = struct{}{}
	}

	if err = validatePolicies(req.Policies); err != nil {
		return err
	}

	if err = h.validateKeyspaceOwnership(ctx, principal.WorkspaceID, req.Policies); err != nil {
		return err
	}

	prune := ptr.SafeDeref(req.Prune, false)
	if !prune && len(req.Policies) == 0 {
		return s.JSON(http.StatusOK, Response{
			Meta: openapi.Meta{RequestId: s.RequestID()},
			Data: openapi.EmptyResponse{},
		})
	}

	incoming, err := encodePolicies(req.Policies)
	if err != nil {
		return fault.Wrap(
			err,
			fault.Code(codes.App.Internal.UnexpectedError.URN()),
			fault.Internal("policy serialization produced gateway-incompatible JSON"),
			fault.Public("We're unable to set the policies."),
		)
	}

	now := time.Now().UnixMilli()
	err = db.TxRetry(ctx, h.DB.RW(), func(ctx context.Context, tx db.DBTX) error {
		if _, lockErr := db.Query.LockEnvironmentForUpdate(ctx, tx, env.ID); lockErr != nil {
			if db.IsNotFound(lockErr) {
				return fault.New(
					"environment not found",
					fault.Code(codes.Data.Environment.NotFound.URN()),
					fault.Internal("environment deleted before lock"),
					fault.Public("The requested environment does not exist."),
				)
			}
			return fault.Wrap(
				lockErr,
				fault.Code(codes.App.Internal.ServiceUnavailable.URN()),
				fault.Internal("unable to lock environment"),
				fault.Public("We're unable to set the policies."),
			)
		}

		var stored []policyDoc
		sentinelConfig, findErr := db.Query.FindSentinelConfigByAppAndEnv(ctx, tx, db.FindSentinelConfigByAppAndEnvParams{
			AppID:         env.AppID,
			EnvironmentID: env.ID,
		})
		switch {
		case findErr == nil:
			stored, findErr = parseStoredPolicies(sentinelConfig)
			if findErr != nil {
				return fault.Wrap(
					findErr,
					fault.Code(codes.App.Internal.UnexpectedError.URN()),
					fault.Internal("stored sentinel config is corrupted"),
					fault.Public("The stored policy configuration is corrupted. Please contact support."),
				)
			}
		case db.IsNotFound(findErr):
			stored = nil
		default:
			return fault.Wrap(
				findErr,
				fault.Code(codes.App.Internal.ServiceUnavailable.URN()),
				fault.Internal("unable to load runtime settings"),
				fault.Public("We're unable to set the policies."),
			)
		}

		storedIDs := make(map[string]struct{}, len(stored))
		for _, doc := range stored {
			storedIDs[doc.ID] = struct{}{}
		}
		creates := 0
		for _, p := range req.Policies {
			if p.Id == nil {
				creates++
				continue
			}
			if _, ok := storedIDs[*p.Id]; !ok {
				return fault.New(
					"policy not found",
					fault.Code(codes.Data.Policy.NotFound.URN()),
					fault.Internal("policy id not found in environment"),
					fault.Public(fmt.Sprintf("Policy %q does not exist.", *p.Id)),
				)
			}
		}

		// Only creates grow the list; a prune result is the request itself.
		// Belt behind the schema's maxItems: an oversized blob bricks the
		// dashboard's strict reader.
		total := len(stored) + creates
		if prune {
			total = len(req.Policies)
		}
		if total > maxPoliciesPerEnvironment {
			return fault.New(
				"too many policies",
				fault.Code(codes.App.Validation.InvalidInput.URN()),
				fault.Internal("policy limit exceeded"),
				fault.Public(fmt.Sprintf(
					"An environment can have at most %d policies; this request would result in %d.",
					maxPoliciesPerEnvironment, total,
				)),
			)
		}

		blob, mergeErr := mergePolicies(stored, incoming, prune)
		if mergeErr != nil {
			return fault.Wrap(
				mergeErr,
				fault.Code(codes.App.Internal.UnexpectedError.URN()),
				fault.Internal("merged sentinel config is not parseable by the gateway"),
				fault.Public("We're unable to set the policies."),
			)
		}

		if upsertErr := db.Query.UpsertAppRuntimeSettingsSentinelConfig(ctx, tx, db.UpsertAppRuntimeSettingsSentinelConfigParams{
			WorkspaceID:    env.WorkspaceID,
			AppID:          env.AppID,
			EnvironmentID:  env.ID,
			SentinelConfig: blob,
			CreatedAt:      now,
			UpdatedAt:      sql.NullInt64{Valid: true, Int64: now},
		}); upsertErr != nil {
			return fault.Wrap(
				upsertErr,
				fault.Code(codes.App.Internal.ServiceUnavailable.URN()),
				fault.Internal("unable to write sentinel config"),
				fault.Public("We're unable to set the policies."),
			)
		}

		auditLogs := make([]auditlog.AuditLog, 0, len(req.Policies)+1)
		for i, p := range req.Policies {
			verb := "Updated"
			if p.Id == nil {
				verb = "Created"
			}
			auditLogs = append(auditLogs, auditlog.AuditLog{
				WorkspaceID:   principal.WorkspaceID,
				Event:         auditlog.EnvironmentUpdateEvent,
				Display:       fmt.Sprintf("%s policy %s (%s) for environment %s", verb, p.Name, incoming[i].ID, env.ID),
				ActorID:       principal.Subject.ID,
				ActorName:     principal.Subject.Name,
				ActorMeta:     map[string]any{},
				ActorType:     auditlog.AuditLogActor(principal.Subject.Type),
				RemoteIP:      s.Location(),
				UserAgent:     s.UserAgent(),
				CorrelationID: "",
				Resources: []auditlog.AuditLogResource{
					{
						ID:          env.ID,
						Type:        auditlog.EnvironmentResourceType,
						Meta:        map[string]any{"policyId": incoming[i].ID, "policyType": variantName(p), "prune": prune},
						Name:        env.Slug,
						DisplayName: env.Slug,
					},
				},
			})
		}

		// A prune with an empty payload wipes everything but yields no
		// per-policy logs, so record the destructive action on its own.
		if len(auditLogs) == 0 && prune {
			auditLogs = append(auditLogs, auditlog.AuditLog{
				WorkspaceID:   principal.WorkspaceID,
				Event:         auditlog.EnvironmentUpdateEvent,
				Display:       fmt.Sprintf("Pruned all policies for environment %s", env.ID),
				ActorID:       principal.Subject.ID,
				ActorName:     principal.Subject.Name,
				ActorMeta:     map[string]any{},
				ActorType:     auditlog.AuditLogActor(principal.Subject.Type),
				RemoteIP:      s.Location(),
				UserAgent:     s.UserAgent(),
				CorrelationID: "",
				Resources: []auditlog.AuditLogResource{
					{
						ID:          env.ID,
						Type:        auditlog.EnvironmentResourceType,
						Meta:        map[string]any{"prune": prune},
						Name:        env.Slug,
						DisplayName: env.Slug,
					},
				},
			})
		}

		return h.Auditlogs.Insert(ctx, tx, auditLogs)
	})
	if err != nil {
		return err
	}

	return s.JSON(http.StatusOK, Response{
		Meta: openapi.Meta{RequestId: s.RequestID()},
		Data: openapi.EmptyResponse{},
	})
}

// validateKeyspaceOwnership rejects keyauth policies referencing keyspaces
// outside the workspace. Ownership never changes, so checking outside the
// write transaction cannot race with it.
func (h *Handler) validateKeyspaceOwnership(ctx context.Context, workspaceID string, policies []openapi.Policy) error {
	var keyspaceIDs []string
	for _, p := range policies {
		if p.Keyauth == nil {
			continue
		}
		for _, id := range p.Keyauth.KeySpaceIds {
			if !slices.Contains(keyspaceIDs, id) {
				keyspaceIDs = append(keyspaceIDs, id)
			}
		}
	}
	if len(keyspaceIDs) == 0 {
		return nil
	}

	found, err := db.Query.FindKeyAuthsByIdsAndWorkspace(ctx, h.DB.RO(), db.FindKeyAuthsByIdsAndWorkspaceParams{
		WorkspaceID: workspaceID,
		KeyAuthIds:  keyspaceIDs,
	})
	if err != nil {
		return fault.Wrap(
			err,
			fault.Code(codes.App.Internal.ServiceUnavailable.URN()),
			fault.Internal("unable to verify keyspaces"),
			fault.Public("We're unable to set the policies."),
		)
	}

	for _, id := range keyspaceIDs {
		if !slices.Contains(found, id) {
			return fault.New(
				"keyspace not found",
				fault.Code(codes.Data.KeySpace.NotFound.URN()),
				fault.Internal("keyspace not found in workspace"),
				fault.Public(fmt.Sprintf("Keyspace %q does not exist.", id)),
			)
		}
	}
	return nil
}
