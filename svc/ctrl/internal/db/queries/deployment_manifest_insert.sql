-- name: InsertDeploymentManifest :exec
INSERT INTO deployment_manifests (
    deployment_id,
    workspace_id,
    project_id,
    app_id,
    environment_id,
    schema_version,
    fingerprint,
    adapter_id,
    output_mode,
    manifest,
    created_at
)
VALUES (
    sqlc.arg(deployment_id),
    sqlc.arg(workspace_id),
    sqlc.arg(project_id),
    sqlc.arg(app_id),
    sqlc.arg(environment_id),
    sqlc.arg(schema_version),
    sqlc.arg(fingerprint),
    sqlc.arg(adapter_id),
    sqlc.arg(output_mode),
    sqlc.arg(manifest),
    sqlc.arg(created_at)
);
