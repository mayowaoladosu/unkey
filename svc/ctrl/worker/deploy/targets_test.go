package deploy

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/mysql/sqlcomment"
	"github.com/unkeyed/unkey/pkg/testutil/containers"
	"github.com/unkeyed/unkey/pkg/uid"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
	"github.com/unkeyed/unkey/svc/ctrl/internal/deploymenttarget"
)

func TestEnsureFrontlineRouteTargetsMutableAliasesAndPinsImmutableURLs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mysqlCfg := containers.MySQL(t)
	database, err := db.New(mysqlCfg.DSN, sqlcomment.Disabled())
	require.NoError(t, err)
	defer func() { require.NoError(t, database.Close()) }()

	workspaceID := uid.New(uid.WorkspacePrefix)
	project := db.Project{ID: uid.New(uid.ProjectPrefix), WorkspaceID: workspaceID}
	app := db.App{ID: uid.New(uid.AppPrefix), WorkspaceID: workspaceID, ProjectID: project.ID}
	environmentID := uid.New(uid.EnvironmentPrefix)
	now := time.Now().UnixMilli()
	deployment := func(id string) db.Deployment {
		return db.Deployment{
			ID:            id,
			WorkspaceID:   workspaceID,
			ProjectID:     project.ID,
			AppID:         app.ID,
			EnvironmentID: environmentID,
		}
	}
	first := deployment(uid.New(uid.DeploymentPrefix))
	second := deployment(uid.New(uid.DeploymentPrefix))
	for _, candidate := range []db.Deployment{first, second} {
		_, err = database.RW().ExecContext(ctx, `
			INSERT INTO deployments (
				id, k8s_name, workspace_id, project_id, environment_id, app_id,
				sentinel_config, cpu_millicores, memory_mib,
				encrypted_environment_variables, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, candidate.ID, uid.DNS1035(12), workspaceID, project.ID, environmentID, app.ID, []byte(`{}`), 250, 256, []byte(`{}`), now)
		require.NoError(t, err)
	}

	ensure := func(domain newDomain, candidate db.Deployment) ensuredFrontlineRoute {
		t.Helper()
		result, txErr := db.TxWithResult(ctx, database.RW(), func(txCtx context.Context, tx db.DBTX) (ensuredFrontlineRoute, error) {
			return ensureFrontlineRoute(txCtx, tx, domain, project, app, candidate)
		})
		require.NoError(t, txErr)
		return result
	}

	mutableDomain := newDomain{
		domain:     "storefront-" + app.ID + ".example.test",
		sticky:     db.FrontlineRoutesStickyEnvironment,
		targetKind: deploymenttarget.KindEnvironment,
		targetKey:  "production",
	}
	createdMutable := ensure(mutableDomain, first)
	require.False(t, createdMutable.NeedsMove)
	require.True(t, createdMutable.TargetID.Valid)
	pendingMutable := ensure(mutableDomain, second)
	require.True(t, pendingMutable.NeedsMove)
	require.Equal(t, createdMutable.ID, pendingMutable.ID)

	immutableDomain := newDomain{
		domain:     "storefront-immutable-" + app.ID + ".example.test",
		sticky:     db.FrontlineRoutesStickyDeployment,
		targetKind: "",
		targetKey:  "",
	}
	createdImmutable := ensure(immutableDomain, first)
	require.False(t, createdImmutable.NeedsMove)
	require.False(t, createdImmutable.TargetID.Valid)
	pinnedImmutable := ensure(immutableDomain, second)
	require.False(t, pinnedImmutable.NeedsMove)
	require.Equal(t, first.ID, pinnedImmutable.Deployment)

	assignments, err := database.ListDeploymentTargetAssignmentsByEnvironment(ctx, environmentID)
	require.NoError(t, err)
	require.Len(t, assignments, 1, "target bootstrap must be recorded exactly once")
	require.Equal(t, db.DeploymentTargetAssignmentsReasonDeploy, assignments[0].Reason)
}
