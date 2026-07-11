package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/testutil/containers"
	"github.com/unkeyed/unkey/pkg/uid"
)

func TestDeploymentResourcesPersistIndependentWorkloads(t *testing.T) {
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
	projectID := uid.New(uid.ProjectPrefix)
	appID := uid.New(uid.AppPrefix)
	environmentID := uid.New(uid.EnvironmentPrefix)
	now := time.Now().UnixMilli()
	_, err = database.RW().ExecContext(ctx, `
		INSERT INTO deployments (
			id, k8s_name, workspace_id, project_id, environment_id, app_id,
			sentinel_config, cpu_millicores, memory_mib,
			encrypted_environment_variables, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, deploymentID, uid.DNS1035(12), workspaceID, projectID, environmentID, appID, []byte(`{}`), 250, 256, []byte(`{}`), now)
	require.NoError(t, err)

	webID := uid.New("resource")
	resources := []InsertDeploymentResourceParams{
		{
			ID:               webID,
			DeploymentID:     deploymentID,
			WorkspaceID:      workspaceID,
			ProjectID:        projectID,
			AppID:            appID,
			EnvironmentID:    environmentID,
			Name:             "web",
			Kind:             DeploymentResourcesKindService,
			K8sName:          sql.NullString{Valid: true, String: uid.DNS1035(12)},
			Image:            sql.NullString{Valid: true, String: "registry.example/app:1"},
			Command:          json.RawMessage(`["node","server.js"]`),
			Port:             3000,
			UpstreamProtocol: DeploymentResourcesUpstreamProtocolHttp1,
			Public:           true,
			CpuMillicores:    250,
			MemoryMib:        256,
			CreatedAt:        now,
		},
		{
			ID:               uid.New("resource"),
			DeploymentID:     deploymentID,
			WorkspaceID:      workspaceID,
			ProjectID:        projectID,
			AppID:            appID,
			EnvironmentID:    environmentID,
			Name:             "emails",
			Kind:             DeploymentResourcesKindWorker,
			K8sName:          sql.NullString{Valid: true, String: uid.DNS1035(12)},
			Image:            sql.NullString{Valid: true, String: "registry.example/app:1"},
			Command:          json.RawMessage(`["node","worker.js"]`),
			Port:             0,
			UpstreamProtocol: DeploymentResourcesUpstreamProtocolHttp1,
			Public:           false,
			CpuMillicores:    250,
			MemoryMib:        256,
			CreatedAt:        now,
		},
	}
	for _, resource := range resources {
		require.NoError(t, Query.InsertDeploymentResource(ctx, database.RW(), resource))
		require.NoError(t, Query.InsertDeploymentResource(ctx, database.RW(), resource), "workflow replay must be idempotent")
	}

	stored, err := Query.ListDeploymentResourcesByDeployment(ctx, database.RO(), deploymentID)
	require.NoError(t, err)
	require.Len(t, stored, 2)
	require.Equal(t, DeploymentResourcesKindWorker, stored[0].Kind)
	require.Equal(t, DeploymentResourcesKindService, stored[1].Kind)
	web, err := Query.FindDeploymentResourceByID(ctx, database.RO(), webID)
	require.NoError(t, err)
	require.True(t, web.Public)
	require.JSONEq(t, `["node","server.js"]`, string(web.Command))

	require.NoError(t, Query.DeleteDeploymentResourcesByEnvironment(ctx, database.RW(), environmentID))
	require.Equal(t, 0, countRows(t, ctx, database, "SELECT COUNT(*) FROM deployment_resources WHERE deployment_id = ?", deploymentID))
}
