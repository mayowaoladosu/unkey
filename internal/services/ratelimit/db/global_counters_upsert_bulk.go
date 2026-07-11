package db

import (
	"context"

	unkeydb "github.com/unkeyed/unkey/pkg/db"
)

// BulkQueries is the receiver for generated bulk query methods. The sqlc plugin
// emits stateless methods that take a [DBTX] argument, so this type only exists
// to satisfy the generated receiver.
type BulkQueries struct{}

// BulkUpsertGlobalCounters writes per-region count observations with the
// generated bulk upsert and the database's primary connection.
func (d *Database) BulkUpsertGlobalCounters(ctx context.Context, args []UpsertRatelimitGlobalCountersParams) error {
	_, err := unkeydb.WithRetryContext(ctx, func() (struct{}, error) {
		return struct{}{}, (&BulkQueries{}).UpsertRatelimitGlobalCounters(ctx, d.rw.db, args)
	})
	return err
}
