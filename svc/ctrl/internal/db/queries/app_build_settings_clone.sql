-- name: CloneAppBuildSettings :exec
INSERT INTO app_build_settings (
    workspace_id,
    app_id,
    environment_id,
    dockerfile,
    docker_context,
    build_command,
    watch_paths,
    auto_deploy,
    created_at,
    updated_at
)
SELECT
    source.workspace_id,
    source.app_id,
    sqlc.arg(target_environment_id),
    source.dockerfile,
    source.docker_context,
    source.build_command,
    source.watch_paths,
    source.auto_deploy,
    sqlc.arg(created_at),
    sqlc.arg(updated_at)
FROM app_build_settings AS source
WHERE source.app_id = sqlc.arg(app_id)
    AND source.environment_id = sqlc.arg(source_environment_id);