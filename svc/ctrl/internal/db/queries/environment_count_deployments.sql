-- name: CountDeploymentsByEnvironmentID :one
SELECT COUNT(*)
FROM deployments
WHERE environment_id = sqlc.arg(environment_id);