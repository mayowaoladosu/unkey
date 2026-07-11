package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/mysql/sqlcomment"
	"github.com/unkeyed/unkey/pkg/testutil/containers"
	"github.com/unkeyed/unkey/pkg/uid"
)

func TestFrontlineRouteRevisionChangesForInsertMoveAndDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mysqlCfg := containers.MySQL(t)
	queries, closeQueries, err := New(mysqlCfg.DSN, sqlcomment.Disabled())
	require.NoError(t, err)
	defer func() { require.NoError(t, closeQueries()) }()

	database, err := sql.Open("mysql", mysqlCfg.DSN)
	require.NoError(t, err)
	defer func() { require.NoError(t, database.Close()) }()

	initial, err := queries.FindFrontlineRouteRevision(ctx)
	require.NoError(t, err)
	bump := func() {
		t.Helper()
		_, bumpErr := database.ExecContext(ctx, `
			INSERT INTO frontline_route_revisions (id, revision) VALUES (1, 1)
			ON DUPLICATE KEY UPDATE revision = revision + 1
		`)
		require.NoError(t, bumpErr)
	}
	routeID := uid.New(uid.FrontlineRoutePrefix)
	domain := routeID + ".example.test"
	now := time.Now().UnixMilli()
	_, err = database.ExecContext(ctx, `
		INSERT INTO frontline_routes (
			id, project_id, app_id, deployment_id, environment_id,
			fully_qualified_domain_name, sticky, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, 'environment', ?, ?)
	`, routeID, uid.New(uid.ProjectPrefix), uid.New(uid.AppPrefix), "d_first", uid.New(uid.EnvironmentPrefix), domain, now, now)
	require.NoError(t, err)
	bump()

	afterInsert, err := queries.FindFrontlineRouteRevision(ctx)
	require.NoError(t, err)
	require.NotEqual(t, initial, afterInsert)

	_, err = database.ExecContext(ctx, `UPDATE frontline_routes SET deployment_id = ?, updated_at = ? WHERE id = ?`, "d_second", now, routeID)
	require.NoError(t, err)
	bump()
	afterMove, err := queries.FindFrontlineRouteRevision(ctx)
	require.NoError(t, err)
	require.NotEqual(t, afterInsert, afterMove, "deployment pointer changes must invalidate even within the same millisecond")

	_, err = database.ExecContext(ctx, `DELETE FROM frontline_routes WHERE id = ?`, routeID)
	require.NoError(t, err)
	bump()
	afterDelete, err := queries.FindFrontlineRouteRevision(ctx)
	require.NoError(t, err)
	require.NotEqual(t, afterMove, afterDelete)
}
