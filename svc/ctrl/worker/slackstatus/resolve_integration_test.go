package slackstatus

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/uid"
	"github.com/unkeyed/unkey/svc/ctrl/integration/harness"
	"github.com/unkeyed/unkey/svc/ctrl/integration/seed"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
)

// newResolveService builds a Service whose only used dependency is the DB; the
// resolve() lookup and env-scoping predicate don't touch Slack or vault.
func newResolveService(t *testing.T, h *harness.Harness) *Service {
	t.Helper()
	return New(Config{Slack: nil, Vault: nil, DB: h.DB})
}

// seedProject creates a workspace + project and returns their ids.
func seedProject(t *testing.T, h *harness.Harness) (workspaceID, projectID string) {
	t.Helper()
	ws := h.Seed.CreateWorkspace(h.Ctx)
	project := h.Seed.CreateProject(h.Ctx, seed.CreateProjectRequest{
		ID:               uid.New(uid.ProjectPrefix),
		WorkspaceID:      ws.ID,
		Name:             "slack-test",
		Slug:             uid.New("slug"),
		DeleteProtection: false,
	})
	return ws.ID, project.ID
}

// seedInstallation inserts a Slack installation for the workspace.
func seedInstallation(t *testing.T, h *harness.Harness, workspaceID string) string {
	t.Helper()
	installationID := uid.New("slack")
	require.NoError(t, h.DB.InsertSlackInstallation(h.Ctx, db.InsertSlackInstallationParams{
		ID:                installationID,
		WorkspaceID:       workspaceID,
		TeamID:            uid.New("team"),
		BotToken:          "vault-ciphertext-blob",
		BotUserID:         "U_BOT",
		InstalledByUserID: "user_1",
		CreatedAt:         1,
		UpdatedAt:         sql.NullInt64{Int64: 0, Valid: false},
	}))
	return installationID
}

// seedChannel connects one channel to the project with the given scope.
func seedChannel(t *testing.T, h *harness.Harness, workspaceID, projectID, installationID, channelID string, notifyProduction, notifyPreviews bool) {
	t.Helper()
	require.NoError(t, h.DB.InsertSlackProjectConnection(h.Ctx, db.InsertSlackProjectConnectionParams{
		ID:               uid.New("slack"),
		WorkspaceID:      workspaceID,
		ProjectID:        projectID,
		InstallationID:   installationID,
		ChannelID:        channelID,
		ChannelName:      "chan-" + channelID,
		NotifyProduction: notifyProduction,
		NotifyPreviews:   notifyPreviews,
		CreatedAt:        1,
		UpdatedAt:        sql.NullInt64{Int64: 0, Valid: false},
	}))
}

// TestResolve_NoConnectionIsNoOp covers AE5: a project with no Slack connection
// resolves to not-connected, so Init/PostApproval no-op without posting.
func TestResolve_NoConnectionIsNoOp(t *testing.T) {
	h := harness.New(t)
	svc := newResolveService(t, h)
	_, projectID := seedProject(t, h)

	got, err := svc.resolve(h.Ctx, projectID)
	require.NoError(t, err)
	require.False(t, got.Connected)
	require.Empty(t, got.Targets)
}

// TestResolve_MultiChannelReturnsAllTargets covers the multi-channel fan-out:
// every connected channel comes back with its own scope and the (still
// encrypted) bot token.
func TestResolve_MultiChannelReturnsAllTargets(t *testing.T) {
	h := harness.New(t)
	svc := newResolveService(t, h)
	workspaceID, projectID := seedProject(t, h)
	installationID := seedInstallation(t, h, workspaceID)
	seedChannel(t, h, workspaceID, projectID, installationID, "C_PROD", true, false)
	seedChannel(t, h, workspaceID, projectID, installationID, "C_ALL", true, true)

	got, err := svc.resolve(h.Ctx, projectID)
	require.NoError(t, err)
	require.True(t, got.Connected)
	require.Len(t, got.Targets, 2)

	byChannel := map[string]resolvedTarget{}
	for _, target := range got.Targets {
		byChannel[target.ChannelID] = target
		require.Equal(t, "vault-ciphertext-blob", target.EncryptedBotToken)
	}
	require.True(t, byChannel["C_PROD"].NotifyProduction)
	require.False(t, byChannel["C_PROD"].NotifyPreviews)
	require.True(t, byChannel["C_ALL"].NotifyPreviews)
}

