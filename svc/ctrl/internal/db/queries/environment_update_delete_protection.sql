-- name: UpdateEnvironmentDeleteProtection :exec
UPDATE environments
SET delete_protection = sqlc.arg(delete_protection),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);