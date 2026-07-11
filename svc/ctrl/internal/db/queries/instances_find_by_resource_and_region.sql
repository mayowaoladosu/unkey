-- name: FindInstancesByResourceAndRegion :many
SELECT *
FROM instances
WHERE resource_id = sqlc.arg(resource_id)
  AND region_id = sqlc.arg(region_id);

-- name: DeleteResourceInstances :exec
DELETE FROM instances
WHERE resource_id = sqlc.arg(resource_id)
  AND region_id = sqlc.arg(region_id);