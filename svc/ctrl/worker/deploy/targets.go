package deploy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
	"github.com/unkeyed/unkey/svc/ctrl/internal/deploymenttarget"
)

type ensuredFrontlineRoute struct {
	ID         string
	NeedsMove  bool
	Mutable    bool
	TargetID   sql.NullString
	Deployment string
}

func ensureFrontlineRoute(
	ctx context.Context,
	tx db.DBTX,
	domain newDomain,
	project db.Project,
	app db.App,
	deployment db.Deployment,
) (ensuredFrontlineRoute, error) {
	queries := db.NewQueries(tx)
	now := time.Now().UnixMilli()

	route, routeErr := queries.FindFrontlineRouteByFQDN(ctx, domain.domain)
	routeExists := routeErr == nil
	if routeErr != nil && !db.IsNotFound(routeErr) {
		return ensuredFrontlineRoute{}, routeErr
	}

	routeID := route.ID
	initialDeploymentID := route.DeploymentID
	if !routeExists {
		routeID, routeErr = deploymenttarget.RouteID(domain.domain)
		if routeErr != nil {
			return ensuredFrontlineRoute{}, routeErr
		}
		initialDeploymentID = deployment.ID
	}

	targetID := sql.NullString{}
	mutable := domain.targetKind != ""
	if mutable {
		identity := deploymenttarget.Identity{
			AppID:         app.ID,
			EnvironmentID: deployment.EnvironmentID,
			Kind:          domain.targetKind,
			Key:           domain.targetKey,
		}
		stableTargetID, err := deploymenttarget.ID(identity)
		if err != nil {
			return ensuredFrontlineRoute{}, err
		}
		targetID = sql.NullString{Valid: true, String: stableTargetID}

		target, findErr := queries.LockDeploymentTargetByID(ctx, stableTargetID)
		targetCreated := false
		if findErr != nil {
			if !db.IsNotFound(findErr) {
				return ensuredFrontlineRoute{}, findErr
			}
			targetCreated = true
			if err := queries.InsertDeploymentTarget(ctx, db.InsertDeploymentTargetParams{
				ID:                   stableTargetID,
				WorkspaceID:          deployment.WorkspaceID,
				ProjectID:            project.ID,
				AppID:                app.ID,
				EnvironmentID:        deployment.EnvironmentID,
				Kind:                 db.DeploymentTargetsKind(domain.targetKind),
				TargetKey:            domain.targetKey,
				DeploymentID:         initialDeploymentID,
				PreviousDeploymentID: sql.NullString{},
				CreatedAt:            now,
				UpdatedAt:            sql.NullInt64{},
			}); err != nil {
				return ensuredFrontlineRoute{}, err
			}
			target, findErr = queries.LockDeploymentTargetByID(ctx, stableTargetID)
			if findErr != nil {
				return ensuredFrontlineRoute{}, findErr
			}
		}
		if target.AppID != app.ID || target.EnvironmentID != deployment.EnvironmentID || target.Kind != db.DeploymentTargetsKind(domain.targetKind) || target.TargetKey != domain.targetKey {
			return ensuredFrontlineRoute{}, fmt.Errorf("deployment target %q identity does not match alias %q", target.ID, domain.domain)
		}

		if targetCreated {
			operationID := "bootstrap:" + routeID + ":" + initialDeploymentID
			assignmentID, err := deploymenttarget.AssignmentID(stableTargetID, operationID)
			if err != nil {
				return ensuredFrontlineRoute{}, err
			}
			reason := db.DeploymentTargetAssignmentsReasonDeploy
			if routeExists {
				reason = db.DeploymentTargetAssignmentsReasonRestore
			}
			if err := queries.RecordDeploymentTargetAssignment(ctx, db.RecordDeploymentTargetAssignmentParams{
				ID:                   assignmentID,
				TargetID:             stableTargetID,
				WorkspaceID:          target.WorkspaceID,
				ProjectID:            target.ProjectID,
				AppID:                target.AppID,
				EnvironmentID:        target.EnvironmentID,
				DeploymentID:         target.DeploymentID,
				PreviousDeploymentID: target.PreviousDeploymentID,
				Reason:               reason,
				OperationID:          operationID,
				CreatedAt:            now,
			}); err != nil {
				return ensuredFrontlineRoute{}, err
			}
		}
	}

	if !routeExists {
		if err := queries.InsertFrontlineRoute(ctx, db.InsertFrontlineRouteParams{
			ID:                       routeID,
			ProjectID:                project.ID,
			AppID:                    app.ID,
			DeploymentID:             deployment.ID,
			TargetID:                 targetID,
			EnvironmentID:            deployment.EnvironmentID,
			FullyQualifiedDomainName: domain.domain,
			Sticky:                   domain.sticky,
			CreatedAt:                now,
			UpdatedAt:                sql.NullInt64{},
		}); err != nil {
			return ensuredFrontlineRoute{}, err
		}
		return ensuredFrontlineRoute{
			ID:         routeID,
			NeedsMove:  false,
			Mutable:    mutable,
			TargetID:   targetID,
			Deployment: deployment.ID,
		}, nil
	}

	if mutable {
		if route.TargetID.Valid && route.TargetID.String != targetID.String {
			return ensuredFrontlineRoute{}, errors.New("frontline alias is linked to a different deployment target")
		}
		if !route.TargetID.Valid {
			if err := queries.LinkFrontlineRouteTarget(ctx, db.LinkFrontlineRouteTargetParams{
				TargetID: targetID,
				UpdatedAt: sql.NullInt64{Valid: true, Int64: now},
				ID:        route.ID,
			}); err != nil {
				return ensuredFrontlineRoute{}, err
			}
		}
	}

	return ensuredFrontlineRoute{
		ID:         route.ID,
		NeedsMove:  mutable && route.DeploymentID != deployment.ID,
		Mutable:    mutable,
		TargetID:   targetID,
		Deployment: route.DeploymentID,
	}, nil
}
