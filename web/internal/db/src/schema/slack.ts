import { relations } from "drizzle-orm";
import {
  bigint,
  boolean,
  index,
  mysqlEnum,
  mysqlTable,
  uniqueIndex,
  varchar,
} from "drizzle-orm/mysql-core";
import { projects } from "./projects";
import { lifecycleDates } from "./util/lifecycle_dates";
import { workspaces } from "./workspaces";

// A per-workspace Slack OAuth install. One row per (workspace, Slack team).
// The bot token is stored vault-encrypted (recoverable: decryptable
// server-side, never returned to the browser) and is keyed in vault by
// workspaceId — a workspace install has no environment, so it does NOT use the
// environmentId keyring that app_environment_variables uses.
export const slackInstallations = mysqlTable(
  "slack_installations",
  {
    pk: bigint("pk", { mode: "number", unsigned: true }).autoincrement().primaryKey(),
    id: varchar("id", { length: 128 }).notNull().unique(),
    workspaceId: varchar("workspace_id", { length: 256 }).notNull(),
    // Slack workspace ("team") identifier returned by oauth.v2.access.
    teamId: varchar("team_id", { length: 64 }).notNull(),
    // Vault-encrypted blob (keyId, nonce, ciphertext), keyring = workspaceId.
    botToken: varchar("bot_token", { length: 4096 }).notNull(),
    botUserId: varchar("bot_user_id", { length: 64 }).notNull(),
    installedByUserId: varchar("installed_by_user_id", { length: 256 }).notNull(),
    ...lifecycleDates,
  },
  (table) => [uniqueIndex("workspace_team_idx").on(table.workspaceId, table.teamId)],
);

export const slackInstallationsRelations = relations(slackInstallations, ({ one }) => ({
  workspace: one(workspaces, {
    fields: [slackInstallations.workspaceId],
    references: [workspaces.id],
  }),
}));

// A per-project notification destination. A project can fan out to any number
// of channels; each channel row carries its own environment scope so e.g. a
// #deploys channel gets production only while #previews gets previews too.
export const slackProjectConnections = mysqlTable(
  "slack_project_connections",
  {
    pk: bigint("pk", { mode: "number", unsigned: true }).autoincrement().primaryKey(),
    id: varchar("id", { length: 128 }).notNull().unique(),
    workspaceId: varchar("workspace_id", { length: 256 }).notNull(),
    projectId: varchar("project_id", { length: 64 }).notNull(),
    // References slackInstallations.id.
    installationId: varchar("installation_id", { length: 128 }).notNull(),
    channelId: varchar("channel_id", { length: 64 }).notNull(),
    channelName: varchar("channel_name", { length: 256 }).notNull(),
    // Per-channel environment scope.
    notifyProduction: boolean("notify_production").notNull().default(true),
    notifyPreviews: boolean("notify_previews").notNull().default(false),
    ...lifecycleDates,
  },
  (table) => [
    uniqueIndex("project_channel_idx").on(table.projectId, table.channelId),
    index("slack_installation_id_idx").on(table.installationId),
  ],
);

export const slackProjectConnectionsRelations = relations(slackProjectConnections, ({ one }) => ({
  workspace: one(workspaces, {
    fields: [slackProjectConnections.workspaceId],
    references: [workspaces.id],
  }),
  project: one(projects, {
    fields: [slackProjectConnections.projectId],
    references: [projects.id],
  }),
  installation: one(slackInstallations, {
    fields: [slackProjectConnections.installationId],
    references: [slackInstallations.id],
  }),
}));

// Project-level Slack settings that are independent of any single channel.
// approvalPolicy is one such setting: who may approve/reject a gated deployment
// from Slack is a property of the project, not of each notification channel, so
// it lives here (one row per project) instead of being duplicated across every
// slack_project_connections row.
export const slackProjectSettings = mysqlTable(
  "slack_project_settings",
  {
    pk: bigint("pk", { mode: "number", unsigned: true }).autoincrement().primaryKey(),
    id: varchar("id", { length: 128 }).notNull().unique(),
    workspaceId: varchar("workspace_id", { length: 256 }).notNull(),
    projectId: varchar("project_id", { length: 64 }).notNull(),
    // Who may approve/reject a gated deployment from Slack.
    approvalPolicy: mysqlEnum("approval_policy", ["anyone", "admins_only"])
      .notNull()
      .default("anyone"),
    ...lifecycleDates,
  },
  (table) => [uniqueIndex("slack_project_settings_project_idx").on(table.projectId)],
);

export const slackProjectSettingsRelations = relations(slackProjectSettings, ({ one }) => ({
  workspace: one(workspaces, {
    fields: [slackProjectSettings.workspaceId],
    references: [workspaces.id],
  }),
  project: one(projects, {
    fields: [slackProjectSettings.projectId],
    references: [projects.id],
  }),
}));
