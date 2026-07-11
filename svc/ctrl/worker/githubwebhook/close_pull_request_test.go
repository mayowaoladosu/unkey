package githubwebhook

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
)

func TestPullRequestActionFor(t *testing.T) {
	tests := []struct {
		name         string
		status       db.DeploymentsStatus
		desiredState db.DeploymentsDesiredState
		want         pullRequestCloseAction
	}{
		{name: "ready and running", status: db.DeploymentsStatusReady, desiredState: db.DeploymentsDesiredStateRunning, want: pullRequestCloseStop},
		{name: "ready and already stopped", status: db.DeploymentsStatusReady, desiredState: db.DeploymentsDesiredStateStopped, want: pullRequestCloseNoop},
		{name: "awaiting approval", status: db.DeploymentsStatusAwaitingApproval, desiredState: db.DeploymentsDesiredStateRunning, want: pullRequestCloseCancel},
		{name: "building", status: db.DeploymentsStatusBuilding, desiredState: db.DeploymentsDesiredStateRunning, want: pullRequestCloseCancel},
		{name: "failed", status: db.DeploymentsStatusFailed, desiredState: db.DeploymentsDesiredStateRunning, want: pullRequestCloseNoop},
		{name: "cancelled", status: db.DeploymentsStatusCancelled, desiredState: db.DeploymentsDesiredStateRunning, want: pullRequestCloseNoop},
		{name: "stopped", status: db.DeploymentsStatusStopped, desiredState: db.DeploymentsDesiredStateStopped, want: pullRequestCloseNoop},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, pullRequestActionFor(db.Deployment{
				Status:       test.status,
				DesiredState: test.desiredState,
			}))
		})
	}
}
