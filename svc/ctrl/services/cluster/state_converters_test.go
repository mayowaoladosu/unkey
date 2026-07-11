package cluster

import (
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	ctrlv1 "github.com/unkeyed/unkey/gen/proto/ctrl/v1"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
)

func TestDeploymentRowToState_Running(t *testing.T) {
	row := deploymentRow{
		dt: db.DeploymentTopology{
			DesiredStatus:          db.DeploymentTopologyDesiredStatusRunning,
			AutoscalingReplicasMin: 1,
			AutoscalingReplicasMax: 3,
		},
		d: db.Deployment{
			ID:             "deploy_123",
			K8sName:        "my-app",
			WorkspaceID:    "ws_1",
			ProjectID:      "prj_1",
			EnvironmentID:  "env_1",
			AppID:          "app_1",
			Image:          sql.NullString{Valid: true, String: "registry.io/app:v1"},
			CpuMillicores:  250,
			MemoryMib:      256,
			Port:           8080,
			ShutdownSignal: db.DeploymentsShutdownSignalSIGTERM,
		},
		k8sNamespace:    sql.NullString{Valid: true, String: "ws-namespace"},
		environmentSlug: "production",
		regionName:      "us-east-1",
	}

	state, err := deploymentRowToState(row, 42)
	require.NoError(t, err)
	require.NotNil(t, state)

	require.Equal(t, uint64(42), state.GetVersion())

	apply := state.GetApply()
	require.NotNil(t, apply, "running status should produce an ApplyDeployment")
	require.Equal(t, "deploy_123", apply.GetDeploymentId())
	require.Equal(t, "my-app", apply.GetK8SName())
	require.Equal(t, "ws-namespace", apply.GetK8SNamespace())
	require.Equal(t, int64(250), apply.GetCpuMillicores())
	require.Equal(t, uint32(1), apply.GetAutoscaling().GetMinReplicas())
	require.Equal(t, uint32(3), apply.GetAutoscaling().GetMaxReplicas())
}

func TestDeploymentRowToState_Stopped(t *testing.T) {
	row := deploymentRow{
		dt: db.DeploymentTopology{
			DesiredStatus: db.DeploymentTopologyDesiredStatusStopped,
		},
		d: db.Deployment{
			K8sName: "my-app",
		},
		k8sNamespace: sql.NullString{Valid: true, String: "ws-namespace"},
	}

	state, err := deploymentRowToState(row, 7)
	require.NoError(t, err)
	require.NotNil(t, state)

	require.Equal(t, uint64(7), state.GetVersion())

	del := state.GetDelete()
	require.NotNil(t, del, "stopped status should produce a DeleteDeployment")
	require.Equal(t, "my-app", del.GetK8SName())
	require.Equal(t, "ws-namespace", del.GetK8SNamespace())
}

func TestDeploymentRowToState_ResourceWorker(t *testing.T) {
	row := deploymentRow{
		dt: db.DeploymentTopology{
			DesiredStatus:          db.DeploymentTopologyDesiredStatusRunning,
			AutoscalingReplicasMin: 1,
			AutoscalingReplicasMax: 2,
		},
		d: db.Deployment{
			ID:                            "deploy_123",
			K8sName:                       "legacy-name",
			WorkspaceID:                   "ws_1",
			ProjectID:                     "prj_1",
			EnvironmentID:                 "env_1",
			AppID:                         "app_1",
			EncryptedEnvironmentVariables: []byte(`{}`),
			ShutdownSignal:                db.DeploymentsShutdownSignalSIGTERM,
		},
		resource: &db.DeploymentResource{
			ID:            "resource_worker",
			Name:          "emails",
			Kind:          db.DeploymentResourcesKindWorker,
			K8sName:       sql.NullString{Valid: true, String: "deploy-emails"},
			Image:         sql.NullString{Valid: true, String: "registry.io/app:v2"},
			Command:       json.RawMessage(`["node","worker.js"]`),
			CpuMillicores: 500,
			MemoryMib:     512,
		},
		k8sNamespace: sql.NullString{Valid: true, String: "ws-namespace"},
	}

	state, err := deploymentRowToState(row, 9)
	require.NoError(t, err)
	apply := state.GetApply()
	require.Equal(t, "resource_worker", apply.GetResourceId())
	require.Equal(t, "deploy-emails", apply.GetK8SName())
	require.Equal(t, ctrlv1.DeploymentResourceKind_DEPLOYMENT_RESOURCE_KIND_WORKER, apply.GetResourceKind())
	require.Equal(t, []string{"node", "worker.js"}, apply.GetCommand())
	require.Zero(t, apply.GetPort())
	require.False(t, apply.GetPublic())
	require.Empty(t, apply.GetHealthcheck())

	row.dt.DesiredStatus = db.DeploymentTopologyDesiredStatusStopped
	state, err = deploymentRowToState(row, 10)
	require.NoError(t, err)
	require.Equal(t, "resource_worker", state.GetDelete().GetResourceId())
	require.Equal(t, "deploy-emails", state.GetDelete().GetK8SName())
}
