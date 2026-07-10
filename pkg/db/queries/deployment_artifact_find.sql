-- name: FindDeploymentArtifact :one
SELECT *
FROM deployment_artifacts
WHERE deployment_id = sqlc.arg(deployment_id)
  AND kind = sqlc.arg(kind)
  AND name = sqlc.arg(name);
