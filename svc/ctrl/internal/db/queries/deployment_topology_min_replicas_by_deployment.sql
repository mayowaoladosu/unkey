-- name: FindDeploymentTopologyMinReplicas :many
-- Returns the per-region minimum replica requirement for a deployment.
-- Used by deploy and wake workflows to compute whether enough regions are
-- healthy before a deployment is considered ready.
SELECT dt.resource_id, dt.region_id, dt.autoscaling_replicas_min
FROM deployment_topology dt
LEFT JOIN deployment_resources dr ON dt.resource_id = dr.id
WHERE dt.deployment_id = sqlc.arg(deployment_id)
	AND (dt.resource_id = '' OR dr.kind != 'cron');
