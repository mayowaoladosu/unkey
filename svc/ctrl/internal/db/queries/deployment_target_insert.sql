-- name: InsertDeploymentTarget :exec
INSERT INTO deployment_targets (
  id,
  workspace_id,
  project_id,
  app_id,
  environment_id,
  kind,
  target_key,
  deployment_id,
  previous_deployment_id,
  created_at,
  updated_at
) VALUES (
  sqlc.arg(id),
  sqlc.arg(workspace_id),
  sqlc.arg(project_id),
  sqlc.arg(app_id),
  sqlc.arg(environment_id),
  sqlc.arg(kind),
  sqlc.arg(target_key),
  sqlc.arg(deployment_id),
  sqlc.narg(previous_deployment_id),
  sqlc.arg(created_at),
  sqlc.narg(updated_at)
)
ON DUPLICATE KEY UPDATE id = id;
