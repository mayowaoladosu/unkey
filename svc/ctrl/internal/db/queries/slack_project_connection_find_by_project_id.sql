-- name: FindSlackProjectConnectionByProjectId :one
SELECT
    pk,
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
FROM slack_project_connections
WHERE project_id = sqlc.arg(project_id);
