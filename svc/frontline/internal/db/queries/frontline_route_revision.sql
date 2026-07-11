-- name: FindFrontlineRouteRevision :one
SELECT CAST(
  COALESCE(
    (SELECT revision FROM frontline_route_revisions WHERE id = 1),
    0
  ) AS SIGNED
) AS revision
;