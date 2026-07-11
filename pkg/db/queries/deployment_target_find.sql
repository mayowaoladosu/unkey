-- name: FindDeploymentTargetByID :one
SELECT * FROM deployment_targets WHERE id = sqlc.arg(id);

-- name: LockDeploymentTargetByID :one
SELECT * FROM deployment_targets WHERE id = sqlc.arg(id) FOR UPDATE;

-- name: FindDeploymentTargetByIdentity :one
SELECT *
FROM deployment_targets
WHERE app_id = sqlc.arg(app_id)
  AND environment_id = sqlc.arg(environment_id)
  AND kind = sqlc.arg(kind)
  AND target_key = sqlc.arg(target_key);
