-- name: FindDeploymentResourceByID :one
SELECT * FROM deployment_resources WHERE id = sqlc.arg(id);

-- name: FindDeploymentResourceByK8sName :one
SELECT * FROM deployment_resources WHERE k8s_name = sqlc.arg(k8s_name);

-- name: ListDeploymentResourcesByDeployment :many
SELECT *
FROM deployment_resources
WHERE deployment_id = sqlc.arg(deployment_id)
ORDER BY name ASC;
