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
			Bindings:         json.RawMessage(`[]`),
			AllowedCallers:   json.RawMessage(`["resource_worker"]`),
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
			Bindings:         json.RawMessage(`[{"name":"WEB","resourceId":"` + webID + `","resourceName":"web","protocol":"http","host":"web-service","port":3000}]`),
			AllowedCallers:   json.RawMessage(`[]`),
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
	require.JSONEq(t, `["resource_worker"]`, string(web.AllowedCallers))

	regionID := uid.New("region")
	require.NoError(t, Query.InsertDeploymentTopology(ctx, database.RW(), InsertDeploymentTopologyParams{
		WorkspaceID:            workspaceID,
		DeploymentID:           deploymentID,
		ResourceID:             webID,
		RegionID:               regionID,
		AutoscalingReplicasMin: 2,
		AutoscalingReplicasMax: 4,
		DesiredStatus:          DeploymentTopologyDesiredStatusRunning,
		CreatedAt:              now,
	}))
	require.NoError(t, Query.UpsertInstance(ctx, database.RW(), UpsertInstanceParams{
		ID:            uid.New(uid.InstancePrefix),
		DeploymentID:  deploymentID,
		ResourceID:    webID,
		WorkspaceID:   workspaceID,
		ProjectID:     projectID,
		AppID:         appID,
		RegionID:      regionID,
		K8sName:       "web-pod-1",
		Address:       "10-0-0-1.ns.pod.cluster.local:3000",
		CpuMillicores: 250,
		MemoryMib:     256,
		Status:        InstancesStatusRunning,
	}))

	topologyRequirements, err := Query.FindDeploymentTopologyMinReplicas(ctx, database.RO(), deploymentID)
	require.NoError(t, err)
	require.Equal(t, []FindDeploymentTopologyMinReplicasRow{{
		ResourceID:             webID,
		RegionID:               regionID,
		AutoscalingReplicasMin: 2,
	}}, topologyRequirements)
	resourceInstances, err := Query.FindInstancesByResourceAndRegion(ctx, database.RO(), FindInstancesByResourceAndRegionParams{
		ResourceID: webID,
		RegionID:   regionID,
	})
	require.NoError(t, err)
	require.Len(t, resourceInstances, 1)
	require.Equal(t, webID, resourceInstances[0].ResourceID)

	require.NoError(t, Query.DeleteDeploymentResourcesByEnvironment(ctx, database.RW(), environmentID))
	require.Equal(t, 0, countRows(t, ctx, database, "SELECT COUNT(*) FROM deployment_resources WHERE deployment_id = ?", deploymentID))
}
