package githubwebhook

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	restate "github.com/restatedev/sdk-go"
	hydrav1 "github.com/unkeyed/unkey/gen/proto/hydra/v1"
	"github.com/unkeyed/unkey/pkg/logger"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
)

const pullRequestClosedMessage = "Pull request closed"

type pullRequestCloseAction uint8

const (
	pullRequestCloseNoop pullRequestCloseAction = iota
	pullRequestCloseStop
	pullRequestCloseCancel
)

// closePullRequest ends every preview deployment associated with a pull
// request while retaining its deployment history. Ready deployments are scaled
// to zero, in-flight workflows are cancelled, and routes that still point at a
// closed preview are removed from Frontline.
func (s *Service) closePullRequest(ctx restate.ObjectContext, req *hydrav1.HandlePushRequest) error {
	deployments, err := restate.Run(ctx, func(runCtx restate.RunContext) ([]db.Deployment, error) {
		return s.db.ListDeploymentsForPullRequest(runCtx, db.ListDeploymentsForPullRequestParams{
			InstallationID: req.GetInstallationId(),
			RepositoryID:   req.GetRepositoryId(),
			IsForkPr:       boolToInt64(req.GetIsForkPr()),
			PrNumber: sql.NullInt64{
				Int64: req.GetPrNumber(),
				Valid: req.GetPrNumber() != 0,
			},
			Branch: sql.NullString{
				String: req.GetBranch(),
				Valid:  req.GetBranch() != "",
			},
			ForkRepositoryFullName: sql.NullString{
				String: req.GetForkRepositoryFullName(),
				Valid:  true,
			},
		})
	}, restate.WithName("list pull request previews"))
	if err != nil {
		return fmt.Errorf("list pull request previews: %w", err)
	}

	for _, deployment := range deployments {
		switch pullRequestActionFor(deployment) {
		case pullRequestCloseStop:
			_, err = hydrav1.NewDeploymentServiceClient(ctx, deployment.ID).
				ScheduleDesiredStateChange().
				Request(&hydrav1.ScheduleDesiredStateChangeRequest{
					DelayMillis: 0,
					State:       hydrav1.DeploymentDesiredState_DEPLOYMENT_DESIRED_STATE_STOPPED,
					Overwrite:   true,
				})
			if err != nil {
				return fmt.Errorf("stop preview deployment %s: %w", deployment.ID, err)
			}
		case pullRequestCloseCancel:
			if err = s.cancelPullRequestDeployment(ctx, deployment); err != nil {
				return err
			}
		case pullRequestCloseNoop:
		}

		if err = restate.RunVoid(ctx, func(runCtx restate.RunContext) error {
			return db.Tx(runCtx, s.db.RW(), func(txCtx context.Context, tx db.DBTX) error {
				queries := db.NewQueries(tx)
				if deleteErr := queries.DeleteFrontlineRoutesByDeploymentID(txCtx, deployment.ID); deleteErr != nil {
					return deleteErr
				}
				return queries.BumpFrontlineRouteRevision(txCtx)
			})
		}, restate.WithName(fmt.Sprintf("release preview routes for %s", deployment.ID))); err != nil {
			return fmt.Errorf("release preview routes for %s: %w", deployment.ID, err)
		}
	}

	logger.Info("closed pull request preview lifecycle",
		"repository", req.GetRepositoryFullName(),
		"branch", req.GetBranch(),
		"pr_number", req.GetPrNumber(),
		"deployments", len(deployments),
	)
	return nil
}

func pullRequestActionFor(deployment db.Deployment) pullRequestCloseAction {
	if deployment.Status == db.DeploymentsStatusReady && deployment.DesiredState == db.DeploymentsDesiredStateRunning {
		return pullRequestCloseStop
	}
	if !deployment.Status.IsTerminal() {
		return pullRequestCloseCancel
	}
	return pullRequestCloseNoop
}

func (s *Service) cancelPullRequestDeployment(ctx restate.ObjectContext, deployment db.Deployment) error {
	now := time.Now().UnixMilli()
	if err := restate.RunVoid(ctx, func(runCtx restate.RunContext) error {
		if err := s.db.EndActiveDeploymentStepsWithError(runCtx, db.EndActiveDeploymentStepsWithErrorParams{
			DeploymentID: deployment.ID,
			EndedAt:      sql.NullInt64{Valid: true, Int64: now},
			Error:        sql.NullString{Valid: true, String: pullRequestClosedMessage},
		}); err != nil {
			logger.Warn("failed to stamp closed pull request on deployment steps",
				"deployment_id", deployment.ID,
				"error", err,
			)
		}
		return s.db.UpdateDeploymentStatusIfActive(runCtx, db.UpdateDeploymentStatusIfActiveParams{
			ID:               deployment.ID,
			Status:           db.DeploymentsStatusCancelled,
			UpdatedAt:        sql.NullInt64{Valid: true, Int64: now},
			TerminalStatuses: db.TerminalDeploymentStatuses,
		})
	}, restate.WithName(fmt.Sprintf("cancel preview deployment %s", deployment.ID))); err != nil {
		return fmt.Errorf("mark preview deployment %s cancelled: %w", deployment.ID, err)
	}

	if !deployment.InvocationID.Valid || deployment.InvocationID.String == "" {
		return nil
	}
	if s.restateAdmin == nil {
		return fmt.Errorf("cancel preview deployment %s: restate admin is not configured", deployment.ID)
	}
	if err := restate.RunVoid(ctx, func(runCtx restate.RunContext) error {
		return s.restateAdmin.CancelInvocation(runCtx, deployment.InvocationID.String)
	}, restate.WithName(fmt.Sprintf("cancel preview invocation %s", deployment.ID))); err != nil {
		return fmt.Errorf("cancel preview invocation %s: %w", deployment.ID, err)
	}
	return nil
}
