package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/unkeyed/unkey/internal/services/auditlogs"
	"github.com/unkeyed/unkey/pkg/auditlog"
	"github.com/unkeyed/unkey/pkg/codes"
	"github.com/unkeyed/unkey/pkg/db"
	"github.com/unkeyed/unkey/pkg/fault"
	"github.com/unkeyed/unkey/pkg/rbac"
	"github.com/unkeyed/unkey/pkg/uid"
	"github.com/unkeyed/unkey/pkg/zen"
	"github.com/unkeyed/unkey/svc/api/internal/policy"
	"github.com/unkeyed/unkey/svc/api/openapi"
)

// maxPoliciesPerEnvironment mirrors SENTINEL_LIMITS.maxPolicies in the
// dashboard's sentinel-policies schema; stored policies of any variant count
// toward it.
const maxPoliciesPerEnvironment = 10

type (
	Request  = openapi.V2PoliciesCreatePolicyRequestBody
	Response = openapi.V2PoliciesCreatePolicyResponseBody
)

type Handler struct {
	DB        db.Database
	Auditlogs auditlogs.AuditLogService
}

func (h *Handler) Method() string {
	return "POST"
}

func (h *Handler) Path() string {
	return "/v2/policies.createPolicy"
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
			Action:       rbac.CreatePolicy,
		}),
		rbac.T(rbac.Tuple{
			ResourceType: rbac.Environment,
			ResourceID:   env.ID,
			Action:       rbac.CreatePolicy,
		}),
	))
	if err != nil {
		return err
	}

	if err = policy.ValidatePolicies(req.Policies); err != nil {
		return err
	}

	if err = h.assertKeyspacesOwned(ctx, principal.WorkspaceID, req.Policies); err != nil {
		return err
	}

	ids := make([]string, len(req.Policies))
	for i := range ids {
		ids[i] = uid.New(uid.PolicyPrefix)
	}

	newRaw, err := policy.MarshalPolicies(req.Policies, ids)
	if err == nil {
		err = policy.AssertWireCompatible(newRaw)
	}
	if err != nil {
		return fault.Wrap(
			err,
			fault.Code(codes.App.Internal.UnexpectedError.URN()),
			fault.Internal("policy serialization produced gateway-incompatible JSON"),
			fault.Public("We're unable to create the policies."),
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
				fault.Public("We're unable to create the policies."),
			)
		}

		var existing []json.RawMessage
		settings, findErr := db.Query.FindAppRuntimeSettingsByAppAndEnv(ctx, tx, db.FindAppRuntimeSettingsByAppAndEnvParams{
			AppID:         env.AppID,
			EnvironmentID: env.ID,
		})
		switch {
		case findErr == nil:
			existing, findErr = policy.ParseStoredPolicies(settings.AppRuntimeSetting.SentinelConfig)
			if findErr != nil {
				return fault.Wrap(
					findErr,
					fault.Code(codes.App.Internal.UnexpectedError.URN()),
					fault.Internal("stored sentinel config is corrupted"),
					fault.Public("The stored policy configuration is corrupted. Please contact support."),
				)
			}
		case db.IsNotFound(findErr):
			existing = nil
		default:
			return fault.Wrap(
				findErr,
				fault.Code(codes.App.Internal.ServiceUnavailable.URN()),
				fault.Internal("unable to load runtime settings"),
				fault.Public("We're unable to create the policies."),
			)
		}

		if len(existing)+len(req.Policies) > maxPoliciesPerEnvironment {
			return fault.New(
				"too many policies",
				fault.Code(codes.App.Validation.InvalidInput.URN()),
				fault.Internal("policy limit exceeded"),
				fault.Public(fmt.Sprintf(
					"An environment can have at most %d policies; it has %d and this request adds %d.",
					maxPoliciesPerEnvironment, len(existing), len(req.Policies),
				)),
			)
		}

		blob, blobErr := policy.BuildBlob(existing, newRaw)
		if blobErr == nil {
			blobErr = policy.AssertParseable(blob)
		}
		if blobErr != nil {
			return fault.Wrap(
				blobErr,
				fault.Code(codes.App.Internal.UnexpectedError.URN()),
				fault.Internal("merged sentinel config is not parseable by the gateway"),
				fault.Public("We're unable to create the policies."),
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
				fault.Public("We're unable to create the policies."),
			)
		}

		auditLogs := make([]auditlog.AuditLog, 0, len(req.Policies))
		for i, p := range req.Policies {
			auditLogs = append(auditLogs, auditlog.AuditLog{
				WorkspaceID:   principal.WorkspaceID,
				Event:         auditlog.EnvironmentUpdateEvent,
				Display:       fmt.Sprintf("Created policy %s (%s) for environment %s", p.Name, ids[i], env.ID),
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
						Meta:        map[string]any{"policyId": ids[i], "policyType": variantName(p)},
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

// assertKeyspacesOwned rejects keyauth policies referencing keyspaces outside
// the workspace. Ownership never changes, so checking outside the write
// transaction cannot race with it.
func (h *Handler) assertKeyspacesOwned(ctx context.Context, workspaceID string, policies []openapi.Policy) error {
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
			fault.Public("We're unable to create the policies."),
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

func variantName(p openapi.Policy) string {
	switch {
	case p.Keyauth != nil:
		return "keyauth"
	case p.Ratelimit != nil:
		return "ratelimit"
	case p.Firewall != nil:
		return "firewall"
	case p.Openapi != nil:
		return "openapi"
	default:
		return "unknown"
	}
}
