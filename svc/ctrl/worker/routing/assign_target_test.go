package routing

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

func TestAssignFrontlineRouteMovesTargetAndRecordsIdempotentHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mysqlCfg := containers.MySQL(t)
	database, err := db.New(mysqlCfg.DSN, sqlcomment.Disabled())
	require.NoError(t, err)
	defer func() { require.NoError(t, database.Close()) }()

	workspaceID := uid.New(uid.WorkspacePrefix)
	projectID := uid.New(uid.ProjectPrefix)
	appID := uid.New(uid.AppPrefix)
	environmentID := uid.New(uid.EnvironmentPrefix)
	firstDeploymentID := uid.New(uid.DeploymentPrefix)
	secondDeploymentID := uid.New(uid.DeploymentPrefix)
	targetID := uid.New("target")
	routeID := uid.New(uid.FrontlineRoutePrefix)
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

	require.NoError(t, database.InsertDeploymentTarget(ctx, db.InsertDeploymentTargetParams{
		ID:                   targetID,
		WorkspaceID:          workspaceID,
		ProjectID:            projectID,
		AppID:                appID,
		EnvironmentID:        environmentID,
		Kind:                 db.DeploymentTargetsKindEnvironment,
		TargetKey:            "production",
		DeploymentID:         firstDeploymentID,
		PreviousDeploymentID: sql.NullString{},
		CreatedAt:            now,
		UpdatedAt:            sql.NullInt64{},
	}))
	require.NoError(t, database.InsertFrontlineRoute(ctx, db.InsertFrontlineRouteParams{
		ID:                       routeID,
		ProjectID:                projectID,
		AppID:                    appID,
		DeploymentID:             firstDeploymentID,
		TargetID:                 sql.NullString{Valid: true, String: targetID},
		EnvironmentID:            environmentID,
		FullyQualifiedDomainName: "app-" + routeID + ".example.test",
		Sticky:                   db.FrontlineRoutesStickyEnvironment,
		CreatedAt:                now,
		UpdatedAt:                sql.NullInt64{},
	}))

	assign := func(deploymentID string, reason db.DeploymentTargetAssignmentsReason, operationID string, assignedAt int64) {
		t.Helper()
		require.NoError(t, db.Tx(ctx, database.RW(), func(txCtx context.Context, tx db.DBTX) error {
			return assignFrontlineRoute(
				txCtx,
				db.NewQueries(tx),
				routeID,
				deploymentID,
				reason,
				operationID,
				assignedAt,
			)
		}))
	}

	assign(secondDeploymentID, db.DeploymentTargetAssignmentsReasonPromote, "invocation_promote", now+1)
	assign(secondDeploymentID, db.DeploymentTargetAssignmentsReasonPromote, "invocation_promote", now+1)

	route, err := database.FindFrontlineRouteByID(ctx, routeID)
	require.NoError(t, err)
	require.Equal(t, secondDeploymentID, route.DeploymentID)
	target, err := database.FindDeploymentTargetByID(ctx, targetID)
	require.NoError(t, err)
	require.Equal(t, secondDeploymentID, target.DeploymentID)
	require.Equal(t, sql.NullString{Valid: true, String: firstDeploymentID}, target.PreviousDeploymentID)
	assignments, err := database.ListDeploymentTargetAssignmentsByTarget(ctx, targetID)
	require.NoError(t, err)
	require.Len(t, assignments, 1)
	require.Equal(t, firstDeploymentID, assignments[0].PreviousDeploymentID.String)

	assign(firstDeploymentID, db.DeploymentTargetAssignmentsReasonRollback, "invocation_rollback", now+2)
	target, err = database.FindDeploymentTargetByID(ctx, targetID)
	require.NoError(t, err)
	require.Equal(t, firstDeploymentID, target.DeploymentID)
	require.Equal(t, sql.NullString{Valid: true, String: secondDeploymentID}, target.PreviousDeploymentID)
	assignments, err = database.ListDeploymentTargetAssignmentsByTarget(ctx, targetID)
	require.NoError(t, err)
	require.Len(t, assignments, 2)
	require.Equal(t, db.DeploymentTargetAssignmentsReasonRollback, assignments[0].Reason)

	targetlessRouteID := uid.New(uid.FrontlineRoutePrefix)
	require.NoError(t, database.InsertFrontlineRoute(ctx, db.InsertFrontlineRouteParams{
		ID:                       targetlessRouteID,
		ProjectID:                projectID,
		AppID:                    appID,
		DeploymentID:             secondDeploymentID,
		TargetID:                 sql.NullString{},
		EnvironmentID:            environmentID,
		FullyQualifiedDomainName: "custom-" + targetlessRouteID + ".example.test",
		Sticky:                   db.FrontlineRoutesStickyLive,
		CreatedAt:                now,
		UpdatedAt:                sql.NullInt64{},
	}))
	require.NoError(t, db.Tx(ctx, database.RW(), func(txCtx context.Context, tx db.DBTX) error {
		return assignFrontlineRoute(
			txCtx,
			db.NewQueries(tx),
			targetlessRouteID,
			firstDeploymentID,
			db.DeploymentTargetAssignmentsReasonRollback,
			"invocation_custom_domain",
			now+3,
		)
	}))
	targetlessRoute, err := database.FindFrontlineRouteByID(ctx, targetlessRouteID)
	require.NoError(t, err)
	require.True(t, targetlessRoute.TargetID.Valid)
	require.Equal(t, firstDeploymentID, targetlessRoute.DeploymentID)
	liveTarget, err := database.FindDeploymentTargetByID(ctx, targetlessRoute.TargetID.String)
	require.NoError(t, err)
	require.Equal(t, db.DeploymentTargetsKindLive, liveTarget.Kind)
	require.Equal(t, "live", liveTarget.TargetKey)
	require.Equal(t, firstDeploymentID, liveTarget.DeploymentID)
	liveHistory, err := database.ListDeploymentTargetAssignmentsByTarget(ctx, liveTarget.ID)
	require.NoError(t, err)
	require.Len(t, liveHistory, 2)
	require.Equal(t, db.DeploymentTargetAssignmentsReasonRollback, liveHistory[0].Reason)
	require.Equal(t, db.DeploymentTargetAssignmentsReasonRestore, liveHistory[1].Reason)
}
