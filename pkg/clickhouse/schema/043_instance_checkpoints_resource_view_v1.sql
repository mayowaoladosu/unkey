-- ClickHouse expands SELECT * when a view is created, so recreate the
-- convenience view after adding deployment_resource_* columns to its source.
DROP VIEW IF EXISTS default.instance_checkpoints;
CREATE VIEW default.instance_checkpoints AS
SELECT *
FROM default.instance_checkpoints_v1
FINAL;
