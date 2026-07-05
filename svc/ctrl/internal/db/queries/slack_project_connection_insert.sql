-- name: InsertSlackProjectConnection :exec
INSERT INTO slack_project_connections (
    id,
    workspace_id,
    project_id,
    installation_id,
    channel_id,
    channel_name,
    notify_production,
    notify_previews,
    created_at,
    updated_at
)
VALUES (
    sqlc.arg(id),
    sqlc.arg(workspace_id),
    sqlc.arg(project_id),
    sqlc.arg(installation_id),
    sqlc.arg(channel_id),
    sqlc.arg(channel_name),
    sqlc.arg(notify_production),
    sqlc.arg(notify_previews),
    sqlc.arg(created_at),
    sqlc.arg(updated_at)
);
