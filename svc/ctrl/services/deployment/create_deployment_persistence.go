package deployment

import (
	"context"
	"fmt"

	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
)

func persistDeploymentWithManifest(
	ctx context.Context,
	database db.Database,
	deployment db.InsertDeploymentParams,
	manifest db.InsertDeploymentManifestParams,
) error {
	return db.Tx(ctx, database.RW(), func(txCtx context.Context, tx db.DBTX) error {
		queries := db.NewQueries(tx)
		if err := queries.InsertDeployment(txCtx, deployment); err != nil {
			return fmt.Errorf("insert deployment: %w", err)
		}
		if err := queries.InsertDeploymentManifest(txCtx, manifest); err != nil {
			return fmt.Errorf("insert deployment manifest: %w", err)
		}
		return nil
	})
}
