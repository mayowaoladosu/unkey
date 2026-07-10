package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	frontlinev1 "github.com/unkeyed/unkey/gen/proto/frontline/v1"
	"github.com/unkeyed/unkey/internal/services/auditlogs"
	"github.com/unkeyed/unkey/pkg/auditlog"
	"github.com/unkeyed/unkey/pkg/codes"
	"github.com/unkeyed/unkey/pkg/db"
	"github.com/unkeyed/unkey/pkg/fault"
	"github.com/unkeyed/unkey/pkg/rbac"
	"github.com/unkeyed/unkey/pkg/uid"
	"github.com/unkeyed/unkey/pkg/zen"
	"github.com/unkeyed/unkey/svc/api/openapi"
	"google.golang.org/protobuf/encoding/protojson"
)

type (
	Request  = openapi.V2PoliciesSetPoliciesRequestBody
	Response = openapi.V2PoliciesSetPoliciesResponseBody
)

type Handler struct {
	DB        db.Database
	Auditlogs auditlogs.AuditLogService
}

// The sentinel_config blob is protojson for frontlinev1.Config and is read
// back by the dashboard through a strict schema. Both contracts must hold:
// no unknown fields, required fields present even when zero (enabled=false).
type configEnvelope struct {
	Policies []json.RawMessage `json:"policies"`
}

// wirePolicy marshals a policy flat with its server-generated id.
type wirePolicy struct {
	ID string `json:"id"`
	openapi.Policy
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

	if err = validatePolicies(req.Policies); err != nil {
		return err
	}

	if err = h.validateKeyspaceOwnership(ctx, principal.WorkspaceID, req.Policies); err != nil {
		return err
	}

	blob, ids, err := encodePolicies(req.Policies)
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

		newLog := func(display string, meta map[string]any) auditlog.AuditLog {
			return auditlog.AuditLog{
				WorkspaceID:   principal.WorkspaceID,
				Event:         auditlog.EnvironmentUpdateEvent,
				Display:       display,
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
						Meta:        meta,
						Name:        env.Slug,
						DisplayName: env.Slug,
					},
				},
			}
		}

		auditLogs := make([]auditlog.AuditLog, 0, len(req.Policies)+1)
		for i, p := range req.Policies {
			auditLogs = append(auditLogs, newLog(
				fmt.Sprintf("Set policy %s (%s) for environment %s", p.Name, ids[i], env.ID),
				map[string]any{"policyId": ids[i], "policyType": variantName(p)},
			))
		}

		// An empty request wipes everything but yields no per-policy logs,
		// so record the destructive action on its own.
		if len(auditLogs) == 0 {
			auditLogs = append(auditLogs, newLog(
				fmt.Sprintf("Removed all policies for environment %s", env.ID),
				map[string]any{},
			))
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

func (h *Handler) validateKeyspaceOwnership(ctx context.Context, workspaceID string, policies []openapi.Policy) error {
	var keyspaceIDs []string
	for _, p := range policies {
		if p.Keyauth != nil {
			keyspaceIDs = append(keyspaceIDs, p.Keyauth.Keyspaces...)
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

// encodePolicies serializes the request into the blob to store, generating
// an id per policy; ids[i] belongs to policies[i]. The blob must strictly
// parse as frontlinev1.Config: a failure is our serialization bug, never a
// user error.
func encodePolicies(policies []openapi.Policy) (blob []byte, ids []string, err error) {
	ids = make([]string, 0, len(policies))
	raws := make([]json.RawMessage, 0, len(policies))
	for _, p := range policies {
		id := uid.New(uid.PolicyPrefix)
		raw, marshalErr := json.Marshal(wirePolicy{ID: id, Policy: p})
		if marshalErr != nil {
			return nil, nil, fmt.Errorf("marshal policy %q: %w", id, marshalErr)
		}

		// The API says `keyspaces`, the proto and stored blobs say
		// `keySpaceIds`; rename the key on the way in.
		if p.Keyauth != nil {
			var doc, keyauth map[string]json.RawMessage
			if err = json.Unmarshal(raw, &doc); err != nil {
				return nil, nil, fmt.Errorf("rewrite policy %q: %w", id, err)
			}
			if err = json.Unmarshal(doc["keyauth"], &keyauth); err != nil {
				return nil, nil, fmt.Errorf("rewrite policy %q: %w", id, err)
			}
			keyauth["keySpaceIds"] = keyauth["keyspaces"]
			delete(keyauth, "keyspaces")
			if doc["keyauth"], err = json.Marshal(keyauth); err != nil {
				return nil, nil, fmt.Errorf("rewrite policy %q: %w", id, err)
			}
			if raw, err = json.Marshal(doc); err != nil {
				return nil, nil, fmt.Errorf("rewrite policy %q: %w", id, err)
			}
		}

		ids = append(ids, id)
		raws = append(raws, raw)
	}

	blob, err = json.Marshal(configEnvelope{Policies: raws})
	if err != nil {
		return nil, nil, err
	}
	if err = protojson.Unmarshal(blob, &frontlinev1.Config{}); err != nil {
		return nil, nil, fmt.Errorf("encoded policies are not gateway-compatible: %w", err)
	}
	return blob, ids, nil
}
