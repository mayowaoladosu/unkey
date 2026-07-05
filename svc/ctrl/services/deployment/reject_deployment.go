package deployment

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"connectrpc.com/connect"
	ctrlv1 "github.com/unkeyed/unkey/gen/proto/ctrl/v1"
	"github.com/unkeyed/unkey/pkg/logger"
	"github.com/unkeyed/unkey/svc/ctrl/internal/auth"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
)

// RejectDeployment rejects a deployment that is awaiting approval, transitioning
// it to cancelled. Unlike CancelDeployment, this targets the pre-workflow
// awaiting_approval state: no Restate invocation exists yet, so it is a pure
// atomic status transition with no workflow to cancel.
func (s *Service) RejectDeployment(ctx context.Context, req *connect.Request[ctrlv1.RejectDeploymentRequest]) (*connect.Response[ctrlv1.RejectDeploymentResponse], error) {
	if err := auth.Authenticate(req, s.bearer); err != nil {
		return nil, err
	}

	deploymentID := req.Msg.GetDeploymentId()
	if deploymentID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("deployment_id is required"))
	}

	deployment, err := s.db.FindDeploymentById(ctx, deploymentID)
	if err != nil {
		if db.IsNotFound(err) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("deployment %s not found", deploymentID))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to find deployment: %w", err))
	}

	if deployment.Status != db.DeploymentsStatusAwaitingApproval {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("deployment %s is not awaiting approval (current status: %s)", deploymentID, deployment.Status))
	}

	// Atomically transition awaiting_approval → cancelled. A rowsAffected of 0
	// means a concurrent action (approve, another reject, cancel) already moved
	// the deployment out of awaiting_approval; this is the authoritative
	// already-resolved guard the Slack interactivity handler surfaces.
	casResult, err := s.db.CompareAndSwapDeploymentStatus(ctx, db.CompareAndSwapDeploymentStatusParams{
		ID:             deploymentID,
		ExpectedStatus: db.DeploymentsStatusAwaitingApproval,
		NewStatus:      db.DeploymentsStatusCancelled,
		UpdatedAt:      sql.NullInt64{Int64: time.Now().UnixMilli(), Valid: true},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update deployment status: %w", err))
	}
	rowsAffected, err := casResult.RowsAffected()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to check rows affected: %w", err))
	}
	if rowsAffected == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("deployment %s is no longer awaiting approval (concurrent update)", deploymentID))
	}

	// Best-effort GitHub commit status so the PR reflects the rejection.
	if deployment.GitCommitSha.Valid && deployment.GitCommitSha.String != "" {
		repoConn, connErr := s.db.FindGithubRepoConnectionByProjectId(ctx, deployment.ProjectID)
		if connErr == nil && repoConn.InstallationID != 0 {
			if statusErr := s.github.CreateCommitStatus(
				repoConn.InstallationID,
				repoConn.RepositoryFullName,
				deployment.GitCommitSha.String,
				"failure",
				"",
				"Deployment rejected",
				"Unkey Deploy Authorization",
			); statusErr != nil {
				logger.Error("failed to update commit status after rejection", "error", statusErr)
			}
		}
	}

	// Retire the Slack approval prompt (if one was posted) so its
	// Approve/Reject buttons don't outlive the decision. Best-effort: the
	// SlackStatusService no-ops when no prompt exists.
	s.resolveSlackApproval(ctx, deploymentID, false, req.Msg.GetResolvedBy())

	logger.Info("deployment rejected",
		"deployment_id", deploymentID,
		"project_id", deployment.ProjectID,
	)

	return connect.NewResponse(&ctrlv1.RejectDeploymentResponse{}), nil
}
