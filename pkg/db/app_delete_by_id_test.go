package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/testutil/containers"
	"github.com/unkeyed/unkey/pkg/uid"
)

func TestDeleteAppById_CleansFrameworkDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	mysqlCfg := containers.MySQL(t)
	database, err := New(Config{PrimaryDSN: mysqlCfg.DSN})
	require.NoError(t, err)
	defer func() { require.NoError(t, database.Close()) }()

	for _, withDetection := range []bool{true, false} {
		name := "without detection"
		if withDetection {
			name = "with detection"
		}

		t.Run(name, func(t *testing.T) {
			appID := uid.New("app")
			projectID := uid.New("prj")
			workspaceID := uid.New("ws")
			now := time.Now().UnixMilli()

			_, err := database.RW().ExecContext(ctx, `
				INSERT INTO apps (
					id, workspace_id, project_id, name, slug, default_branch, created_at
				) VALUES (?, ?, ?, ?, ?, ?, ?)
			`, appID, workspaceID, projectID, name, uid.New("slug"), "main", now)
			require.NoError(t, err)

			if withDetection {
				_, err = database.RW().ExecContext(ctx, `
					INSERT INTO app_framework_detections (
						workspace_id, project_id, app_id, repository_full_name, branch,
						tree_sha, fingerprint, detection_version, confidence, build_strategy,
						detection, defaults, detected_at, created_at
					) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				`,
					workspaceID,
					projectID,
					appID,
					"unkeyed/test-repo",
					"main",
					"0123456789abcdef0123456789abcdef01234567",
					"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
					1,
					"high",
					"zero-config",
					[]byte(`{}`),
					[]byte(`{}`),
					now,
					now,
				)
				require.NoError(t, err)
			}

			require.NoError(t, Query.DeleteAppById(ctx, database.RW(), appID))
			require.Equal(t, 0, countRows(t, ctx, database, "SELECT COUNT(*) FROM apps WHERE id = ?", appID))
			require.Equal(t, 0, countRows(t, ctx, database, "SELECT COUNT(*) FROM app_framework_detections WHERE app_id = ?", appID))
		})
	}
}

//nolint:gosec // query is always a test constant
func countRows(t *testing.T, ctx context.Context, database Database, query string, appID string) int {
	t.Helper()

	var count int
	require.NoError(t, database.RO().QueryRowContext(ctx, query, appID).Scan(&count))
	return count
}
