-- name: UpdateDeploymentResourceTopologyDesiredStatus :exec
UPDATE deployment_topology
SET desired_status = sqlc.arg(desired_status), updated_at = sqlc.arg(updated_at)
WHERE deployment_id = sqlc.arg(deployment_id)
  AND resource_id = sqlc.arg(resource_id)
  AND region_id = sqlc.arg(region_id);
