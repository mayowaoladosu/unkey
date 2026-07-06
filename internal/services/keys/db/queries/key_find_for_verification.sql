-- name: FindKeyForVerification :one
-- FindKeyForVerification loads a key by its SHA-256 hash together with its
-- workspace status, RBAC roles and permissions, identity, and rate limit
-- configuration in a single round trip. Roles, permissions, and rate limits
-- are returned as JSON arrays via JSON_ARRAYAGG so the caller can unmarshal
-- them into typed Go structs. Key-level and identity-level rate limits are
-- unioned so that both sources are available for the verification pipeline.
--
-- hash_idx resolves the key row first. Correlated aggregates drive from
-- keys_roles / keys_permissions / ratelimits keyed by k.id so MySQL does not
-- scan workspace-wide role or permission catalogs (see EXPLAIN plans).
SELECT
  k.id,
  k.key_auth_id,
  k.workspace_id,
  k.for_workspace_id,
  k.name,
  k.meta,
  k.expires,
  k.deleted_at_m,
  k.refill_day,
  k.refill_amount,
  k.last_refill_at,
  k.enabled,
  k.remaining_requests,
  k.pending_migration_id,
  a.ip_whitelist,
  a.workspace_id AS api_workspace_id,
  a.id AS api_id,
  a.deleted_at_m AS api_deleted_at_m,
  COALESCE(
    (
      SELECT
        JSON_ARRAYAGG(r.name)
      FROM
        keys_roles kr
        STRAIGHT_JOIN roles r ON r.id = kr.role_id
      WHERE
        kr.key_id = k.id
    ),
    JSON_ARRAY()
  ) AS roles,
  COALESCE(
    (
      SELECT
        JSON_ARRAYAGG(slug)
      FROM
        (
          SELECT
            p.slug
          FROM
            keys_permissions kp
            STRAIGHT_JOIN permissions p ON p.id = kp.permission_id
          WHERE
            kp.key_id = k.id
          UNION ALL
          SELECT
            p.slug
          FROM
            keys_roles kr
            STRAIGHT_JOIN roles_permissions rp ON rp.role_id = kr.role_id
            STRAIGHT_JOIN permissions p ON p.id = rp.permission_id
          WHERE
            kr.key_id = k.id
        ) combined_perms
    ),
    JSON_ARRAY()
  ) AS permissions,
  COALESCE(
    (
      SELECT
        JSON_ARRAYAGG(
          JSON_OBJECT(
            'id',
            rl.id,
            'name',
            rl.name,
            'key_id',
            rl.key_id,
            'identity_id',
            rl.identity_id,
            'limit',
            rl.`limit`,
            'duration',
            rl.duration,
            'auto_apply',
            rl.auto_apply
          )
        )
      FROM
        (
          SELECT
            rl.id,
            rl.name,
            rl.key_id,
            rl.identity_id,
            rl.`limit`,
            rl.duration,
            rl.auto_apply
          FROM
            `ratelimits` rl
          WHERE
            rl.key_id = k.id
          UNION ALL
          SELECT
            rl.id,
            rl.name,
            rl.key_id,
            rl.identity_id,
            rl.`limit`,
            rl.duration,
            rl.auto_apply
          FROM
            `ratelimits` rl
          WHERE
            k.identity_id IS NOT NULL
            AND rl.identity_id = k.identity_id
        ) rl
    ),
    JSON_ARRAY()
  ) AS ratelimits,
  i.id AS identity_id,
  i.external_id,
  i.meta AS identity_meta,
  ka.deleted_at_m AS key_auth_deleted_at_m,
  ws.enabled AS workspace_enabled,
  fws.enabled AS for_workspace_enabled
FROM
  `keys` k
  JOIN apis a ON a.key_auth_id = k.key_auth_id
  JOIN key_auth ka ON ka.id = k.key_auth_id
  JOIN workspaces ws ON ws.id = k.workspace_id
  LEFT JOIN workspaces fws ON fws.id = k.for_workspace_id
  LEFT JOIN identities i ON k.identity_id = i.id
  AND i.deleted = 0
WHERE
  k.hash = sqlc.arg(hash)
  AND k.deleted_at_m IS NULL;
