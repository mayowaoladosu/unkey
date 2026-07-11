-- name: UpdateEnvironment :exec
UPDATE environments
SET slug = sqlc.arg(slug),
    description = sqlc.arg(description),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);