package router

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/svc/frontline/internal/db"
)

func TestSelectDestinationRoutesStaticArtifactWithoutInstances(t *testing.T) {
	service := &service{platform: "dev", region: "local", regionPlatform: "local.dev"}
	route := db.FindFrontlineRouteByFQDNRow{
		EnvironmentID:    "env_test",
		DeploymentID:     "d_test",
		WorkspaceID:      "ws_test",
		ProjectID:        "prj_test",
		AppID:            "app_test",
		StaticOutputName: sql.NullString{String: "web", Valid: true},
		StaticStorageKey: sql.NullString{String: "deployments/ws_test/d_test/web/bundle.tar.gz", Valid: true},
		StaticDigest:     sql.NullString{String: "2184e0e935333793af5a4244ded7051bae1a68e7053df0495c9f3e63947e62f4", Valid: true},
		StaticMetadata:   []byte(`{"spaFallback":true}`),
	}

	decision, err := service.selectDestination(route, nil, nil)
	require.NoError(t, err)
	require.Equal(t, DestinationStaticArtifact, decision.Destination)
	require.Equal(t, route.WorkspaceID, decision.WorkspaceID)
	require.Equal(t, route.StaticStorageKey.String, decision.StaticArtifact.StorageKey)
	require.Equal(t, route.StaticDigest.String, decision.StaticArtifact.Digest)
	require.True(t, decision.StaticArtifact.SPAFallback)
}
