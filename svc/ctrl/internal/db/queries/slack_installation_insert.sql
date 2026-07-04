-- name: InsertSlackInstallation :exec
INSERT INTO slack_installations (
    id,
    workspace_id,
    team_id,
    bot_token,
    bot_user_id,
    installed_by_user_id,
    created_at,
    updated_at
)
VALUES (
    sqlc.arg(id),
    sqlc.arg(workspace_id),
    sqlc.arg(team_id),
    sqlc.arg(bot_token),
    sqlc.arg(bot_user_id),
    sqlc.arg(installed_by_user_id),
    sqlc.arg(created_at),
    sqlc.arg(updated_at)
);
