-- Policy reads and writes only need the sentinel blob; use
-- FindAppRuntimeSettingsByAppAndEnv when the full settings row is needed.

-- name: FindSentinelConfigByAppAndEnv :one
SELECT sentinel_config
FROM app_runtime_settings
WHERE app_id = sqlc.arg(app_id)
  AND environment_id = sqlc.arg(environment_id);
