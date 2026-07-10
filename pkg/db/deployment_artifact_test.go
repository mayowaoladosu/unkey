package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/testutil/containers"
	"github.com/unkeyed/unkey/pkg/uid"
)

func TestDeploymentArtifactsAreIdempotentAndQueryable(t *testing.T) {
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

	artifact := RecordDeploymentArtifactParams{
		ID:            uid.New("artifact"),
		DeploymentID:  deploymentID,
		WorkspaceID:   workspaceID,
		ProjectID:     projectID,
		AppID:         appID,
		EnvironmentID: environmentID,
		Name:          "web",
		Kind:          DeploymentArtifactsKindStaticBundle,
		StorageKey:    "deployments/" + deploymentID + "/web.tar.gz",
		Digest:        "2184e0e935333793af5a4244ded7051bae1a68e7053df0495c9f3e63947e62f4",
		SizeBytes:     1024,
		ContentType:   "application/gzip",
		Metadata:      []byte(`{"spaFallback":true}`),
		CreatedAt:     now,
	}

	require.NoError(t, Query.RecordDeploymentArtifact(ctx, database.RW(), artifact))
	require.NoError(t, Query.RecordDeploymentArtifact(ctx, database.RW(), artifact), "replayed materialization must be idempotent")

	stored, err := Query.FindDeploymentArtifact(ctx, database.RO(), FindDeploymentArtifactParams{
		DeploymentID: deploymentID,
		Kind:         DeploymentArtifactsKindStaticBundle,
		Name:         "web",
	})
	require.NoError(t, err)
	require.Equal(t, artifact.StorageKey, stored.StorageKey)
	require.Equal(t, artifact.Digest, stored.Digest)
	require.JSONEq(t, string(artifact.Metadata), string(stored.Metadata))

	artifacts, err := Query.ListDeploymentArtifacts(ctx, database.RO(), deploymentID)
	require.NoError(t, err)
	require.Len(t, artifacts, 1)
}
