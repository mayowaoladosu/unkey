CREATE TABLE `deployment_artifacts` (
	`pk` bigint unsigned AUTO_INCREMENT NOT NULL,
	`id` varchar(128) NOT NULL,
	`deployment_id` varchar(128) NOT NULL,
	`workspace_id` varchar(256) NOT NULL,
	`project_id` varchar(256) NOT NULL,
	`app_id` varchar(64) NOT NULL,
	`environment_id` varchar(128) NOT NULL,
	`name` varchar(128) NOT NULL,
	`kind` enum('container_image','static_bundle','function_bundle','source_map') NOT NULL,
	`storage_key` varchar(1024) NOT NULL,
	`digest` varchar(64) NOT NULL,
	`size_bytes` bigint unsigned NOT NULL,
	`content_type` varchar(256) NOT NULL,
	`metadata` json NOT NULL DEFAULT ('{}'),
	`created_at` bigint NOT NULL,
	CONSTRAINT `deployment_artifacts_pk` PRIMARY KEY(`pk`),
	CONSTRAINT `deployment_artifacts_id_unique` UNIQUE(`id`),
	CONSTRAINT `deployment_artifacts_deployment_kind_name_idx` UNIQUE(`deployment_id`,`kind`,`name`)
);

CREATE INDEX `deployment_artifacts_workspace_idx` ON `deployment_artifacts` (`workspace_id`);
CREATE INDEX `deployment_artifacts_project_idx` ON `deployment_artifacts` (`project_id`);
CREATE INDEX `deployment_artifacts_app_idx` ON `deployment_artifacts` (`app_id`);
CREATE INDEX `deployment_artifacts_environment_idx` ON `deployment_artifacts` (`environment_id`);
