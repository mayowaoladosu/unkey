package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/testutil/containers"
	"github.com/unkeyed/unkey/pkg/uid"
)

func TestDeploymentManifestIsImmutableAndReadable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mysqlCfg := containers.MySQL(t)
	database, err := New(Config{PrimaryDSN: mysqlCfg.DSN})
	require.NoError(t, err)
	defer func() { require.NoError(t, database.Close()) }()

	deploymentID := uid.New(uid.DeploymentPrefix)
	workspaceID := uid.New(uid.WorkspacePrefix)
	projectID := uid.New("prj")
	appID := uid.New("app")
	environmentID := uid.New("env")
	now := time.Now().UnixMilli()

	_, err = database.RW().ExecContext(ctx, `
		INSERT INTO deployments (
			id, k8s_name, workspace_id, project_id, environment_id, app_id,
			sentinel_config, cpu_millicores, memory_mib,
			encrypted_environment_variables, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, deploymentID, uid.DNS1035(12), workspaceID, projectID, environmentID, appID, []byte(`{}`), 250, 256, []byte(`{}`), now)
	require.NoError(t, err)

	manifest := []byte(`{"version":1,"outputs":[{"kind":"container","name":"web"}]}`)
	params := InsertDeploymentManifestParams{
		DeploymentID:  deploymentID,
		WorkspaceID:   workspaceID,
		ProjectID:     projectID,
		AppID:         appID,
		EnvironmentID: environmentID,
		SchemaVersion: 1,
		Fingerprint:   "2184e0e935333793af5a4244ded7051bae1a68e7053df0495c9f3e63947e62f4",
		AdapterID:     "container",
		OutputMode:    DeploymentManifestsOutputModeContainer,
		Manifest:      manifest,
		CreatedAt:     now,
	}

	require.NoError(t, Query.InsertDeploymentManifest(ctx, database.RW(), params))

	stored, err := Query.FindDeploymentManifestByDeploymentID(ctx, database.RO(), deploymentID)
	require.NoError(t, err)
	require.Equal(t, params.DeploymentID, stored.DeploymentID)
	require.Equal(t, params.Fingerprint, stored.Fingerprint)
	require.Equal(t, params.AdapterID, stored.AdapterID)
	require.Equal(t, params.OutputMode, stored.OutputMode)
	require.JSONEq(t, string(manifest), string(stored.Manifest))

	err = Query.InsertDeploymentManifest(ctx, database.RW(), params)
	require.Error(t, err, "a deployment manifest must never be replaced")
}
