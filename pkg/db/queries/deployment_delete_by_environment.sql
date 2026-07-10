-- name: DeleteDeploymentsByEnvironmentId :exec
DELETE deployment_manifests, deployments
FROM deployments
LEFT JOIN deployment_manifests ON deployment_manifests.deployment_id = deployments.id
WHERE deployments.environment_id = sqlc.arg(environment_id);
