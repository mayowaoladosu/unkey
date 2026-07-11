-- Resource dimensions for independently materialized deployment outputs.
-- Defaults keep historical rows and rolling upgrades queryable.

ALTER TABLE default.runtime_logs_raw_v1
  ADD COLUMN IF NOT EXISTS resource_id String DEFAULT '' AFTER deployment_id;
ALTER TABLE default.runtime_logs_raw_v1
  ADD COLUMN IF NOT EXISTS resource_name String DEFAULT '' AFTER resource_id;
ALTER TABLE default.runtime_logs_raw_v1
  ADD COLUMN IF NOT EXISTS resource_kind LowCardinality(String) DEFAULT '' AFTER resource_name;
ALTER TABLE default.runtime_logs_raw_v1
  ADD INDEX IF NOT EXISTS idx_resource_id resource_id TYPE bloom_filter(0.001) GRANULARITY 1;

ALTER TABLE default.instance_events_raw_v1
  ADD COLUMN IF NOT EXISTS resource_id String DEFAULT '' AFTER deployment_id;
ALTER TABLE default.instance_events_raw_v1
  ADD COLUMN IF NOT EXISTS resource_name String DEFAULT '' AFTER resource_id;
ALTER TABLE default.instance_events_raw_v1
  ADD COLUMN IF NOT EXISTS resource_kind LowCardinality(String) DEFAULT '' AFTER resource_name;
ALTER TABLE default.instance_events_raw_v1
  ADD INDEX IF NOT EXISTS idx_resource_id resource_id TYPE bloom_filter(0.001) GRANULARITY 1;

ALTER TABLE default.frontline_requests_raw_v1
  ADD COLUMN IF NOT EXISTS resource_id String DEFAULT '' AFTER deployment_id;
ALTER TABLE default.frontline_requests_raw_v1
  ADD COLUMN IF NOT EXISTS resource_name String DEFAULT '' AFTER resource_id;
ALTER TABLE default.frontline_requests_raw_v1
  ADD COLUMN IF NOT EXISTS resource_kind LowCardinality(String) DEFAULT '' AFTER resource_name;
ALTER TABLE default.frontline_requests_raw_v1
  ADD INDEX IF NOT EXISTS idx_resource_id resource_id TYPE bloom_filter(0.001) GRANULARITY 1;

-- Resource dimensions for deployment compute checkpoints and dashboard rollups.
-- resource_id remains the parent deployment for billing compatibility;
-- deployment_resource_* identify independently materialized outputs.

ALTER TABLE default.instance_checkpoints_v1
  ADD COLUMN IF NOT EXISTS deployment_resource_id String DEFAULT '' AFTER resource_id;
ALTER TABLE default.instance_checkpoints_v1
  ADD COLUMN IF NOT EXISTS deployment_resource_name String DEFAULT '' AFTER deployment_resource_id;
ALTER TABLE default.instance_checkpoints_v1
  ADD COLUMN IF NOT EXISTS deployment_resource_kind LowCardinality(String) DEFAULT '' AFTER deployment_resource_name;
ALTER TABLE default.instance_checkpoints_v1
  ADD INDEX IF NOT EXISTS idx_deployment_resource deployment_resource_id TYPE bloom_filter(0.001) GRANULARITY 1;

ALTER TABLE default.instance_resources_per_15s_v1
  ADD COLUMN IF NOT EXISTS deployment_resource_id String DEFAULT '' AFTER resource_id;
ALTER TABLE default.instance_resources_per_15s_v1
  ADD COLUMN IF NOT EXISTS deployment_resource_name LowCardinality(String) DEFAULT '' AFTER deployment_resource_id;
ALTER TABLE default.instance_resources_per_15s_v1
  ADD COLUMN IF NOT EXISTS deployment_resource_kind LowCardinality(String) DEFAULT '' AFTER deployment_resource_name;

ALTER TABLE default.instance_resources_per_minute_v1
  ADD COLUMN IF NOT EXISTS deployment_resource_id String DEFAULT '' AFTER resource_id;
