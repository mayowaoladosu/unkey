-- name: InsertHorizontalAutoscalingPolicy :exec
INSERT INTO horizontal_autoscaling_policies (
    id,
    workspace_id,
    replicas_min,
    replicas_max,
    memory_threshold,
    cpu_threshold,
    rps_threshold,
    created_at,
    updated_at
) VALUES (
    sqlc.arg(id),
    sqlc.arg(workspace_id),
    sqlc.arg(replicas_min),
    sqlc.arg(replicas_max),
    sqlc.arg(memory_threshold),
    sqlc.arg(cpu_threshold),
    sqlc.arg(rps_threshold),
    sqlc.arg(created_at),
    sqlc.arg(updated_at)
);