-- name: DeleteDeploymentTargetAssignmentsByEnvironment :exec
DELETE FROM deployment_target_assignments WHERE environment_id = sqlc.arg(environment_id);

-- name: DeleteDeploymentTargetsByEnvironment :exec
DELETE FROM deployment_targets WHERE environment_id = sqlc.arg(environment_id);
