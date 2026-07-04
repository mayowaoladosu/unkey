CREATE TABLE `slack_installations` (
	`pk` bigint unsigned AUTO_INCREMENT NOT NULL,
	`id` varchar(128) NOT NULL,
	`workspace_id` varchar(256) NOT NULL,
	`team_id` varchar(64) NOT NULL,
	`bot_token` varchar(4096) NOT NULL,
	`bot_user_id` varchar(64) NOT NULL,
	`installed_by_user_id` varchar(256) NOT NULL,
	`created_at` bigint NOT NULL,
	`updated_at` bigint,
	CONSTRAINT `slack_installations_pk` PRIMARY KEY(`pk`),
	CONSTRAINT `slack_installations_id_unique` UNIQUE(`id`),
	CONSTRAINT `workspace_team_idx` UNIQUE(`workspace_id`,`team_id`)
);

