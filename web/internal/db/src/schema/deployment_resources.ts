import { relations, sql } from "drizzle-orm";
import {
  bigint,
  boolean,
  index,
  int,
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

export const deploymentResources = mysqlTable(
  "deployment_resources",
  {
    pk: bigint("pk", { mode: "number", unsigned: true }).autoincrement().primaryKey(),
    id: varchar("id", { length: 128 }).notNull().unique(),
    deploymentId: varchar("deployment_id", { length: 128 }).notNull(),
    workspaceId: varchar("workspace_id", { length: 256 }).notNull(),
    projectId: varchar("project_id", { length: 256 }).notNull(),
    appId: varchar("app_id", { length: 64 }).notNull(),
    environmentId: varchar("environment_id", { length: 128 }).notNull(),
    name: varchar("name", { length: 128 }).notNull(),
    kind: mysqlEnum("kind", ["service", "function", "worker", "cron", "static"]).notNull(),
    k8sName: varchar("k8s_name", { length: 63 }).unique(),
    image: varchar("image", { length: 256 }),
    command: json("command").$type<string[]>().notNull().default(sql`(JSON_ARRAY())`),
    port: int("port").notNull().default(0),
    upstreamProtocol: mysqlEnum("upstream_protocol", ["http1", "h2c"]).notNull().default("http1"),
    public: boolean("public").notNull().default(false),
    schedule: varchar("schedule", { length: 128 }),
    runtime: varchar("runtime", { length: 64 }),
    handler: varchar("handler", { length: 512 }),
    bindings: json("bindings")
      .$type<
        Array<{
          name: string;
          resourceId: string;
          resourceName: string;
          protocol: "http" | "tcp";
          host: string;
          port: number;
        }>
      >()
      .notNull()
      .default(sql`(JSON_ARRAY())`),
    allowedCallers: json("allowed_callers").$type<string[]>().notNull().default(sql`(JSON_ARRAY())`),
    cpuMillicores: int("cpu_millicores").notNull(),
    memoryMib: int("memory_mib").notNull(),
    storageMib: int("storage_mib", { unsigned: true }).notNull().default(0),
    createdAt: bigint("created_at", { mode: "number" }).notNull(),
  },
  (table) => [
    uniqueIndex("deployment_resources_deployment_name_unique").on(table.deploymentId, table.name),
    index("deployment_resources_workspace_idx").on(table.workspaceId),
    index("deployment_resources_project_idx").on(table.projectId),
    index("deployment_resources_app_idx").on(table.appId),
    index("deployment_resources_environment_idx").on(table.environmentId),
    index("deployment_resources_deployment_idx").on(table.deploymentId),
  ],
);

export const deploymentResourcesRelations = relations(deploymentResources, ({ one }) => ({
  deployment: one(deployments, {
    fields: [deploymentResources.deploymentId],
    references: [deployments.id],
  }),
  workspace: one(workspaces, {
    fields: [deploymentResources.workspaceId],
    references: [workspaces.id],
  }),
  project: one(projects, {
    fields: [deploymentResources.projectId],
    references: [projects.id],
  }),
  app: one(apps, {
    fields: [deploymentResources.appId],
    references: [apps.id],
  }),
  environment: one(environments, {
    fields: [deploymentResources.environmentId],
    references: [environments.id],
  }),
}));
