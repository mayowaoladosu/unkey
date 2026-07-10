-- name: FindAppliedFrameworkDetection :one
SELECT
    fingerprint,
    detection_version,
    detected_preset_id,
    detection,
    defaults
FROM app_framework_detections
WHERE workspace_id = sqlc.arg(workspace_id)
  AND project_id = sqlc.arg(project_id)
  AND app_id = sqlc.arg(app_id)
  AND applied_fingerprint = fingerprint
LIMIT 1;
