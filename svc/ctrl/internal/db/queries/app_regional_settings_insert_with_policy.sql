-- name: InsertAppRegionalSettingsWithPolicy :exec
INSERT INTO app_regional_settings (
    workspace_id,
    app_id,
    environment_id,
    region_id,
    replicas,
    horizontal_autoscaling_policy_id,
    created_at,
    updated_at
) VALUES (
    sqlc.arg(workspace_id),
    sqlc.arg(app_id),
    sqlc.arg(environment_id),
    sqlc.arg(region_id),
    sqlc.arg(replicas),
    sqlc.arg(horizontal_autoscaling_policy_id),
    sqlc.arg(created_at),
    sqlc.arg(updated_at)
);