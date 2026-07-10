import { relations } from "drizzle-orm";
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

export type DeploymentManifestDocument = {
  version: number;
  source: unknown;
  build: unknown;
  outputs: unknown[];
  runtime: unknown;
  routes: unknown[];
  provenance: unknown;
};

/** Immutable deployment intent compiled before any build is materialized. */
export const deploymentManifests = mysqlTable(
  "deployment_manifests",
  {
    pk: bigint("pk", { mode: "number", unsigned: true }).autoincrement().primaryKey(),
    deploymentId: varchar("deployment_id", { length: 128 }).notNull(),
    workspaceId: varchar("workspace_id", { length: 256 }).notNull(),
    projectId: varchar("project_id", { length: 256 }).notNull(),
    appId: varchar("app_id", { length: 64 }).notNull(),
    environmentId: varchar("environment_id", { length: 128 }).notNull(),
    schemaVersion: bigint("schema_version", { mode: "number", unsigned: true }).notNull(),
    fingerprint: varchar("fingerprint", { length: 64 }).notNull(),
    adapterId: varchar("adapter_id", { length: 64 }).notNull(),
    outputMode: mysqlEnum("output_mode", ["container", "static", "mixed"]).notNull(),
    manifest: json("manifest").$type<DeploymentManifestDocument>().notNull(),
    createdAt: bigint("created_at", { mode: "number" }).notNull(),
  },
  (table) => [
    uniqueIndex("deployment_manifests_deployment_idx").on(table.deploymentId),
    index("deployment_manifests_workspace_idx").on(table.workspaceId),
    index("deployment_manifests_project_idx").on(table.projectId),
    index("deployment_manifests_app_idx").on(table.appId),
    index("deployment_manifests_environment_idx").on(table.environmentId),
  ],
);

export const deploymentManifestsRelations = relations(deploymentManifests, ({ one }) => ({
  deployment: one(deployments, {
    fields: [deploymentManifests.deploymentId],
    references: [deployments.id],
  }),
  workspace: one(workspaces, {
    fields: [deploymentManifests.workspaceId],
    references: [workspaces.id],
  }),
  project: one(projects, {
    fields: [deploymentManifests.projectId],
    references: [projects.id],
  }),
  app: one(apps, {
    fields: [deploymentManifests.appId],
    references: [apps.id],
  }),
  environment: one(environments, {
    fields: [deploymentManifests.environmentId],
    references: [environments.id],
  }),
}));
