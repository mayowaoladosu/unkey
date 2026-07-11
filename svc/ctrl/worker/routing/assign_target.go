package routing

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
	"github.com/unkeyed/unkey/svc/ctrl/internal/deploymenttarget"
)

func assignFrontlineRoute(
	ctx context.Context,
	queries *db.Queries,
	routeID string,
	deploymentID string,
	reason db.DeploymentTargetAssignmentsReason,
	operationID string,
	now int64,
) error {
	route, err := queries.FindFrontlineRouteByID(ctx, routeID)
	if err != nil {
		return fmt.Errorf("find frontline route %q: %w", routeID, err)
	}

	targetID, err := ensureRouteTarget(ctx, queries, route, deploymentID, now)
	if err != nil {
		return err
	}
	if targetID.Valid {
		target, findErr := queries.LockDeploymentTargetByID(ctx, targetID.String)
		if findErr != nil {
			return fmt.Errorf("find deployment target %q: %w", targetID.String, findErr)
		}
		if target.DeploymentID != deploymentID {
			assignmentID, identityErr := deploymenttarget.AssignmentID(target.ID, operationID)
			if identityErr != nil {
				return identityErr
			}
			if assignErr := queries.AssignDeploymentTarget(ctx, db.AssignDeploymentTargetParams{
				DeploymentID: deploymentID,
				UpdatedAt:    sql.NullInt64{Valid: true, Int64: now},
				ID:           target.ID,
			}); assignErr != nil {
				return fmt.Errorf("assign deployment target %q: %w", target.ID, assignErr)
			}
			if recordErr := queries.RecordDeploymentTargetAssignment(ctx, db.RecordDeploymentTargetAssignmentParams{
				ID:                   assignmentID,
				TargetID:             target.ID,
				WorkspaceID:          target.WorkspaceID,
				ProjectID:            target.ProjectID,
				AppID:                target.AppID,
				EnvironmentID:        target.EnvironmentID,
				DeploymentID:         deploymentID,
				PreviousDeploymentID: sql.NullString{Valid: true, String: target.DeploymentID},
				Reason:               reason,
				OperationID:          operationID,
				CreatedAt:            now,
			}); recordErr != nil {
				return fmt.Errorf("record deployment target assignment %q: %w", target.ID, recordErr)
			}
		}
	}

	if route.DeploymentID == deploymentID {
		return nil
	}
	if err := queries.ReassignFrontlineRoute(ctx, db.ReassignFrontlineRouteParams{
		ID:           routeID,
		DeploymentID: deploymentID,
		UpdatedAt:    sql.NullInt64{Valid: true, Int64: now},
	}); err != nil {
		return fmt.Errorf("reassign frontline route %q: %w", routeID, err)
	}
	if err := queries.BumpFrontlineRouteRevision(ctx); err != nil {
		return fmt.Errorf("bump frontline route revision: %w", err)
	}
	return nil
}

