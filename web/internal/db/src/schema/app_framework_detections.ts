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
import { projects } from "./projects";
import { lifecycleDates } from "./util/lifecycle_dates";
import { workspaces } from "./workspaces";

export type FrameworkDetectionDefaults = {
  // Null means "no safe recommendation" and must preserve the effective
  // setting. It is not an instruction to clear a user override.
  rootDirectory: string | null;
  dockerfile: string | null;
  buildCommand: string | null;
};

/**
 * Cached advisory repository analysis. Effective build configuration remains
 * in app_build_settings, so detected facts and accepted overrides never share
 * an authority boundary.
 */
export const appFrameworkDetections = mysqlTable(
  "app_framework_detections",
  {
    pk: bigint("pk", { mode: "number", unsigned: true }).autoincrement().primaryKey(),
    workspaceId: varchar("workspace_id", { length: 256 }).notNull(),
    projectId: varchar("project_id", { length: 64 }).notNull(),
    appId: varchar("app_id", { length: 64 }).notNull(),

    repositoryFullName: varchar("repository_full_name", { length: 500 }).notNull(),
    branch: varchar("branch", { length: 256 }).notNull(),
    treeSha: varchar("tree_sha", { length: 64 }).notNull(),
    fingerprint: varchar("fingerprint", { length: 64 }).notNull(),
    detectionVersion: bigint("detection_version", { mode: "number", unsigned: true })
      .notNull()
      .default(1),

    detectedPresetId: varchar("detected_preset_id", { length: 128 }),
    detectedPresetName: varchar("detected_preset_name", { length: 256 }),
    confidence: mysqlEnum("confidence", ["none", "low", "medium", "high"])
      .notNull()
      .default("none"),
    buildStrategy: mysqlEnum("build_strategy", ["dockerfile", "zero-config", "unknown"])
      .notNull()
      .default("unknown"),

    detection: json("detection").$type<unknown>().notNull(),
    defaults: json("defaults").$type<FrameworkDetectionDefaults>().notNull(),
    detectedAt: bigint("detected_at", { mode: "number" }).notNull(),

    appliedFingerprint: varchar("applied_fingerprint", { length: 64 }),
    appliedDefaults: json("applied_defaults").$type<FrameworkDetectionDefaults>(),
    appliedAt: bigint("applied_at", { mode: "number" }),

    ...lifecycleDates,
  },
  (table) => [
    uniqueIndex("app_framework_detections_app_idx").on(table.appId),
    index("app_framework_detections_workspace_idx").on(table.workspaceId),
    index("app_framework_detections_project_idx").on(table.projectId),
  ],
);

export const appFrameworkDetectionsRelations = relations(appFrameworkDetections, ({ one }) => ({
  workspace: one(workspaces, {
    fields: [appFrameworkDetections.workspaceId],
    references: [workspaces.id],
  }),
  project: one(projects, {
    fields: [appFrameworkDetections.projectId],
    references: [projects.id],
  }),
  app: one(apps, {
    fields: [appFrameworkDetections.appId],
    references: [apps.id],
  }),
}));
