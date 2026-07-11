-- name: BumpFrontlineRouteRevision :exec
INSERT INTO frontline_route_revisions (id, revision)
VALUES (1, 1)
ON DUPLICATE KEY UPDATE revision = revision + 1;
