package deployment

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/mysql/sqlcomment"
	"github.com/unkeyed/unkey/pkg/testutil/containers"
	"github.com/unkeyed/unkey/pkg/uid"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
)

func TestPersistDeploymentWithManifestIsAtomic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mysqlCfg := containers.MySQL(t)
	database, err := db.New(mysqlCfg.DSN, sqlcomment.Static{})
	require.NoError(t, err)
	defer func() { require.NoError(t, database.Close()) }()

	deploymentID := uid.New(uid.DeploymentPrefix)
	workspaceID := uid.New(uid.WorkspacePrefix)
	projectID := uid.New("prj")
	appID := uid.New("app")
	environmentID := uid.New("env")
	now := time.Now().UnixMilli()

	deployment := db.InsertDeploymentParams{
		ID:                            deploymentID,
		K8sName:                       uid.DNS1035(12),
		WorkspaceID:                   workspaceID,
		ProjectID:                     projectID,
		AppID:                         appID,
		EnvironmentID:                 environmentID,
		SentinelConfig:                []byte(`{}`),
		EncryptedEnvironmentVariables: []byte(`{}`),
		Status:                        db.DeploymentsStatusPending,
		CpuMillicores:                 250,
		MemoryMib:                     256,
		Port:                          8080,
		ShutdownSignal:                db.DeploymentsShutdownSignalSIGTERM,
		UpstreamProtocol:              db.DeploymentsUpstreamProtocolHttp1,
		DeploymentTrigger:             db.DeploymentsTriggerDashboard,
		CreatedAt:                     now,
		UpdatedAt:                     sql.NullInt64{},
	}
	manifest := db.InsertDeploymentManifestParams{
		DeploymentID:  deploymentID,
		WorkspaceID:   workspaceID,
		ProjectID:     projectID,
		AppID:         appID,
		EnvironmentID: environmentID,
		SchemaVersion: 1,
		Fingerprint:   "2184e0e935333793af5a4244ded7051bae1a68e7053df0495c9f3e63947e62f4",
		AdapterID:     "container",
		OutputMode:    db.DeploymentManifestsOutputMode("invalid"),
		Manifest:      []byte(`{"version":1}`),
		CreatedAt:     now,
	}

	err = persistDeploymentWithManifest(ctx, database, deployment, manifest)
	require.Error(t, err)
	_, err = database.FindDeploymentById(ctx, deploymentID)
	require.True(t, db.IsNotFound(err), "deployment row must roll back when manifest persistence fails")

	manifest.OutputMode = db.DeploymentManifestsOutputModeContainer
	require.NoError(t, persistDeploymentWithManifest(ctx, database, deployment, manifest))
	stored, err := database.FindDeploymentManifestByDeploymentID(ctx, deploymentID)
	require.NoError(t, err)
	require.Equal(t, manifest.Fingerprint, stored.Fingerprint)
}
