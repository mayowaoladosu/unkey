package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/testutil/containers"
	"github.com/unkeyed/unkey/pkg/uid"
)

func TestDeploymentTargetsRetainIdempotentAssignmentHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mysqlCfg := containers.MySQL(t)
	database, err := New(Config{PrimaryDSN: mysqlCfg.DSN})
	require.NoError(t, err)
	defer func() { require.NoError(t, database.Close()) }()

	workspaceID := uid.New(uid.WorkspacePrefix)
	projectID := uid.New("prj")
	appID := uid.New("app")
	environmentID := uid.New("env")
	firstDeploymentID := uid.New(uid.DeploymentPrefix)
	secondDeploymentID := uid.New(uid.DeploymentPrefix)
	targetID := uid.New("target")
	now := time.Now().UnixMilli()

	for _, deploymentID := range []string{firstDeploymentID, secondDeploymentID} {
		_, err = database.RW().ExecContext(ctx, `
			INSERT INTO deployments (
				id, k8s_name, workspace_id, project_id, environment_id, app_id,
				sentinel_config, cpu_millicores, memory_mib,
				encrypted_environment_variables, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, deploymentID, uid.DNS1035(12), workspaceID, projectID, environmentID, appID, []byte(`{}`), 250, 256, []byte(`{}`), now)
		require.NoError(t, err)
	}

	require.NoError(t, Query.InsertDeploymentTarget(ctx, database.RW(), InsertDeploymentTargetParams{
		ID:                   targetID,
		WorkspaceID:          workspaceID,
		ProjectID:            projectID,
		AppID:                appID,
		EnvironmentID:        environmentID,
		Kind:                 DeploymentTargetsKindEnvironment,
		TargetKey:            "production",
		DeploymentID:         firstDeploymentID,
		PreviousDeploymentID: sql.NullString{},
		CreatedAt:            now,
		UpdatedAt:            sql.NullInt64{},
	}))

	initialAssignment := RecordDeploymentTargetAssignmentParams{
		ID:                   uid.New("assignment"),
		TargetID:             targetID,
		WorkspaceID:          workspaceID,
		ProjectID:            projectID,
		AppID:                appID,
		EnvironmentID:        environmentID,
		DeploymentID:         firstDeploymentID,
		PreviousDeploymentID: sql.NullString{},
		Reason:               DeploymentTargetAssignmentsReasonDeploy,
		OperationID:          "deploy:" + firstDeploymentID,
		CreatedAt:            now,
	}
	require.NoError(t, Query.RecordDeploymentTargetAssignment(ctx, database.RW(), initialAssignment))
	require.NoError(t, Query.RecordDeploymentTargetAssignment(ctx, database.RW(), initialAssignment), "workflow replay must not duplicate assignment history")

	require.NoError(t, Query.AssignDeploymentTarget(ctx, database.RW(), AssignDeploymentTargetParams{
		DeploymentID: secondDeploymentID,
		UpdatedAt:    sql.NullInt64{Valid: true, Int64: now + 1},
		ID:           targetID,
	}))
	require.NoError(t, Query.RecordDeploymentTargetAssignment(ctx, database.RW(), RecordDeploymentTargetAssignmentParams{
		ID:                   uid.New("assignment"),
		TargetID:             targetID,
		WorkspaceID:          workspaceID,
		ProjectID:            projectID,
		AppID:                appID,
		EnvironmentID:        environmentID,
		DeploymentID:         secondDeploymentID,
		PreviousDeploymentID: sql.NullString{Valid: true, String: firstDeploymentID},
		Reason:               DeploymentTargetAssignmentsReasonPromote,
		OperationID:          "promote:" + firstDeploymentID + ":" + secondDeploymentID,
		CreatedAt:            now + 1,
	}))

	target, err := Query.FindDeploymentTargetByID(ctx, database.RO(), targetID)
	require.NoError(t, err)
	require.Equal(t, secondDeploymentID, target.DeploymentID)
	require.Equal(t, sql.NullString{Valid: true, String: firstDeploymentID}, target.PreviousDeploymentID)

	targets, err := Query.ListDeploymentTargetsByEnvironment(ctx, database.RO(), environmentID)
	require.NoError(t, err)
	require.Len(t, targets, 1)

	assignments, err := Query.ListDeploymentTargetAssignmentsByTarget(ctx, database.RO(), targetID)
	require.NoError(t, err)
	require.Len(t, assignments, 2)
	require.Equal(t, secondDeploymentID, assignments[0].DeploymentID)
	require.Equal(t, DeploymentTargetAssignmentsReasonPromote, assignments[0].Reason)
	require.Equal(t, firstDeploymentID, assignments[1].DeploymentID)

	require.NoError(t, Query.DeleteDeploymentTargetAssignmentsByEnvironment(ctx, database.RW(), environmentID))
	require.NoError(t, Query.DeleteDeploymentTargetsByEnvironment(ctx, database.RW(), environmentID))
	require.Equal(t, 0, countRows(t, ctx, database, "SELECT COUNT(*) FROM deployment_target_assignments WHERE environment_id = ?", environmentID))
	require.Equal(t, 0, countRows(t, ctx, database, "SELECT COUNT(*) FROM deployment_targets WHERE environment_id = ?", environmentID))
}
