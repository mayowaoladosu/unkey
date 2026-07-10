-- name: DeleteAppById :exec
DELETE app_framework_detections, apps
FROM apps
LEFT JOIN app_framework_detections ON app_framework_detections.app_id = apps.id
WHERE apps.id = sqlc.arg(id);
