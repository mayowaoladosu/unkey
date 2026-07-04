-- name: FindSlackInstallationById :one
SELECT
    pk,
    id,
    workspace_id,
    team_id,
    bot_token,
    bot_user_id,
    installed_by_user_id,
    created_at,
    updated_at
FROM slack_installations
WHERE id = sqlc.arg(id);
