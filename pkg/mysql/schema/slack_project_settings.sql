CREATE TABLE `slack_project_settings` (
	`pk` bigint unsigned AUTO_INCREMENT NOT NULL,
	`id` varchar(128) NOT NULL,
	`workspace_id` varchar(256) NOT NULL,
	`project_id` varchar(64) NOT NULL,
	`approval_policy` enum('anyone','admins_only') NOT NULL DEFAULT 'anyone',
	`created_at` bigint NOT NULL,
	`updated_at` bigint,
	CONSTRAINT `slack_project_settings_pk` PRIMARY KEY(`pk`),
	CONSTRAINT `slack_project_settings_id_unique` UNIQUE(`id`),
	CONSTRAINT `slack_project_settings_project_idx` UNIQUE(`project_id`)
);

