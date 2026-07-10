CREATE TABLE `deployment_manifests` (
	`pk` bigint unsigned AUTO_INCREMENT NOT NULL,
	`deployment_id` varchar(128) NOT NULL,
	`workspace_id` varchar(256) NOT NULL,
	`project_id` varchar(256) NOT NULL,
	`app_id` varchar(64) NOT NULL,
	`environment_id` varchar(128) NOT NULL,
	`schema_version` bigint unsigned NOT NULL,
	`fingerprint` varchar(64) NOT NULL,
	`adapter_id` varchar(64) NOT NULL,
	`output_mode` enum('container','static','mixed') NOT NULL,
	`manifest` json NOT NULL,
	`created_at` bigint NOT NULL,
	CONSTRAINT `deployment_manifests_pk` PRIMARY KEY(`pk`),
	CONSTRAINT `deployment_manifests_deployment_idx` UNIQUE(`deployment_id`)
);

CREATE INDEX `deployment_manifests_workspace_idx` ON `deployment_manifests` (`workspace_id`);
CREATE INDEX `deployment_manifests_project_idx` ON `deployment_manifests` (`project_id`);
CREATE INDEX `deployment_manifests_app_idx` ON `deployment_manifests` (`app_id`);
CREATE INDEX `deployment_manifests_environment_idx` ON `deployment_manifests` (`environment_id`);
