package githubwebhook_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	hydrav1 "github.com/unkeyed/unkey/gen/proto/hydra/v1"
	"github.com/unkeyed/unkey/pkg/uid"
	"github.com/unkeyed/unkey/svc/ctrl/integration/harness"
	"github.com/unkeyed/unkey/svc/ctrl/integration/seed"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
)

func TestPullRequestDeploymentQueries_IsolateLifecycle(t *testing.T) {
	h := harness.New(t)
	ctx := h.Ctx
	installationID := int64(101)
	repositoryID := int64(202)

	ws := h.Seed.CreateWorkspace(ctx)
	project := h.Seed.CreateProject(ctx, seed.CreateProjectRequest{
		ID:               uid.New(uid.ProjectPrefix),
		WorkspaceID:      ws.ID,
		Name:             "preview-query",
		Slug:             uid.New("slug"),
		DeleteProtection: false,
	})
	app := h.Seed.CreateApp(ctx, seed.CreateAppRequest{
		ID:            uid.New(uid.AppPrefix),
		WorkspaceID:   ws.ID,
		ProjectID:     project.ID,
		Name:          "preview-query",
		Slug:          uid.New("app"),
		DefaultBranch: "main",
	})
	environment := h.Seed.CreateEnvironment(ctx, seed.CreateEnvironmentRequest{
		ID:               uid.New(uid.EnvironmentPrefix),
		WorkspaceID:      ws.ID,
		ProjectID:        project.ID,
		AppID:            app.ID,
		Slug:             "preview",
		DeleteProtection: false,
	})

	require.NoError(t, h.DB.InsertGithubRepoConnection(ctx, db.InsertGithubRepoConnectionParams{
		WorkspaceID:        ws.ID,
		ProjectID:          project.ID,
		AppID:              app.ID,
		InstallationID:     installationID,
		RepositoryID:       repositoryID,
		RepositoryFullName: "acme/repo",
		CreatedAt:          time.Now().UnixMilli(),
		UpdatedAt:          sql.NullInt64{},
	}))

	forkDeployment := createPreviewDeployment(t, ctx, h, ws.ID, project.ID, app.ID, environment.ID)
	branchDeployment := createPreviewDeployment(t, ctx, h, ws.ID, project.ID, app.ID, environment.ID)
	otherForkDeployment := createPreviewDeployment(t, ctx, h, ws.ID, project.ID, app.ID, environment.ID)

	_, err := h.DB.RW().ExecContext(ctx,
		"UPDATE deployments SET git_branch = ?, pr_number = ?, fork_repository_full_name = ? WHERE id = ?",
		"feature/preview", 42, "contributor/repo", forkDeployment.ID,
	)
	require.NoError(t, err)
	_, err = h.DB.RW().ExecContext(ctx,
		"UPDATE deployments SET git_branch = ?, pr_number = NULL, fork_repository_full_name = NULL WHERE id = ?",
		"feature/preview", branchDeployment.ID,
	)
	require.NoError(t, err)
	_, err = h.DB.RW().ExecContext(ctx,
		"UPDATE deployments SET git_branch = ?, pr_number = ?, fork_repository_full_name = ? WHERE id = ?",
		"feature/preview", 42, "another/repo", otherForkDeployment.ID,
	)
	require.NoError(t, err)

	forkMatches, err := h.DB.ListDeploymentsForPullRequest(ctx, db.ListDeploymentsForPullRequestParams{
		InstallationID:         installationID,
		RepositoryID:           repositoryID,
		IsForkPr:               1,
		PrNumber:               sql.NullInt64{Valid: true, Int64: 42},
		ForkRepositoryFullName: sql.NullString{Valid: true, String: "contributor/repo"},
		Branch:                 sql.NullString{Valid: true, String: "feature/preview"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{forkDeployment.ID}, deploymentIDs(forkMatches))

	branchMatches, err := h.DB.ListDeploymentsForPullRequest(ctx, db.ListDeploymentsForPullRequestParams{
		InstallationID:         installationID,
		RepositoryID:           repositoryID,
		IsForkPr:               0,
		PrNumber:               sql.NullInt64{Valid: true, Int64: 42},
		ForkRepositoryFullName: sql.NullString{Valid: true, String: ""},
		Branch:                 sql.NullString{Valid: true, String: "feature/preview"},
	})
	require.NoError(t, err)
	require.Equal(t, []string{branchDeployment.ID}, deploymentIDs(branchMatches))

	now := time.Now().UnixMilli()
	for _, deployment := range []db.Deployment{forkDeployment, branchDeployment} {
		require.NoError(t, h.DB.InsertFrontlineRoute(ctx, db.InsertFrontlineRouteParams{
			ID:                       uid.New(uid.FrontlineRoutePrefix),
			ProjectID:                project.ID,
			AppID:                    app.ID,
			DeploymentID:             deployment.ID,
			EnvironmentID:            environment.ID,
			FullyQualifiedDomainName: uid.New("preview") + ".example.com",
			Sticky:                   db.FrontlineRoutesStickyNone,
			CreatedAt:                now,
			UpdatedAt:                sql.NullInt64{},
		}))
	}

	client := hydrav1.NewGitHubWebhookServiceIngressClient(
		h.Restate,
		fmt.Sprintf("%d:%d", installationID, repositoryID),
	)
	closeRequest := &hydrav1.HandlePushRequest{
		InstallationId:         installationID,
		RepositoryId:           repositoryID,
		RepositoryFullName:     "acme/repo",
		Branch:                 "feature/preview",
		IsForkPr:               true,
		PrNumber:               42,
		ForkRepositoryFullName: "contributor/repo",
		PullRequestClosed:      true,
	}
	_, err = client.HandlePush().Request(ctx, closeRequest)
	require.NoError(t, err)
	// GitHub can redeliver a closure. The second invocation must be a no-op,
	// not a failed transition or a route-not-found error.
	_, err = client.HandlePush().Request(ctx, closeRequest)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		deployment, findErr := h.DB.FindDeploymentById(ctx, forkDeployment.ID)
		return findErr == nil && deployment.DesiredState == db.DeploymentsDesiredStateStopped
	}, 10*time.Second, 50*time.Millisecond)

	forkRoutes, err := h.DB.FindFrontlineRoutesByDeploymentID(ctx, forkDeployment.ID)
	require.NoError(t, err)
	require.Empty(t, forkRoutes)
	branchRoutes, err := h.DB.FindFrontlineRoutesByDeploymentID(ctx, branchDeployment.ID)
	require.NoError(t, err)
	require.Len(t, branchRoutes, 1)
}

func createPreviewDeployment(
	t *testing.T,
	ctx context.Context,
	h *harness.Harness,
	workspaceID string,
	projectID string,
	appID string,
	environmentID string,
) db.Deployment {
	t.Helper()
	return h.Seed.CreateDeployment(ctx, seed.CreateDeploymentRequest{
		WorkspaceID:   workspaceID,
		ProjectID:     projectID,
		AppID:         appID,
		EnvironmentID: environmentID,
		Status:        db.DeploymentsStatusReady,
	})
}

func deploymentIDs(deployments []db.Deployment) []string {
	ids := make([]string, len(deployments))
	for index, deployment := range deployments {
		ids[index] = deployment.ID
	}
	return ids
}
