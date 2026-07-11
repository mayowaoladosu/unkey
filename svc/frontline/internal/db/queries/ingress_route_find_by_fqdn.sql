-- name: FindFrontlineRouteByFQDN :one
-- FindFrontlineRouteByFQDN resolves a hostname to the routing data frontline
-- needs on the request path: the deployment ID, the policy bytes the engine
-- evaluates, and the upstream protocol used to pick a transport. Joining
-- deployments here keeps the fast path to a single round trip.
SELECT
  fr.environment_id,
  fr.deployment_id,
  COALESCE(public_dr.id, '') AS resource_id,
  COALESCE(public_dr.name, '') AS resource_name,
  COALESCE(public_dr.kind, '') AS resource_kind,
  d.workspace_id,
  d.project_id,
  d.app_id,
  d.sentinel_config,
  d.upstream_protocol,
  da.name AS static_output_name,
  da.storage_key AS static_storage_key,
  da.digest AS static_digest,
  da.metadata AS static_metadata
FROM frontline_routes fr
INNER JOIN deployments d ON fr.deployment_id = d.id
LEFT JOIN deployment_resources public_dr
  ON public_dr.deployment_id = d.id
  AND public_dr.public = true
  AND public_dr.pk = (
    SELECT MIN(candidate.pk)
    FROM deployment_resources candidate
    WHERE candidate.deployment_id = d.id AND candidate.public = true
  )
LEFT JOIN deployment_artifacts da
  ON da.deployment_id = d.id
  AND da.kind = 'static_bundle'
WHERE fr.fully_qualified_domain_name = sqlc.arg(fqdn);