ALTER TABLE default.instance_resources_per_minute_v1
  ADD COLUMN IF NOT EXISTS deployment_resource_name LowCardinality(String) DEFAULT '' AFTER deployment_resource_id;
ALTER TABLE default.instance_resources_per_minute_v1
  ADD COLUMN IF NOT EXISTS deployment_resource_kind LowCardinality(String) DEFAULT '' AFTER deployment_resource_name;

ALTER TABLE default.instance_resources_per_hour_v1
  ADD COLUMN IF NOT EXISTS deployment_resource_id String DEFAULT '' AFTER resource_id;
ALTER TABLE default.instance_resources_per_hour_v1
  ADD COLUMN IF NOT EXISTS deployment_resource_name LowCardinality(String) DEFAULT '' AFTER deployment_resource_id;
ALTER TABLE default.instance_resources_per_hour_v1
  ADD COLUMN IF NOT EXISTS deployment_resource_kind LowCardinality(String) DEFAULT '' AFTER deployment_resource_name;

ALTER TABLE default.instance_resources_per_day_v1
  ADD COLUMN IF NOT EXISTS deployment_resource_id String DEFAULT '' AFTER resource_id;
ALTER TABLE default.instance_resources_per_day_v1
  ADD COLUMN IF NOT EXISTS deployment_resource_name LowCardinality(String) DEFAULT '' AFTER deployment_resource_id;
ALTER TABLE default.instance_resources_per_day_v1
  ADD COLUMN IF NOT EXISTS deployment_resource_kind LowCardinality(String) DEFAULT '' AFTER deployment_resource_name;

ALTER TABLE default.instance_resources_per_month_v1
  ADD COLUMN IF NOT EXISTS deployment_resource_id String DEFAULT '' AFTER resource_id;
ALTER TABLE default.instance_resources_per_month_v1
  ADD COLUMN IF NOT EXISTS deployment_resource_name LowCardinality(String) DEFAULT '' AFTER deployment_resource_id;
ALTER TABLE default.instance_resources_per_month_v1
  ADD COLUMN IF NOT EXISTS deployment_resource_kind LowCardinality(String) DEFAULT '' AFTER deployment_resource_name;

DROP VIEW IF EXISTS default.instance_resources_per_15s_mv_v1;
CREATE MATERIALIZED VIEW default.instance_resources_per_15s_mv_v1
TO default.instance_resources_per_15s_v1 AS
SELECT
  toStartOfInterval(fromUnixTimestamp64Milli(ts), INTERVAL 15 SECOND) AS time,
  workspace_id, project_id, environment_id, resource_type, resource_id,
  deployment_resource_id, deployment_resource_name, deployment_resource_kind,
  container_uid, instance_id,
  min(cpu_usage_usec) AS cpu_usage_usec_min,
  max(cpu_usage_usec) AS cpu_usage_usec_max,
  sum(memory_bytes) AS memory_bytes_sum,
  max(memory_bytes) AS memory_bytes_max,
  max(cpu_allocated_millicores) AS cpu_allocated_millicores_max,
  max(memory_allocated_bytes) AS memory_allocated_bytes_max,
  max(disk_allocated_bytes) AS disk_allocated_bytes_max,
  max(disk_used_bytes) AS disk_used_bytes_max,
  min(network_egress_public_bytes) AS network_egress_public_bytes_min,
  max(network_egress_public_bytes) AS network_egress_public_bytes_max,
  min(network_egress_private_bytes) AS network_egress_private_bytes_min,
  max(network_egress_private_bytes) AS network_egress_private_bytes_max,
  min(network_ingress_public_bytes) AS network_ingress_public_bytes_min,
  max(network_ingress_public_bytes) AS network_ingress_public_bytes_max,
  min(network_ingress_private_bytes) AS network_ingress_private_bytes_min,
  max(network_ingress_private_bytes) AS network_ingress_private_bytes_max,
  toInt64(count()) AS sample_count
