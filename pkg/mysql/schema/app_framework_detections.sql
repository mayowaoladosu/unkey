CREATE TABLE `app_framework_detections` (
	`pk` bigint unsigned AUTO_INCREMENT NOT NULL,
	`workspace_id` varchar(256) NOT NULL,
	`project_id` varchar(64) NOT NULL,
	`app_id` varchar(64) NOT NULL,
	`repository_full_name` varchar(500) NOT NULL,
	`branch` varchar(256) NOT NULL,
	`tree_sha` varchar(64) NOT NULL,
	`fingerprint` varchar(64) NOT NULL,
	`detection_version` bigint unsigned NOT NULL DEFAULT 1,
	`detected_preset_id` varchar(128),
	`detected_preset_name` varchar(256),
	`confidence` enum('none','low','medium','high') NOT NULL DEFAULT 'none',
	`build_strategy` enum('dockerfile','zero-config','unknown') NOT NULL DEFAULT 'unknown',
	`detection` json NOT NULL,
	`defaults` json NOT NULL,
	`detected_at` bigint NOT NULL,
	`applied_fingerprint` varchar(64),
	`applied_defaults` json,
	`applied_at` bigint,
	`created_at` bigint NOT NULL,
	`updated_at` bigint,
	CONSTRAINT `app_framework_detections_pk` PRIMARY KEY(`pk`),
	CONSTRAINT `app_framework_detections_app_idx` UNIQUE(`app_id`)
);

CREATE INDEX `app_framework_detections_workspace_idx` ON `app_framework_detections` (`workspace_id`);
CREATE INDEX `app_framework_detections_project_idx` ON `app_framework_detections` (`project_id`);
