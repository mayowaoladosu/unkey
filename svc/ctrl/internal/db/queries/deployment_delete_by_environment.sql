-- name: DeleteDeploymentsByEnvironmentId :exec
DELETE deployment_artifacts, deployment_manifests, deployments
FROM deployments
LEFT JOIN deployment_manifests ON deployment_manifests.deployment_id = deployments.id
LEFT JOIN deployment_artifacts ON deployment_artifacts.deployment_id = deployments.id
WHERE deployments.environment_id = sqlc.arg(environment_id);
