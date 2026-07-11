package routing

import (
	"context"
	"time"

	restate "github.com/restatedev/sdk-go"
	hydrav1 "github.com/unkeyed/unkey/gen/proto/hydra/v1"
	"github.com/unkeyed/unkey/pkg/logger"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
)

// AssignFrontlineRoutes reassigns a set of frontline routes to a new deployment.
//
// Each route, its explicit target pointer, and append-only assignment history
// are updated in one database transaction wrapped by [restate.Run] for
// durability. The invocation ID makes history insertion replay-idempotent.
//
// Returns an empty response on success. Database errors from the route updates
// propagate directly to the caller.
func (s *Service) AssignFrontlineRoutes(ctx restate.ObjectContext, req *hydrav1.AssignFrontlineRoutesRequest) (*hydrav1.AssignFrontlineRoutesResponse, error) {
	logger.Info("assigning domains",
		"deployment_id", req.GetDeploymentId(),
		"frontline_routes", req.GetFrontlineRouteIds(),
	)

	operationID := ctx.Request().ID
	err := restate.RunVoid(ctx, func(stepCtx restate.RunContext) error {
		return db.Tx(stepCtx, s.db.RW(), func(txCtx context.Context, tx db.DBTX) error {
			queries := db.NewQueries(tx)
			now := time.Now().UnixMilli()
			for _, frontlineRouteID := range req.GetFrontlineRouteIds() {
				if assignErr := assignFrontlineRoute(
					txCtx,
					queries,
					frontlineRouteID,
					req.GetDeploymentId(),
					db.DeploymentTargetAssignmentsReasonDeploy,
					operationID,
					now,
				); assignErr != nil {
					return assignErr
				}
			}
			return nil
		})
	}, restate.WithName("assign frontline routes and deployment targets"))
	if err != nil {
		return nil, err
	}

	return &hydrav1.AssignFrontlineRoutesResponse{}, nil
}
