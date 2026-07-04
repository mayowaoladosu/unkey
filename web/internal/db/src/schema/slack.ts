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

// A per-project notification destination. One channel per project (unique on
// project_id) — per-environment routing to different channels is out of scope.
export const slackProjectConnections = mysqlTable(
  "slack_project_connections",
  {
    pk: bigint("pk", { mode: "number", unsigned: true }).autoincrement().primaryKey(),
    id: varchar("id", { length: 128 }).notNull().unique(),
    workspaceId: varchar("workspace_id", { length: 256 }).notNull(),
    projectId: varchar("project_id", { length: 64 }).notNull().unique(),
    // References slackInstallations.id.
    installationId: varchar("installation_id", { length: 128 }).notNull(),
    channelId: varchar("channel_id", { length: 64 }).notNull(),
    channelName: varchar("channel_name", { length: 256 }).notNull(),
    // Production deployments always notify; previews only when this is set.
    includePreviews: boolean("include_previews").notNull().default(false),
    // Who may approve/reject a gated deployment from Slack.
    approvalPolicy: mysqlEnum("approval_policy", ["anyone", "admins_only"])
      .notNull()
      .default("anyone"),
    ...lifecycleDates,
  },
  (table) => [index("slack_installation_id_idx").on(table.installationId)],
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
