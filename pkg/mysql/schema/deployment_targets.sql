CREATE TABLE `deployment_targets` (
	`pk` bigint unsigned AUTO_INCREMENT NOT NULL,
	`id` varchar(128) NOT NULL,
	`workspace_id` varchar(256) NOT NULL,
	`project_id` varchar(256) NOT NULL,
	`app_id` varchar(64) NOT NULL,
	`environment_id` varchar(128) NOT NULL,
	`kind` enum('branch','environment','live') NOT NULL,
	`target_key` varchar(256) NOT NULL,
	`deployment_id` varchar(128) NOT NULL,
	`previous_deployment_id` varchar(128),
	`created_at` bigint NOT NULL,
	`updated_at` bigint,
	CONSTRAINT `deployment_targets_pk` PRIMARY KEY(`pk`),
	CONSTRAINT `deployment_targets_id_unique` UNIQUE(`id`),
	CONSTRAINT `deployment_targets_identity_unique` UNIQUE(`app_id`,`environment_id`,`kind`,`target_key`)
);

CREATE INDEX `deployment_targets_workspace_idx` ON `deployment_targets` (`workspace_id`);
CREATE INDEX `deployment_targets_project_idx` ON `deployment_targets` (`project_id`);
CREATE INDEX `deployment_targets_environment_idx` ON `deployment_targets` (`environment_id`);
CREATE INDEX `deployment_targets_deployment_idx` ON `deployment_targets` (`deployment_id`);

CREATE TABLE `deployment_target_assignments` (
	`pk` bigint unsigned AUTO_INCREMENT NOT NULL,
	`id` varchar(128) NOT NULL,
	`target_id` varchar(128) NOT NULL,
	`workspace_id` varchar(256) NOT NULL,
	`project_id` varchar(256) NOT NULL,
	`app_id` varchar(64) NOT NULL,
	`environment_id` varchar(128) NOT NULL,
	`deployment_id` varchar(128) NOT NULL,
	`previous_deployment_id` varchar(128),
	`reason` enum('deploy','promote','rollback','restore') NOT NULL,
	`operation_id` varchar(256) NOT NULL,
	`created_at` bigint NOT NULL,
	CONSTRAINT `deployment_target_assignments_pk` PRIMARY KEY(`pk`),
	CONSTRAINT `deployment_target_assignments_id_unique` UNIQUE(`id`),
	CONSTRAINT `deployment_target_assignments_operation_unique` UNIQUE(`target_id`,`operation_id`)
);

CREATE INDEX `deployment_target_assignments_target_idx` ON `deployment_target_assignments` (`target_id`);
CREATE INDEX `deployment_target_assignments_environment_idx` ON `deployment_target_assignments` (`environment_id`);
CREATE INDEX `deployment_target_assignments_deployment_idx` ON `deployment_target_assignments` (`deployment_id`);
