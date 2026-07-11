-- name: FindFrontlineRouteByID :one
SELECT * FROM frontline_routes WHERE id = sqlc.arg(id);

-- name: LinkFrontlineRouteTarget :exec
UPDATE frontline_routes
SET
  target_id = sqlc.arg(target_id),
  updated_at = sqlc.narg(updated_at)
WHERE id = sqlc.arg(id);
