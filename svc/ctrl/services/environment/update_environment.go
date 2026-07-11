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
	"github.com/unkeyed/unkey/svc/ctrl/internal/actor"
	"github.com/unkeyed/unkey/svc/ctrl/internal/auth"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
)

func (s *Service) UpdateEnvironment(ctx context.Context, req *connect.Request[ctrlv1.UpdateEnvironmentRequest]) (*connect.Response[ctrlv1.UpdateEnvironmentResponse], error) {
	if err := auth.Authenticate(req, s.bearer); err != nil {
		return nil, err
	}
	if err := assert.All(
		assert.NotEmpty(req.Msg.GetEnvironmentId(), "environment_id is required"),
		assert.NotEmpty(req.Msg.GetSlug(), "slug is required"),
		assert.NotNil(req.Msg.GetActor(), "actor is required"),
	); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	now := time.Now().UnixMilli()
	err := db.TxRetry(ctx, s.db.RW(), func(txCtx context.Context, tx db.DBTX) error {
		queries := db.NewQueries(tx)
		env, findErr := queries.FindEnvironmentById(txCtx, req.Msg.GetEnvironmentId())
		if findErr != nil {
			return findErr
		}
		if updateErr := queries.UpdateEnvironment(txCtx, db.UpdateEnvironmentParams{
			Slug: req.Msg.GetSlug(), Description: req.Msg.GetDescription(),
			UpdatedAt: sql.NullInt64{Int64: now, Valid: true}, ID: env.ID,
		}); updateErr != nil {
			return updateErr
		}
		a := req.Msg.GetActor()
		return s.auditlogs.Insert(txCtx, tx, []auditlog.AuditLog{{
			WorkspaceID: env.WorkspaceID, Event: auditlog.EnvironmentUpdateEvent,
			Display: fmt.Sprintf("Updated environment %s", req.Msg.GetSlug()),
			ActorID: a.GetId(), ActorName: a.GetName(), ActorType: actor.AuditType(a.GetType()),
			ActorMeta: actor.Meta(a.GetMeta()), RemoteIP: a.GetRemoteIp(), UserAgent: a.GetUserAgent(),
			Resources: []auditlog.AuditLogResource{{
				ID: env.ID, Type: auditlog.EnvironmentResourceType,
				Meta: map[string]any{"slug": req.Msg.GetSlug(), "previousSlug": env.Slug, "appId": env.AppID, "projectId": env.ProjectID},
				Name: req.Msg.GetSlug(), DisplayName: req.Msg.GetSlug(),
			}},
		}})
	})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("environment not found"))
		}
		if db.IsDuplicateKeyError(err) {
			return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("environment with slug %q already exists", req.Msg.GetSlug()))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("update environment: %w", err))
	}
	return connect.NewResponse(&ctrlv1.UpdateEnvironmentResponse{}), nil
}

func (s *Service) SetEnvironmentDeleteProtection(ctx context.Context, req *connect.Request[ctrlv1.SetEnvironmentDeleteProtectionRequest]) (*connect.Response[ctrlv1.SetEnvironmentDeleteProtectionResponse], error) {
	if err := auth.Authenticate(req, s.bearer); err != nil {
		return nil, err
	}
	if err := assert.All(
		assert.NotEmpty(req.Msg.GetEnvironmentId(), "environment_id is required"),
		assert.NotNil(req.Msg.GetActor(), "actor is required"),
	); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err := db.TxRetry(ctx, s.db.RW(), func(txCtx context.Context, tx db.DBTX) error {
		queries := db.NewQueries(tx)
		env, findErr := queries.FindEnvironmentById(txCtx, req.Msg.GetEnvironmentId())
		if findErr != nil {
			return findErr
		}
		if updateErr := queries.UpdateEnvironmentDeleteProtection(txCtx, db.UpdateEnvironmentDeleteProtectionParams{
			DeleteProtection: sql.NullBool{Bool: req.Msg.GetEnabled(), Valid: true},
			UpdatedAt:        sql.NullInt64{Int64: time.Now().UnixMilli(), Valid: true},
			ID:               env.ID,
		}); updateErr != nil {
			return updateErr
		}
		a := req.Msg.GetActor()
		state := "disabled"
		if req.Msg.GetEnabled() {
			state = "enabled"
		}
		return s.auditlogs.Insert(txCtx, tx, []auditlog.AuditLog{{
			WorkspaceID: env.WorkspaceID, Event: auditlog.EnvironmentUpdateEvent,
			Display: fmt.Sprintf("Delete protection %s for environment %s", state, env.Slug),
			ActorID: a.GetId(), ActorName: a.GetName(), ActorType: actor.AuditType(a.GetType()),
			ActorMeta: actor.Meta(a.GetMeta()), RemoteIP: a.GetRemoteIp(), UserAgent: a.GetUserAgent(),
			Resources: []auditlog.AuditLogResource{{
				ID: env.ID, Type: auditlog.EnvironmentResourceType,
				Meta: map[string]any{"slug": env.Slug, "deleteProtection": req.Msg.GetEnabled(), "appId": env.AppID, "projectId": env.ProjectID},
				Name: env.Slug, DisplayName: env.Slug,
			}},
		}})
	})
	if err != nil {
		if db.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("environment not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("update delete protection: %w", err))
	}
	return connect.NewResponse(&ctrlv1.SetEnvironmentDeleteProtectionResponse{}), nil
}