import { relations } from "drizzle-orm";
import { bigint, index, mysqlEnum, mysqlTable, uniqueIndex, varchar } from "drizzle-orm/mysql-core";
import { apps } from "./apps";
import { deployments } from "./deployments";
import { environments } from "./environments";
import { projects } from "./projects";
import { lifecycleDates } from "./util/lifecycle_dates";
import { workspaces } from "./workspaces";

export const deploymentTargetKind = ["branch", "environment", "live"] as const;
export const deploymentTargetAssignmentReason = [
  "deploy",
  "promote",
  "rollback",
  "restore",
] as const;

export const deploymentTargets = mysqlTable(
  "deployment_targets",
  {
    pk: bigint("pk", { mode: "number", unsigned: true }).autoincrement().primaryKey(),
    id: varchar("id", { length: 128 }).notNull().unique(),
    workspaceId: varchar("workspace_id", { length: 256 }).notNull(),
    projectId: varchar("project_id", { length: 256 }).notNull(),
    appId: varchar("app_id", { length: 64 }).notNull(),
    environmentId: varchar("environment_id", { length: 128 }).notNull(),
    kind: mysqlEnum("kind", deploymentTargetKind).notNull(),
    targetKey: varchar("target_key", { length: 256 }).notNull(),
    deploymentId: varchar("deployment_id", { length: 128 }).notNull(),
    previousDeploymentId: varchar("previous_deployment_id", { length: 128 }),
    ...lifecycleDates,
  },
  (table) => [
    uniqueIndex("deployment_targets_identity_unique").on(
      table.appId,
      table.environmentId,
      table.kind,
      table.targetKey,
    ),
    index("deployment_targets_workspace_idx").on(table.workspaceId),
    index("deployment_targets_project_idx").on(table.projectId),
    index("deployment_targets_environment_idx").on(table.environmentId),
    index("deployment_targets_deployment_idx").on(table.deploymentId),
  ],
);

export const deploymentTargetAssignments = mysqlTable(
  "deployment_target_assignments",
  {
    pk: bigint("pk", { mode: "number", unsigned: true }).autoincrement().primaryKey(),
    id: varchar("id", { length: 128 }).notNull().unique(),
    targetId: varchar("target_id", { length: 128 }).notNull(),
    workspaceId: varchar("workspace_id", { length: 256 }).notNull(),
    projectId: varchar("project_id", { length: 256 }).notNull(),
    appId: varchar("app_id", { length: 64 }).notNull(),
    environmentId: varchar("environment_id", { length: 128 }).notNull(),
    deploymentId: varchar("deployment_id", { length: 128 }).notNull(),
    previousDeploymentId: varchar("previous_deployment_id", { length: 128 }),
    reason: mysqlEnum("reason", deploymentTargetAssignmentReason).notNull(),
    operationId: varchar("operation_id", { length: 256 }).notNull(),
    createdAt: bigint("created_at", { mode: "number" }).notNull(),
  },
  (table) => [
    uniqueIndex("deployment_target_assignments_operation_unique").on(
      table.targetId,
      table.operationId,
    ),
    index("deployment_target_assignments_target_idx").on(table.targetId),
    index("deployment_target_assignments_environment_idx").on(table.environmentId),
    index("deployment_target_assignments_deployment_idx").on(table.deploymentId),
  ],
);

export const deploymentTargetsRelations = relations(deploymentTargets, ({ one, many }) => ({
  workspace: one(workspaces, {
    fields: [deploymentTargets.workspaceId],
    references: [workspaces.id],
  }),
  project: one(projects, {
    fields: [deploymentTargets.projectId],
    references: [projects.id],
  }),
  app: one(apps, {
    fields: [deploymentTargets.appId],
    references: [apps.id],
  }),
  environment: one(environments, {
    fields: [deploymentTargets.environmentId],
    references: [environments.id],
  }),
  deployment: one(deployments, {
    fields: [deploymentTargets.deploymentId],
    references: [deployments.id],
  }),
  assignments: many(deploymentTargetAssignments),
}));

export const deploymentTargetAssignmentsRelations = relations(
  deploymentTargetAssignments,
  ({ one }) => ({
    target: one(deploymentTargets, {
      fields: [deploymentTargetAssignments.targetId],
      references: [deploymentTargets.id],
    }),
    deployment: one(deployments, {
      fields: [deploymentTargetAssignments.deploymentId],
      references: [deployments.id],
    }),
  }),
);
