import { relations, sql } from "drizzle-orm";
import {
  bigint,
  index,
  json,
  mysqlEnum,
  mysqlTable,
  uniqueIndex,
  varchar,
} from "drizzle-orm/mysql-core";
import { apps } from "./apps";
import { deployments } from "./deployments";
import { environments } from "./environments";
import { projects } from "./projects";
import { workspaces } from "./workspaces";

export type DeploymentArtifactMetadata = {
  spaFallback?: boolean;
  fileCount?: number;
  uncompressedBytes?: number;
};

export const deploymentArtifacts = mysqlTable(
  "deployment_artifacts",
  {
    pk: bigint("pk", { mode: "number", unsigned: true }).autoincrement().primaryKey(),
    id: varchar("id", { length: 128 }).notNull(),
    deploymentId: varchar("deployment_id", { length: 128 }).notNull(),
    workspaceId: varchar("workspace_id", { length: 256 }).notNull(),
    projectId: varchar("project_id", { length: 256 }).notNull(),
    appId: varchar("app_id", { length: 64 }).notNull(),
    environmentId: varchar("environment_id", { length: 128 }).notNull(),
    name: varchar("name", { length: 128 }).notNull(),
    kind: mysqlEnum("kind", [
      "container_image",
      "static_bundle",
      "function_bundle",
      "source_map",
    ]).notNull(),
    storageKey: varchar("storage_key", { length: 1024 }).notNull(),
    digest: varchar("digest", { length: 64 }).notNull(),
    sizeBytes: bigint("size_bytes", { mode: "number", unsigned: true }).notNull(),
    contentType: varchar("content_type", { length: 256 }).notNull(),
    metadata: json("metadata").$type<DeploymentArtifactMetadata>().notNull().default(sql`('{}')`),
    createdAt: bigint("created_at", { mode: "number" }).notNull(),
  },
  (table) => [
    uniqueIndex("deployment_artifacts_id_unique").on(table.id),
    uniqueIndex("deployment_artifacts_deployment_kind_name_idx").on(
      table.deploymentId,
      table.kind,
      table.name,
    ),
    index("deployment_artifacts_workspace_idx").on(table.workspaceId),
    index("deployment_artifacts_project_idx").on(table.projectId),
    index("deployment_artifacts_app_idx").on(table.appId),
    index("deployment_artifacts_environment_idx").on(table.environmentId),
  ],
);

export const deploymentArtifactsRelations = relations(deploymentArtifacts, ({ one }) => ({
  deployment: one(deployments, {
    fields: [deploymentArtifacts.deploymentId],
    references: [deployments.id],
  }),
  workspace: one(workspaces, {
    fields: [deploymentArtifacts.workspaceId],
    references: [workspaces.id],
  }),
  project: one(projects, {
    fields: [deploymentArtifacts.projectId],
    references: [projects.id],
  }),
  app: one(apps, {
    fields: [deploymentArtifacts.appId],
    references: [apps.id],
  }),
  environment: one(environments, {
    fields: [deploymentArtifacts.environmentId],
    references: [environments.id],
  }),
}));
