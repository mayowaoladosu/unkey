-- name: InsertSlackProjectConnection :exec
INSERT INTO slack_project_connections (
    id,
    workspace_id,
    project_id,
    installation_id,
    channel_id,
    channel_name,
    include_previews,
    approval_policy,
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
    sqlc.arg(include_previews),
    sqlc.arg(approval_policy),
    sqlc.arg(created_at),
    sqlc.arg(updated_at)
);
