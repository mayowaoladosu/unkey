package environment

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	ctrlv1 "github.com/unkeyed/unkey/gen/proto/ctrl/v1"
	hydrav1 "github.com/unkeyed/unkey/gen/proto/hydra/v1"
	"github.com/unkeyed/unkey/pkg/assert"
	"github.com/unkeyed/unkey/pkg/auditlog"
	"github.com/unkeyed/unkey/svc/ctrl/internal/auth"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
)

func (s *Service) DeleteEnvironment(ctx context.Context, req *connect.Request[ctrlv1.DeleteEnvironmentRequest]) (*connect.Response[ctrlv1.DeleteEnvironmentResponse], error) {
	if err := auth.Authenticate(req, s.bearer); err != nil {
		return nil, err
	}
	if err := assert.All(
		assert.NotEmpty(req.Msg.GetEnvironmentId(), "environment_id is required"),
		assert.NotNil(req.Msg.GetActor(), "actor is required"),
	); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	env, err := s.db.FindEnvironmentById(ctx, req.Msg.GetEnvironmentId())
	if err != nil {
		if db.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("environment not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if env.Slug == "production" || env.Slug == "preview" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("default environments cannot be deleted"))
	}
	if env.DeleteProtection.Valid && env.DeleteProtection.Bool {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("delete protection is enabled"))
	}

	client := hydrav1.NewEnvironmentServiceIngressClient(s.restate, env.ID)
	_, err = client.Delete().Send(ctx, &hydrav1.DeleteEnvironmentRequest{
		Actor: req.Msg.GetActor(), CorrelationId: auditlog.NewCorrelationID(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("trigger environment deletion: %w", err))
	}
	return connect.NewResponse(&ctrlv1.DeleteEnvironmentResponse{}), nil
}