FROM default.instance_checkpoints_v1
GROUP BY time, workspace_id, project_id, environment_id, resource_type, resource_id,
  deployment_resource_id, deployment_resource_name, deployment_resource_kind, container_uid, instance_id;

DROP VIEW IF EXISTS default.instance_resources_per_minute_mv_v1;
CREATE MATERIALIZED VIEW default.instance_resources_per_minute_mv_v1
TO default.instance_resources_per_minute_v1 AS
SELECT
  toStartOfMinute(fromUnixTimestamp64Milli(ts)) AS time,
  workspace_id, project_id, environment_id, resource_type, resource_id,
  deployment_resource_id, deployment_resource_name, deployment_resource_kind,
  container_uid, instance_id,
  min(cpu_usage_usec) AS cpu_usage_usec_min,
  max(cpu_usage_usec) AS cpu_usage_usec_max,
  sum(memory_bytes) AS memory_bytes_sum,
  max(memory_bytes) AS memory_bytes_max,
  max(cpu_allocated_millicores) AS cpu_allocated_millicores_max,
  max(memory_allocated_bytes) AS memory_allocated_bytes_max,
  max(disk_allocated_bytes) AS disk_allocated_bytes_max,
  max(disk_used_bytes) AS disk_used_bytes_max,
  min(network_egress_public_bytes) AS network_egress_public_bytes_min,
  max(network_egress_public_bytes) AS network_egress_public_bytes_max,
  min(network_egress_private_bytes) AS network_egress_private_bytes_min,
  max(network_egress_private_bytes) AS network_egress_private_bytes_max,
  min(network_ingress_public_bytes) AS network_ingress_public_bytes_min,
  max(network_ingress_public_bytes) AS network_ingress_public_bytes_max,
  min(network_ingress_private_bytes) AS network_ingress_private_bytes_min,
  max(network_ingress_private_bytes) AS network_ingress_private_bytes_max,
  toInt64(count()) AS sample_count
FROM default.instance_checkpoints_v1
GROUP BY time, workspace_id, project_id, environment_id, resource_type, resource_id,
  deployment_resource_id, deployment_resource_name, deployment_resource_kind, container_uid, instance_id;

DROP VIEW IF EXISTS default.instance_resources_per_hour_mv_v1;
CREATE MATERIALIZED VIEW default.instance_resources_per_hour_mv_v1
TO default.instance_resources_per_hour_v1 AS
SELECT
  toStartOfHour(fromUnixTimestamp64Milli(ts)) AS time,
  workspace_id, project_id, environment_id, resource_type, resource_id,
  deployment_resource_id, deployment_resource_name, deployment_resource_kind,
  container_uid, instance_id,
  min(cpu_usage_usec) AS cpu_usage_usec_min,
  max(cpu_usage_usec) AS cpu_usage_usec_max,
  max(memory_bytes) AS memory_bytes_max,
  max(cpu_allocated_millicores) AS cpu_allocated_millicores_max,
  max(memory_allocated_bytes) AS memory_allocated_bytes_max,
  max(disk_allocated_bytes) AS disk_allocated_bytes_max,
  max(disk_used_bytes) AS disk_used_bytes_max,
  min(network_egress_public_bytes) AS network_egress_public_bytes_min,
  max(network_egress_public_bytes) AS network_egress_public_bytes_max,
  min(network_egress_private_bytes) AS network_egress_private_bytes_min,
  max(network_egress_private_bytes) AS network_egress_private_bytes_max,
  min(network_ingress_public_bytes) AS network_ingress_public_bytes_min,
  max(network_ingress_public_bytes) AS network_ingress_public_bytes_max,
  min(network_ingress_private_bytes) AS network_ingress_private_bytes_min,
  max(network_ingress_private_bytes) AS network_ingress_private_bytes_max,
  toInt64(count()) AS sample_count
