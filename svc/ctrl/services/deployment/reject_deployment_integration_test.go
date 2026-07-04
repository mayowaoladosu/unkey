package deployment_test

import (
	"database/sql"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	ctrlv1 "github.com/unkeyed/unkey/gen/proto/ctrl/v1"
	"github.com/unkeyed/unkey/pkg/uid"
	"github.com/unkeyed/unkey/svc/ctrl/integration/harness"
	"github.com/unkeyed/unkey/svc/ctrl/integration/seed"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
	"github.com/unkeyed/unkey/svc/ctrl/services/deployment"
	githubclient "github.com/unkeyed/unkey/svc/ctrl/worker/github"
)

const testBearer = "test-bearer-token"

// newRejectService builds a DeployService backed by the harness MySQL. Reject
// only touches the DB, bearer auth, and (when a commit sha is present) GitHub,
// so Restate/RestateAdmin/Auditlogs are unused here.
func newRejectService(t *testing.T, h *harness.Harness) *deployment.Service {
	t.Helper()
	return deployment.New(deployment.Config{
		Database:                        h.DB,
		Restate:                         nil,
		RestateAdmin:                    nil,
		GitHub:                          githubclient.NewNoop(),
		Auditlogs:                       nil,
		AllowUnauthenticatedDeployments: false,
		Bearer:                          testBearer,
	})
}

// seedDeployment creates a full workspace→project→app→environment→deployment
// hierarchy with the given status and returns the deployment.
func seedDeployment(t *testing.T, h *harness.Harness, status db.DeploymentsStatus) db.Deployment {
	t.Helper()
	ws := h.Seed.CreateWorkspace(h.Ctx)
	project := h.Seed.CreateProject(h.Ctx, seed.CreateProjectRequest{
		ID:               uid.New(uid.ProjectPrefix),
		WorkspaceID:      ws.ID,
		Name:             "test-project",
		Slug:             uid.New("slug"),
		DeleteProtection: false,
	})
	app := h.Seed.CreateApp(h.Ctx, seed.CreateAppRequest{
		ID:            uid.New(uid.AppPrefix),
		WorkspaceID:   ws.ID,
		ProjectID:     project.ID,
		Name:          "default",
		Slug:          "default",
		DefaultBranch: "main",
	})
	env := h.Seed.CreateEnvironment(h.Ctx, seed.CreateEnvironmentRequest{
		ID:               uid.New(uid.EnvironmentPrefix),
		WorkspaceID:      ws.ID,
		ProjectID:        project.ID,
		AppID:            app.ID,
		Slug:             "preview",
		Description:      "",
		SentinelConfig:   nil,
		DeleteProtection: false,
	})
	return h.Seed.CreateDeployment(h.Ctx, seed.CreateDeploymentRequest{
		ID:            "",
		WorkspaceID:   ws.ID,
		ProjectID:     project.ID,
		AppID:         app.ID,
		EnvironmentID: env.ID,
		Status:        status,
		CreatedAt:     0,
		UpdatedAt:     sql.NullInt64{Int64: 0, Valid: false},
	})
}

func rejectRequest(deploymentID, bearer string) *connect.Request[ctrlv1.RejectDeploymentRequest] {
	req := connect.NewRequest(&ctrlv1.RejectDeploymentRequest{DeploymentId: deploymentID})
	if bearer != "" {
		req.Header().Set("Authorization", "Bearer "+bearer)
	}
	return req
}

// TestRejectDeployment_AwaitingApprovalTransitionsToCancelled covers U8: a gated
// deployment is rejected and lands in cancelled.
func TestRejectDeployment_AwaitingApprovalTransitionsToCancelled(t *testing.T) {
	h := harness.New(t)
	svc := newRejectService(t, h)
	dep := seedDeployment(t, h, db.DeploymentsStatusAwaitingApproval)

	_, err := svc.RejectDeployment(h.Ctx, rejectRequest(dep.ID, testBearer))
	require.NoError(t, err)

	updated, err := h.DB.FindDeploymentById(h.Ctx, dep.ID)
	require.NoError(t, err)
	require.Equal(t, db.DeploymentsStatusCancelled, updated.Status)
}

// TestRejectDeployment_NonGatedIsFailedPrecondition covers U8: rejecting a
// deployment that is not awaiting approval is refused.
func TestRejectDeployment_NonGatedIsFailedPrecondition(t *testing.T) {
	h := harness.New(t)
	svc := newRejectService(t, h)
	dep := seedDeployment(t, h, db.DeploymentsStatusPending)

	_, err := svc.RejectDeployment(h.Ctx, rejectRequest(dep.ID, testBearer))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	updated, err := h.DB.FindDeploymentById(h.Ctx, dep.ID)
	require.NoError(t, err)
	require.Equal(t, db.DeploymentsStatusPending, updated.Status)
}

// TestRejectDeployment_RequiresAuth covers the bearer guard.
func TestRejectDeployment_RequiresAuth(t *testing.T) {
	h := harness.New(t)
	svc := newRejectService(t, h)
	dep := seedDeployment(t, h, db.DeploymentsStatusAwaitingApproval)

	_, err := svc.RejectDeployment(h.Ctx, rejectRequest(dep.ID, "wrong-token"))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
