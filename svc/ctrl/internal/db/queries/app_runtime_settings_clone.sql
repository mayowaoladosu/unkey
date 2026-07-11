-- name: CloneAppRuntimeSettings :exec
INSERT INTO app_runtime_settings (
    workspace_id,
    app_id,
    environment_id,
    port,
    cpu_millicores,
    memory_mib,
    storage_mib,
    command,
    outputs,
    healthcheck,
    shutdown_signal,
    upstream_protocol,
    sentinel_config,
    openapi_spec_path,
    created_at,
    updated_at
)
SELECT
    source.workspace_id,
    source.app_id,
    sqlc.arg(target_environment_id),
    source.port,
    source.cpu_millicores,
    source.memory_mib,
    source.storage_mib,
    source.command,
    source.outputs,
    source.healthcheck,
    source.shutdown_signal,
    source.upstream_protocol,
    source.sentinel_config,
    source.openapi_spec_path,
    sqlc.arg(created_at),
    sqlc.arg(updated_at)
FROM app_runtime_settings AS source
WHERE source.app_id = sqlc.arg(app_id)
    AND source.environment_id = sqlc.arg(source_environment_id);