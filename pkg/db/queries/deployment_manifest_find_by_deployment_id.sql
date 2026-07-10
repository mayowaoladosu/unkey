-- name: FindDeploymentManifestByDeploymentID :one
SELECT *
FROM deployment_manifests
WHERE deployment_id = sqlc.arg(deployment_id);
