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

// seedSlackConnection inserts an installation + project connection so resolve()
// finds a live connection.
func seedSlackConnection(t *testing.T, h *harness.Harness, workspaceID, projectID string, includePreviews bool) string {
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
	require.NoError(t, h.DB.InsertSlackProjectConnection(h.Ctx, db.InsertSlackProjectConnectionParams{
		ID:              uid.New("slack"),
		WorkspaceID:     workspaceID,
		ProjectID:       projectID,
		InstallationID:  installationID,
		ChannelID:       "C_TEST",
		ChannelName:     "deploys",
		IncludePreviews: includePreviews,
		ApprovalPolicy:  db.SlackProjectConnectionsApprovalPolicyAnyone,
		CreatedAt:       1,
		UpdatedAt:       sql.NullInt64{Int64: 0, Valid: false},
	}))
	return installationID
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
}

// TestResolve_ConnectedReturnsChannelAndToken covers the connected lookup: the
// channel and the (still-encrypted) bot token are returned for the sender.
func TestResolve_ConnectedReturnsChannelAndToken(t *testing.T) {
	h := harness.New(t)
	svc := newResolveService(t, h)
	workspaceID, projectID := seedProject(t, h)
	seedSlackConnection(t, h, workspaceID, projectID, false)

	got, err := svc.resolve(h.Ctx, projectID)
	require.NoError(t, err)
	require.True(t, got.Connected)
	require.Equal(t, "C_TEST", got.ChannelID)
	require.Equal(t, "vault-ciphertext-blob", got.EncryptedBotToken)
	require.False(t, got.IncludePreviews)
}

// TestEnvironmentScoping covers AE1: a preview only notifies when the project
// opted in; production always notifies. Combines resolve() with the scoping
// predicate exactly as Init does.
func TestEnvironmentScoping(t *testing.T) {
	h := harness.New(t)
	svc := newResolveService(t, h)

	// Project scoped to production only (includePreviews = false).
	wsA, projA := seedProject(t, h)
	seedSlackConnection(t, h, wsA, projA, false)
	prodOnly, err := svc.resolve(h.Ctx, projA)
	require.NoError(t, err)
	require.True(t, prodOnly.Connected)
	require.False(t, shouldNotifyEnvironment(false, prodOnly.IncludePreviews), "preview must be skipped when includePreviews=false")
	require.True(t, shouldNotifyEnvironment(true, prodOnly.IncludePreviews), "production must always notify")

	// Project opted into previews.
	wsB, projB := seedProject(t, h)
	seedSlackConnection(t, h, wsB, projB, true)
	withPreviews, err := svc.resolve(h.Ctx, projB)
	require.NoError(t, err)
	require.True(t, withPreviews.IncludePreviews)
	require.True(t, shouldNotifyEnvironment(false, withPreviews.IncludePreviews), "preview must notify when includePreviews=true")
}
