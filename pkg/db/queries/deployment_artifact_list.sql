-- name: ListDeploymentArtifacts :many
SELECT *
FROM deployment_artifacts
WHERE deployment_id = sqlc.arg(deployment_id)
ORDER BY created_at ASC, name ASC;
