package router

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/svc/frontline/internal/db"
)

func TestSelectDestinationRoutesToSingaporeOnlyDeploymentFromUSEast(t *testing.T) {
	t.Parallel()

	svc := &service{
		platform:       "aws",
		region:         "us-east-1",
		regionPlatform: "us-east-1.aws",
	}

	decision, err := svc.selectDestination(
		db.FindFrontlineRouteByFQDNRow{
			DeploymentID:     "dep_123",
			EnvironmentID:    "env_123",
			UpstreamProtocol: db.DeploymentsUpstreamProtocolHttp1,
		},
		[]db.FindInstancesByDeploymentIDRow{
			{
				ID:             "ins_123",
				DeploymentID:   "dep_123",
				WorkspaceID:    "ws_123",
				ProjectID:      "proj_123",
				AppID:          "app_123",
				Status:         db.InstancesStatusRunning,
				RegionName:     "ap-southeast-1",
				RegionPlatform: "aws",
			},
		},
		nil,
	)

	require.NoError(t, err)
	require.Equal(t, DestinationRemoteRegion, decision.Destination)
	require.Equal(t, "ap-southeast-1.aws", decision.RemoteRegionAddress)
}
