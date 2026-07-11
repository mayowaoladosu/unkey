-- name: FindInstancesByDeploymentID :many
-- FindInstancesByDeploymentID returns all instances for a given deployment
-- with region metadata for instance-aware routing decisions.
SELECT
  i.*,
  COALESCE(dr.name, '') AS resource_name,
  COALESCE(dr.kind, '') AS resource_kind,
  r.name AS region_name,
  r.platform AS region_platform
FROM instances i
INNER JOIN regions r ON i.region_id = r.id
LEFT JOIN deployment_resources dr ON i.resource_id = dr.id
WHERE i.deployment_id = sqlc.arg(deployment_id)
  AND (i.resource_id = '' OR dr.public = true);