FROM default.instance_checkpoints_v1
GROUP BY time, workspace_id, project_id, environment_id, resource_type, resource_id,
  deployment_resource_id, deployment_resource_name, deployment_resource_kind, container_uid, instance_id;

DROP VIEW IF EXISTS default.instance_resources_per_day_mv_v1;
CREATE MATERIALIZED VIEW default.instance_resources_per_day_mv_v1
TO default.instance_resources_per_day_v1 AS
SELECT
  toStartOfDay(fromUnixTimestamp64Milli(ts)) AS time,
  workspace_id, project_id, environment_id, resource_type, resource_id,
  deployment_resource_id, deployment_resource_name, deployment_resource_kind,
  container_uid, instance_id,
  min(cpu_usage_usec) AS cpu_usage_usec_min,
  max(cpu_usage_usec) AS cpu_usage_usec_max,
  max(memory_bytes) AS memory_bytes_max,
  max(cpu_allocated_millicores) AS cpu_allocated_millicores_max,
  max(memory_allocated_bytes) AS memory_allocated_bytes_max,
  max(disk_allocated_bytes) AS disk_allocated_bytes_max,
  max(disk_used_bytes) AS disk_used_bytes_max,
  min(network_egress_public_bytes) AS network_egress_public_bytes_min,
  max(network_egress_public_bytes) AS network_egress_public_bytes_max,
  min(network_egress_private_bytes) AS network_egress_private_bytes_min,
  max(network_egress_private_bytes) AS network_egress_private_bytes_max,
  min(network_ingress_public_bytes) AS network_ingress_public_bytes_min,
  max(network_ingress_public_bytes) AS network_ingress_public_bytes_max,
  min(network_ingress_private_bytes) AS network_ingress_private_bytes_min,
  max(network_ingress_private_bytes) AS network_ingress_private_bytes_max,
  toInt64(count()) AS sample_count
FROM default.instance_checkpoints_v1
GROUP BY time, workspace_id, project_id, environment_id, resource_type, resource_id,
  deployment_resource_id, deployment_resource_name, deployment_resource_kind, container_uid, instance_id;

DROP VIEW IF EXISTS default.instance_resources_per_month_mv_v1;
CREATE MATERIALIZED VIEW default.instance_resources_per_month_mv_v1
TO default.instance_resources_per_month_v1 AS
SELECT
  toStartOfMonth(fromUnixTimestamp64Milli(ts)) AS time,
  workspace_id, project_id, environment_id, resource_type, resource_id,
  deployment_resource_id, deployment_resource_name, deployment_resource_kind,
  container_uid, instance_id,
  min(cpu_usage_usec) AS cpu_usage_usec_min,
  max(cpu_usage_usec) AS cpu_usage_usec_max,
  max(memory_bytes) AS memory_bytes_max,
  max(cpu_allocated_millicores) AS cpu_allocated_millicores_max,
  max(memory_allocated_bytes) AS memory_allocated_bytes_max,
  max(disk_allocated_bytes) AS disk_allocated_bytes_max,
  max(disk_used_bytes) AS disk_used_bytes_max,
  min(network_egress_public_bytes) AS network_egress_public_bytes_min,
  max(network_egress_public_bytes) AS network_egress_public_bytes_max,
  min(network_egress_private_bytes) AS network_egress_private_bytes_min,
  max(network_egress_private_bytes) AS network_egress_private_bytes_max,
  min(network_ingress_public_bytes) AS network_ingress_public_bytes_min,
  max(network_ingress_public_bytes) AS network_ingress_public_bytes_max,
  min(network_ingress_private_bytes) AS network_ingress_private_bytes_min,
  max(network_ingress_private_bytes) AS network_ingress_private_bytes_max,
  toInt64(count()) AS sample_count
FROM default.instance_checkpoints_v1
GROUP BY time, workspace_id, project_id, environment_id, resource_type, resource_id,
  deployment_resource_id, deployment_resource_name, deployment_resource_kind, container_uid, instance_id;
