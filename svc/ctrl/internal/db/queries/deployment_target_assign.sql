-- name: AssignDeploymentTarget :exec
UPDATE deployment_targets
SET
  previous_deployment_id = deployment_id,
  deployment_id = sqlc.arg(deployment_id),
  updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);
