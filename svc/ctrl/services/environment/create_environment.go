package environment

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"connectrpc.com/connect"
	ctrlv1 "github.com/unkeyed/unkey/gen/proto/ctrl/v1"
	"github.com/unkeyed/unkey/pkg/assert"
	"github.com/unkeyed/unkey/pkg/auditlog"
	"github.com/unkeyed/unkey/pkg/uid"
	"github.com/unkeyed/unkey/svc/ctrl/internal/actor"
	"github.com/unkeyed/unkey/svc/ctrl/internal/auth"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
)

func (s *Service) CreateEnvironment(
	ctx context.Context,
	req *connect.Request[ctrlv1.CreateEnvironmentRequest],
) (*connect.Response[ctrlv1.CreateEnvironmentResponse], error) {
	if err := auth.Authenticate(req, s.bearer); err != nil {
		return nil, err
	}
	if err := assert.All(
		assert.NotEmpty(req.Msg.GetWorkspaceId(), "workspace_id is required"),
		assert.NotEmpty(req.Msg.GetProjectId(), "project_id is required"),
		assert.NotEmpty(req.Msg.GetAppId(), "app_id is required"),
		assert.NotEmpty(req.Msg.GetSlug(), "slug is required"),
		assert.NotEmpty(req.Msg.GetSourceEnvironmentId(), "source_environment_id is required"),
		assert.NotNil(req.Msg.GetActor(), "actor is required"),
	); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	environmentID := uid.New(uid.EnvironmentPrefix)
	now := time.Now().UnixMilli()
	err := db.TxRetry(ctx, s.db.RW(), func(txCtx context.Context, tx db.DBTX) error {
		queries := db.NewQueries(tx)
		source, findErr := queries.FindEnvironmentById(txCtx, req.Msg.GetSourceEnvironmentId())
		if findErr != nil {
			return fmt.Errorf("find source environment: %w", findErr)
		}
		if source.WorkspaceID != req.Msg.GetWorkspaceId() ||
			source.ProjectID != req.Msg.GetProjectId() ||
			source.AppID != req.Msg.GetAppId() {
			return connect.NewError(connect.CodeNotFound, fmt.Errorf("source environment not found"))
		}

		if insertErr := queries.InsertEnvironment(txCtx, db.InsertEnvironmentParams{
			ID:          environmentID,
			WorkspaceID: source.WorkspaceID,
			ProjectID:   source.ProjectID,
			AppID:       source.AppID,
			Slug:        req.Msg.GetSlug(),
			Description: req.Msg.GetDescription(),
			CreatedAt:   now,
			UpdatedAt:   sql.NullInt64{Valid: false},
		}); insertErr != nil {
			return fmt.Errorf("insert environment: %w", insertErr)
		}
		if req.Msg.GetDeleteProtection() {
			if protectErr := queries.UpdateEnvironmentDeleteProtection(txCtx, db.UpdateEnvironmentDeleteProtectionParams{
				DeleteProtection: sql.NullBool{Bool: true, Valid: true},
				UpdatedAt:        sql.NullInt64{Int64: now, Valid: true},
				ID:               environmentID,
			}); protectErr != nil {
				return fmt.Errorf("enable delete protection: %w", protectErr)
			}
		}

		updatedAt := sql.NullInt64{Int64: now, Valid: true}
		if cloneErr := queries.CloneAppBuildSettings(txCtx, db.CloneAppBuildSettingsParams{
			TargetEnvironmentID: environmentID,
			CreatedAt:           now,
			UpdatedAt:           updatedAt,
			AppID:               source.AppID,
			SourceEnvironmentID: source.ID,
		}); cloneErr != nil {
			return fmt.Errorf("clone build settings: %w", cloneErr)
		}
		if cloneErr := queries.CloneAppRuntimeSettings(txCtx, db.CloneAppRuntimeSettingsParams{
			TargetEnvironmentID: environmentID,
			CreatedAt:           now,
			UpdatedAt:           updatedAt,
			AppID:               source.AppID,
			SourceEnvironmentID: source.ID,
		}); cloneErr != nil {
			return fmt.Errorf("clone runtime settings: %w", cloneErr)
		}

		regional, regionalErr := queries.FindAppRegionalSettingsByAppAndEnv(txCtx, db.FindAppRegionalSettingsByAppAndEnvParams{
			AppID:         source.AppID,
			EnvironmentID: source.ID,
		})
		if regionalErr != nil {
			return fmt.Errorf("find source regional settings: %w", regionalErr)
		}
		policies := make(map[string]string)
		for _, region := range regional {
			policyID := sql.NullString{Valid: false}
			if region.HorizontalAutoscalingPolicyID.Valid {
				clonedPolicyID, exists := policies[region.HorizontalAutoscalingPolicyID.String]
				if !exists {
					clonedPolicyID = uid.New(uid.AutoscalingPolicyPrefix)
					if insertErr := queries.InsertHorizontalAutoscalingPolicy(txCtx, db.InsertHorizontalAutoscalingPolicyParams{
						ID:              clonedPolicyID,
						WorkspaceID:     source.WorkspaceID,
						ReplicasMin:     region.AutoscalingReplicasMin.Int32,
						ReplicasMax:     region.AutoscalingReplicasMax.Int32,
						MemoryThreshold: region.AutoscalingThresholdMemory,
						CpuThreshold:    region.AutoscalingThresholdCpu,
						RpsThreshold:    sql.NullInt16{Valid: false},
						CreatedAt:       now,
						UpdatedAt:       updatedAt,
					}); insertErr != nil {
						return fmt.Errorf("clone autoscaling policy: %w", insertErr)
					}
					policies[region.HorizontalAutoscalingPolicyID.String] = clonedPolicyID
				}
				policyID = sql.NullString{String: clonedPolicyID, Valid: true}
			}

			if insertErr := queries.InsertAppRegionalSettingsWithPolicy(txCtx, db.InsertAppRegionalSettingsWithPolicyParams{
				WorkspaceID:                   source.WorkspaceID,
				AppID:                         source.AppID,
				EnvironmentID:                 environmentID,
				RegionID:                      region.RegionID,
				Replicas:                      region.Replicas,
				HorizontalAutoscalingPolicyID: policyID,
				CreatedAt:                     now,
				UpdatedAt:                     updatedAt,
			}); insertErr != nil {
				return fmt.Errorf("clone regional settings: %w", insertErr)
			}
		}

		a := req.Msg.GetActor()
		if auditErr := s.auditlogs.Insert(txCtx, tx, []auditlog.AuditLog{{
			WorkspaceID: source.WorkspaceID,
			Event:       auditlog.EnvironmentCreateEvent,
			Display:     fmt.Sprintf("Created environment %s from %s", req.Msg.GetSlug(), source.Slug),
			ActorID:     a.GetId(),
			ActorName:   a.GetName(),
			ActorType:   actor.AuditType(a.GetType()),
			ActorMeta:   actor.Meta(a.GetMeta()),
			RemoteIP:    a.GetRemoteIp(),
			UserAgent:   a.GetUserAgent(),
			Resources: []auditlog.AuditLogResource{{
				ID: environmentID, Type: auditlog.EnvironmentResourceType,
				Meta: map[string]any{"slug": req.Msg.GetSlug(), "appId": source.AppID, "projectId": source.ProjectID, "sourceEnvironmentId": source.ID},
				Name: req.Msg.GetSlug(), DisplayName: req.Msg.GetSlug(),
			}},
		}}); auditErr != nil {
			return fmt.Errorf("insert audit log: %w", auditErr)
		}

		return nil
	})
	if err != nil {
		if db.IsDuplicateKeyError(err) {
			return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("environment with slug %q already exists", req.Msg.GetSlug()))
		}
		if connectErr, ok := err.(*connect.Error); ok {
			return nil, connectErr
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create environment: %w", err))
	}

	return connect.NewResponse(&ctrlv1.CreateEnvironmentResponse{Id: environmentID}), nil
}