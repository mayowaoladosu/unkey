CREATE TABLE `slack_project_connections` (
	`pk` bigint unsigned AUTO_INCREMENT NOT NULL,
	`id` varchar(128) NOT NULL,
	`workspace_id` varchar(256) NOT NULL,
	`project_id` varchar(64) NOT NULL,
	`installation_id` varchar(128) NOT NULL,
	`channel_id` varchar(64) NOT NULL,
	`channel_name` varchar(256) NOT NULL,
	`include_previews` boolean NOT NULL DEFAULT false,
	`approval_policy` enum('anyone','admins_only') NOT NULL DEFAULT 'anyone',
	`created_at` bigint NOT NULL,
	`updated_at` bigint,
	CONSTRAINT `slack_project_connections_pk` PRIMARY KEY(`pk`),
	CONSTRAINT `slack_project_connections_id_unique` UNIQUE(`id`),
	CONSTRAINT `slack_project_connections_project_id_unique` UNIQUE(`project_id`)
);

CREATE INDEX `slack_installation_id_idx` ON `slack_project_connections` (`installation_id`);

