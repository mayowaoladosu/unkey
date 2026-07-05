package githubwebhook

import (
	"time"

	restate "github.com/restatedev/sdk-go"
	hydrav1 "github.com/unkeyed/unkey/gen/proto/hydra/v1"
	"github.com/unkeyed/unkey/pkg/logger"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
	"github.com/unkeyed/unkey/svc/ctrl/internal/urls"
)

// blockDeploymentForApproval creates a GitHub commit status to signal that
// the push requires authorization from a project member. Clicking "Details"
// in the PR goes directly to the dashboard authorize page.
func (s *Service) blockDeploymentForApproval(
	ctx restate.ObjectContext,
	req *hydrav1.HandlePushRequest,
	project db.Project,
	app db.App,
	repo db.GithubRepoConnection,
	env db.Environment,
	deploymentID string,
) error {
	workspace, err := restate.Run(ctx, func(runCtx restate.RunContext) (db.Workspace, error) {
		return s.db.FindWorkspaceByID(runCtx, project.WorkspaceID)
	}, restate.WithName("find workspace for approval log url"), restate.WithMaxRetryDuration(30*time.Second))
	if err != nil {
		return err
	}

	logURL := urls.DeploymentLogURL(s.dashboardURL, workspace.Slug, project.ID, app.ID, deploymentID)

	if !s.allowUnauthenticatedDeployments {
		_ = restate.RunVoid(ctx, func(_ restate.RunContext) error {
			return s.github.CreateCommitStatus(
				repo.InstallationID,
				req.GetRepositoryFullName(),
				req.GetAfter(),
				"failure",
				logURL,
				"Awaiting authorization from a project member",
				"Unkey Deploy Authorization",
			)
		}, restate.WithName("create commit status for authorization"), restate.WithMaxRetryDuration(30*time.Second))
	}

	// Post an interactive Slack approval prompt, fire-and-forget through the
	// deployment-keyed SlackStatusService. That service owns the connection
	// lookup, vault token decrypt, no-op-when-unconnected behaviour, and durable
	// retry, so the webhook worker needs no Slack or vault credentials.
	hydrav1.NewSlackStatusServiceClient(ctx, deploymentID).PostApproval().Send(&hydrav1.SlackPostApprovalRequest{
		WorkspaceId:      project.WorkspaceID,
		ProjectId:        project.ID,
		EnvironmentLabel: env.Slug,
		ReviewUrl:        logURL,
		IsProduction:     env.Slug == "production",
		CommitSha:        req.GetAfter(),
		CommitMessage:    req.GetCommitMessage(),
		Trigger:          "github",
		TriggeredBy:      req.GetSenderLogin(),
	})

	logger.Info("deployment blocked for authorization",
		"deployment_id", deploymentID,
		"project_id", project.ID,
		"sender", req.GetSenderLogin(),
	)

	return nil
}
