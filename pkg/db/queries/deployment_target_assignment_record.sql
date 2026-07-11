-- name: RecordDeploymentTargetAssignment :exec
INSERT INTO deployment_target_assignments (
  id,
  target_id,
  workspace_id,
  project_id,
  app_id,
  environment_id,
  deployment_id,
  previous_deployment_id,
  reason,
  operation_id,
  created_at
) VALUES (
  sqlc.arg(id),
  sqlc.arg(target_id),
  sqlc.arg(workspace_id),
  sqlc.arg(project_id),
  sqlc.arg(app_id),
  sqlc.arg(environment_id),
  sqlc.arg(deployment_id),
  sqlc.narg(previous_deployment_id),
  sqlc.arg(reason),
  sqlc.arg(operation_id),
  sqlc.arg(created_at)
)
ON DUPLICATE KEY UPDATE id = id;