func ensureRouteTarget(
	ctx context.Context,
	queries *db.Queries,
	route db.FrontlineRoute,
	requestedDeploymentID string,
	now int64,
) (sql.NullString, error) {
	if route.TargetID.Valid {
		return route.TargetID, nil
	}

	var kind deploymenttarget.Kind
	var targetKey string
	switch route.Sticky {
	case db.FrontlineRoutesStickyBranch:
		kind = deploymenttarget.KindBranch
	case db.FrontlineRoutesStickyEnvironment:
		kind = deploymenttarget.KindEnvironment
	case db.FrontlineRoutesStickyLive:
		kind = deploymenttarget.KindLive
		targetKey = "live"
	default:
		return sql.NullString{}, nil
	}

	seedDeploymentID := route.DeploymentID
	if seedDeploymentID == "" {
		seedDeploymentID = requestedDeploymentID
	}
	seedDeployment, err := queries.FindDeploymentById(ctx, seedDeploymentID)
	if err != nil {
		return sql.NullString{}, fmt.Errorf("find deployment for targetless alias %q: %w", route.ID, err)
	}
	switch kind {
	case deploymenttarget.KindBranch:
		if !seedDeployment.GitBranch.Valid || seedDeployment.GitBranch.String == "" {
			return sql.NullString{}, fmt.Errorf("targetless branch alias %q has no branch provenance", route.ID)
		}
		targetKey = seedDeployment.GitBranch.String
	case deploymenttarget.KindEnvironment:
		environment, findErr := queries.FindEnvironmentById(ctx, route.EnvironmentID)
		if findErr != nil {
			return sql.NullString{}, fmt.Errorf("find environment for targetless alias %q: %w", route.ID, findErr)
		}
		targetKey = environment.Slug
	case deploymenttarget.KindLive:
		// The key is the single live target in this environment.
	}

	stableTargetID, err := deploymenttarget.ID(deploymenttarget.Identity{
		AppID:         route.AppID,
		EnvironmentID: route.EnvironmentID,
		Kind:          kind,
		Key:           targetKey,
	})
	if err != nil {
		return sql.NullString{}, err
	}
	target, findErr := queries.LockDeploymentTargetByID(ctx, stableTargetID)
	targetCreated := false
	if findErr != nil {
		if !db.IsNotFound(findErr) {
			return sql.NullString{}, findErr
		}
		targetCreated = true
		if insertErr := queries.InsertDeploymentTarget(ctx, db.InsertDeploymentTargetParams{
			ID:                   stableTargetID,
			WorkspaceID:          seedDeployment.WorkspaceID,
			ProjectID:            route.ProjectID,
			AppID:                route.AppID,
			EnvironmentID:        route.EnvironmentID,
			Kind:                 db.DeploymentTargetsKind(kind),
			TargetKey:            targetKey,
			DeploymentID:         seedDeploymentID,
			PreviousDeploymentID: sql.NullString{},
			CreatedAt:            now,
			UpdatedAt:            sql.NullInt64{},
		}); insertErr != nil {
			return sql.NullString{}, insertErr
		}
		target, findErr = queries.LockDeploymentTargetByID(ctx, stableTargetID)
		if findErr != nil {
			return sql.NullString{}, findErr
		}
	}
	if target.AppID != route.AppID || target.EnvironmentID != route.EnvironmentID || target.Kind != db.DeploymentTargetsKind(kind) || target.TargetKey != targetKey {
		return sql.NullString{}, fmt.Errorf("deployment target %q identity does not match alias %q", target.ID, route.ID)
	}

	if err := queries.LinkFrontlineRouteTarget(ctx, db.LinkFrontlineRouteTargetParams{
		TargetID: sql.NullString{Valid: true, String: stableTargetID},
		UpdatedAt: sql.NullInt64{Valid: true, Int64: now},
		ID:        route.ID,
	}); err != nil {
		return sql.NullString{}, fmt.Errorf("link targetless alias %q: %w", route.ID, err)
	}

	if targetCreated {
		bootstrapOperationID := "bootstrap:" + route.ID + ":" + seedDeploymentID
		assignmentID, identityErr := deploymenttarget.AssignmentID(stableTargetID, bootstrapOperationID)
		if identityErr != nil {
			return sql.NullString{}, identityErr
		}
		reason := db.DeploymentTargetAssignmentsReasonRestore
		if route.DeploymentID == "" {
			reason = db.DeploymentTargetAssignmentsReasonDeploy
		}
		if recordErr := queries.RecordDeploymentTargetAssignment(ctx, db.RecordDeploymentTargetAssignmentParams{
			ID:                   assignmentID,
			TargetID:             stableTargetID,
			WorkspaceID:          target.WorkspaceID,
			ProjectID:            target.ProjectID,
			AppID:                target.AppID,
			EnvironmentID:        target.EnvironmentID,
			DeploymentID:         target.DeploymentID,
			PreviousDeploymentID: target.PreviousDeploymentID,
			Reason:               reason,
			OperationID:          bootstrapOperationID,
			CreatedAt:            now,
		}); recordErr != nil {
			return sql.NullString{}, recordErr
		}
	}

	return sql.NullString{Valid: true, String: stableTargetID}, nil
}
