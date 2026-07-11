-- name: DeleteFrontlineRoutesByDeploymentID :exec
DELETE FROM frontline_routes
WHERE deployment_id = sqlc.arg('deployment_id');
