package clickhouse_test

import (
	"context"
	"testing"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/testutil/containers"
)

func TestInstanceCheckpointsViewIncludesDeploymentResourceDimensions(t *testing.T) {
	t.Parallel()

	cfg := containers.ClickHouse(t)
	opts, err := ch.ParseDSN(cfg.DSN)
	require.NoError(t, err)
	conn, err := ch.Open(opts)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	ctx := context.Background()
	rows, err := conn.Query(ctx, `
		SELECT name
		FROM system.columns
		WHERE database = 'default'
		  AND table = 'instance_checkpoints'
		  AND name LIKE 'deployment_resource_%'
		ORDER BY name
	`)
	require.NoError(t, err)
	t.Cleanup(func() { rows.Close() })

	var columns []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		columns = append(columns, name)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{
		"deployment_resource_id",
		"deployment_resource_kind",
		"deployment_resource_name",
	}, columns)

	var count uint64
	err = conn.QueryRow(
		ctx,
		"SELECT count() FROM default.instance_checkpoints WHERE deployment_resource_id = ''",
	).Scan(&count)
	require.NoError(t, err)
}