// TestEnvironmentScoping covers AE1 per channel: a preview only notifies
// channels that opted into previews; production only notifies channels that
// have production enabled.
func TestEnvironmentScoping(t *testing.T) {
	h := harness.New(t)
	svc := newResolveService(t, h)
	workspaceID, projectID := seedProject(t, h)
	installationID := seedInstallation(t, h, workspaceID)
	seedChannel(t, h, workspaceID, projectID, installationID, "C_PRODONLY", true, false)
	seedChannel(t, h, workspaceID, projectID, installationID, "C_PREVIEWONLY", false, true)

	got, err := svc.resolve(h.Ctx, projectID)
	require.NoError(t, err)
	require.Len(t, got.Targets, 2)

	for _, target := range got.Targets {
		prod := shouldNotifyEnvironment(true, target.NotifyProduction, target.NotifyPreviews)
		preview := shouldNotifyEnvironment(false, target.NotifyProduction, target.NotifyPreviews)
		switch target.ChannelID {
		case "C_PRODONLY":
			require.True(t, prod, "production-scoped channel must get production notifications")
			require.False(t, preview, "production-scoped channel must not get previews")
		case "C_PREVIEWONLY":
			require.False(t, prod, "preview-scoped channel must not get production notifications")
			require.True(t, preview, "preview-scoped channel must get previews")
		default:
			t.Fatalf("unexpected channel %s", target.ChannelID)
		}
	}
}

// TestDeploymentAwaitingApproval covers PostApproval's status re-check: a late
// fire-and-forget prompt must only post while the deployment is still
// awaiting_approval, and a missing deployment counts as not-awaiting.
func TestDeploymentAwaitingApproval(t *testing.T) {
	h := harness.New(t)
	svc := newResolveService(t, h)
	workspaceID, projectID := seedProject(t, h)

	app := h.Seed.CreateApp(h.Ctx, seed.CreateAppRequest{
		ID:            uid.New("app"),
		WorkspaceID:   workspaceID,
		ProjectID:     projectID,
		Name:          "slack-approval-test",
		Slug:          "default",
		DefaultBranch: "main",
	})
	env := h.Seed.CreateEnvironment(h.Ctx, seed.CreateEnvironmentRequest{
		ID:          uid.New("env"),
		WorkspaceID: workspaceID,
		ProjectID:   projectID,
		AppID:       app.ID,
		Slug:        "preview",
	})

	seedDeployment := func(status db.DeploymentsStatus) string {
		d := h.Seed.CreateDeployment(h.Ctx, seed.CreateDeploymentRequest{
			ID:            uid.New(uid.DeploymentPrefix),
			WorkspaceID:   workspaceID,
			ProjectID:     projectID,
			AppID:         app.ID,
			EnvironmentID: env.ID,
			Status:        status,
		})
		return d.ID
	}

	awaiting := seedDeployment(db.DeploymentsStatusAwaitingApproval)
	pending := seedDeployment(db.DeploymentsStatusPending)
	cancelled := seedDeployment(db.DeploymentsStatusCancelled)

	got, err := svc.deploymentAwaitingApproval(h.Ctx, awaiting)
	require.NoError(t, err)
	require.True(t, got, "awaiting_approval deployment must be postable")

	got, err = svc.deploymentAwaitingApproval(h.Ctx, pending)
	require.NoError(t, err)
	require.False(t, got, "already-authorized (pending) deployment must not be posted")

	got, err = svc.deploymentAwaitingApproval(h.Ctx, cancelled)
	require.NoError(t, err)
	require.False(t, got, "rejected/cancelled deployment must not be posted")

	got, err = svc.deploymentAwaitingApproval(h.Ctx, "dep_does_not_exist")
	require.NoError(t, err)
	require.False(t, got, "missing deployment must not be posted and must not error")
}
