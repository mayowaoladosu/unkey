-- name: RecordDeploymentArtifact :exec
INSERT INTO deployment_artifacts (
    id,
    deployment_id,
    workspace_id,
    project_id,
    app_id,
    environment_id,
    name,
    kind,
    storage_key,
    digest,
    size_bytes,
    content_type,
    metadata,
    created_at
)
VALUES (
    sqlc.arg(id),
    sqlc.arg(deployment_id),
    sqlc.arg(workspace_id),
    sqlc.arg(project_id),
    sqlc.arg(app_id),
    sqlc.arg(environment_id),
    sqlc.arg(name),
    sqlc.arg(kind),
    sqlc.arg(storage_key),
    sqlc.arg(digest),
    sqlc.arg(size_bytes),
    sqlc.arg(content_type),
    sqlc.arg(metadata),
    sqlc.arg(created_at)
)
ON DUPLICATE KEY UPDATE id = deployment_artifacts.id;
