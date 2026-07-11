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
