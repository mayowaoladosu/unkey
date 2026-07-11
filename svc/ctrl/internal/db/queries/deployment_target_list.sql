-- name: ListDeploymentTargetsByEnvironment :many
SELECT *
FROM deployment_targets
WHERE environment_id = sqlc.arg(environment_id)
ORDER BY kind ASC, target_key ASC;

-- name: ListDeploymentTargetAssignmentsByEnvironment :many
SELECT *
FROM deployment_target_assignments
WHERE environment_id = sqlc.arg(environment_id)
ORDER BY created_at DESC, pk DESC;

-- name: ListDeploymentTargetAssignmentsByTarget :many
SELECT *
FROM deployment_target_assignments
WHERE target_id = sqlc.arg(target_id)
ORDER BY created_at DESC, pk DESC;
