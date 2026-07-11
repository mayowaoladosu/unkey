CREATE TABLE `deployment_resources` (
	`pk` bigint unsigned AUTO_INCREMENT NOT NULL,
	`id` varchar(128) NOT NULL,
	`deployment_id` varchar(128) NOT NULL,
	`workspace_id` varchar(256) NOT NULL,
	`project_id` varchar(256) NOT NULL,
	`app_id` varchar(64) NOT NULL,
	`environment_id` varchar(128) NOT NULL,
	`name` varchar(128) NOT NULL,
	`kind` enum('service','function','worker','cron','static') NOT NULL,
	`k8s_name` varchar(63),
	`image` varchar(256),
	`command` json NOT NULL DEFAULT (JSON_ARRAY()),
	`port` int NOT NULL DEFAULT 0,
	`upstream_protocol` enum('http1','h2c') NOT NULL DEFAULT 'http1',
	`public` boolean NOT NULL DEFAULT false,
	`schedule` varchar(128),
	`runtime` varchar(64),
	`handler` varchar(512),
	`cpu_millicores` int NOT NULL,
	`memory_mib` int NOT NULL,
	`storage_mib` int unsigned NOT NULL DEFAULT 0,
	`created_at` bigint NOT NULL,
	CONSTRAINT `deployment_resources_pk` PRIMARY KEY(`pk`),
	CONSTRAINT `deployment_resources_id_unique` UNIQUE(`id`),
	CONSTRAINT `deployment_resources_deployment_name_unique` UNIQUE(`deployment_id`,`name`),
	CONSTRAINT `deployment_resources_k8s_name_unique` UNIQUE(`k8s_name`)
);

CREATE INDEX `deployment_resources_workspace_idx` ON `deployment_resources` (`workspace_id`);
CREATE INDEX `deployment_resources_project_idx` ON `deployment_resources` (`project_id`);
CREATE INDEX `deployment_resources_app_idx` ON `deployment_resources` (`app_id`);
CREATE INDEX `deployment_resources_environment_idx` ON `deployment_resources` (`environment_id`);
CREATE INDEX `deployment_resources_deployment_idx` ON `deployment_resources` (`deployment_id`);
