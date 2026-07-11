package deploy

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	restate "github.com/restatedev/sdk-go"
	hydrav1 "github.com/unkeyed/unkey/gen/proto/hydra/v1"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
	"github.com/unkeyed/unkey/svc/ctrl/internal/deploymanifest"
)

// prepareDeploymentForTraffic cancels pending standby work and restores a
// retained deployment before any alias is moved to it. Container deployments
// wait for healthy instances; static-only deployments only need their durable
// desired-state marker restored because Frontline serves their artifact.
func (w *Workflow) prepareDeploymentForTraffic(
	ctx restate.ObjectContext,
	deployment db.Deployment,
) error {
	_, err := hydrav1.NewDeploymentServiceClient(ctx, deployment.ID).
		ClearScheduledStateChanges().
		Request(&hydrav1.ClearScheduledStateChangesRequest{})
	if err != nil {
		return fmt.Errorf("clear scheduled state changes: %w", err)
	}
	if deployment.DesiredState != db.DeploymentsDesiredStateStopped {
		return nil
	}

	staticOnly := false
	manifestRow, manifestErr := restate.Run(ctx, func(runCtx restate.RunContext) (db.DeploymentManifest, error) {
		return w.db.FindDeploymentManifestByDeploymentID(runCtx, deployment.ID)
	}, restate.WithName("find retained deployment manifest"), restate.WithMaxRetryAttempts(runMaxAttempts))
	if manifestErr == nil {
		manifest, parseErr := deploymanifest.Parse(manifestRow.Manifest)
		if parseErr != nil {
			return fmt.Errorf("parse retained deployment manifest: %w", parseErr)
		}
		staticOnly = isStaticOnlyDeployment(manifest)
	} else if !db.IsNotFound(manifestErr) {
		return fmt.Errorf("find retained deployment manifest: %w", manifestErr)
	}

	if staticOnly {
		return restate.RunVoid(ctx, func(runCtx restate.RunContext) error {
			return db.Tx(runCtx, w.db.RW(), func(txCtx context.Context, tx db.DBTX) error {
				queries := db.NewQueries(tx)
				now := sql.NullInt64{Valid: true, Int64: time.Now().UnixMilli()}
				if updateErr := queries.UpdateDeploymentDesiredState(txCtx, db.UpdateDeploymentDesiredStateParams{
					ID:           deployment.ID,
					DesiredState: db.DeploymentsDesiredStateRunning,
					UpdatedAt:    now,
				}); updateErr != nil {
					return updateErr
				}
				return queries.UpdateDeploymentStatus(txCtx, db.UpdateDeploymentStatusParams{
					ID:        deployment.ID,
					Status:    db.DeploymentsStatusReady,
					UpdatedAt: now,
				})
			})
		}, restate.WithName("restore static deployment state"), restate.WithMaxRetryAttempts(runMaxAttempts))
	}

	if restate.Key(ctx) == deployment.ID {
		return w.wakeStoppedDeployment(ctx, deployment)
	}

	_, err = hydrav1.NewDeployServiceClient(ctx, deployment.ID).
		WakeDeployment().
		Request(&hydrav1.WakeDeploymentRequest{DeploymentId: deployment.ID})
	if err != nil {
		return fmt.Errorf("wake retained deployment: %w", err)
	}
	return nil
}
