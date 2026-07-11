-- name: DeleteDeploymentResourcesByEnvironment :exec
DELETE FROM deployment_resources WHERE environment_id = sqlc.arg(environment_id);
