import crypto from "node:crypto";
import { VaultService } from "@/gen/proto/vault/v1/service_pb";
import { and, db, eq, schema } from "@/lib/db";
import { slackAppEnv } from "@/lib/env";
import { exchangeOAuthCode, postMessage, listChannels as slackListChannels } from "@/lib/slack";
import { createVaultClient } from "@/lib/vault-client";
import { TRPCError } from "@trpc/server";
import { newId } from "@unkey/id";
import { z } from "zod";
import { requireWorkspaceAdmin, t, workspaceProcedure } from "../../trpc";

const vault = createVaultClient(VaultService);

// Bot token scopes requested in the OAuth authorize URL. These MUST match the
// bot scopes configured on the Slack app, or install fails with invalid_scope.
//   chat:write         post + edit deployment/approval messages
//   chat:write.public  post to public channels the bot hasn't been invited to
//   channels:read      list public channels for the picker
//   groups:read        list private channels for the picker
//   users:read         users.info (required alongside users:read.email)
//   users:read.email   resolve a Slack user's email for admins-only approval
const SLACK_SCOPES =
  "chat:write,chat:write.public,channels:read,groups:read,users:read,users:read.email";
const STATE_TTL_MS = 15 * 60 * 1000;

// Signed state binds the install callback to the user + workspace that started
// it, so a phished callback cannot bind an install to a victim's workspace.
const signedStatePayload = z.object({
  workspaceId: z.string().min(1),
  userId: z.string().min(1),
  nonce: z.string().min(1),
  exp: z.number().int().positive(),
});
const signedState = signedStatePayload.extend({ sig: z.string().min(1) });
type SignedStatePayload = z.infer<typeof signedStatePayload>;

// Derive an HMAC key from the Slack signing secret so the state key never leaves
// the server and rotates with the Slack app credentials.
const stateSigningKey = (): Buffer | null => {
  const env = slackAppEnv();
  if (!env) {
    return null;
  }
  return crypto
    .createHash("sha256")
    .update(`unkey-slack-install-state:${env.SLACK_SIGNING_SECRET}`)
    .digest();
};

const stableStringify = (payload: SignedStatePayload): string =>
  JSON.stringify(payload, Object.keys(payload).sort());

const signState = (payload: SignedStatePayload): string => {
  const key = stateSigningKey();
  if (!key) {
    throw new TRPCError({ code: "INTERNAL_SERVER_ERROR", message: "Slack app not configured" });
  }
  const sig = crypto.createHmac("sha256", key).update(stableStringify(payload)).digest("base64url");
  return JSON.stringify({ ...payload, sig });
};

const verifyState = (raw: string): SignedStatePayload | null => {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return null;
  }
  const result = signedState.safeParse(parsed);
  if (!result.success) {
    return null;
  }
  const { sig, ...payload } = result.data;
  const key = stateSigningKey();
  if (!key) {
    return null;
  }
  const expected = crypto
    .createHmac("sha256", key)
    .update(stableStringify(payload))
    .digest("base64url");
  const a = Buffer.from(sig);
  const b = Buffer.from(expected);
  if (a.length !== b.length || !crypto.timingSafeEqual(a, b)) {
    return null;
  }
  if (payload.exp < Date.now()) {
    return null;
  }
  return payload;
};

// requireSlackEnv returns the parsed Slack app env or throws a clear error.
const requireSlackEnv = () => {
  const env = slackAppEnv();
  if (!env) {
    throw new TRPCError({
      code: "PRECONDITION_FAILED",
      message: "Slack integration is not configured",
    });
  }
  return env;
};

// findInstallation loads the workspace's Slack installation, or throws NOT_FOUND.
const findInstallation = async (workspaceId: string) => {
  const installation = await db.query.slackInstallations.findFirst({
    where: (table, { eq }) => eq(table.workspaceId, workspaceId),
  });
  if (!installation) {
    throw new TRPCError({
      code: "NOT_FOUND",
      message: "Slack is not connected for this workspace",
    });
  }
  return installation;
};

const decryptBotToken = async (workspaceId: string, encrypted: string): Promise<string> => {
  const { plaintext } = await vault.decrypt({ keyring: workspaceId, encrypted });
  return plaintext;
};

// All Slack integration mutations are admin-gated: a non-admin must not be able
// to install, repoint, downgrade the approval policy, or disconnect.
const slackAdminProcedure = workspaceProcedure.use(requireWorkspaceAdmin);

export const slackRouter = t.router({
  // Whether the workspace already has a Slack install.
  hasInstallation: workspaceProcedure.query(async ({ ctx }) => {
    const installation = await db.query.slackInstallations.findFirst({
      where: (table, { eq }) => eq(table.workspaceId, ctx.workspace.id),
      columns: { id: true },
    });
    return { installed: Boolean(installation) };
  }),

  // The current per-project connection, for the settings UI.
  getConnection: workspaceProcedure
    .input(z.object({ projectId: z.string().min(1) }))
    .query(async ({ input, ctx }) => {
      const connection = await db.query.slackProjectConnections.findFirst({
        where: (table, { and, eq }) =>
          and(eq(table.projectId, input.projectId), eq(table.workspaceId, ctx.workspace.id)),
      });
      if (!connection) {
        return null;
      }
      return {
        channelId: connection.channelId,
        channelName: connection.channelName,
        includePreviews: connection.includePreviews,
        approvalPolicy: connection.approvalPolicy,
      };
    }),

  // Mint signed state and return the Slack authorize URL to redirect to.
  prepareInstallation: slackAdminProcedure.mutation(async ({ ctx }) => {
    const env = requireSlackEnv();
    const state = signState({
      workspaceId: ctx.workspace.id,
      userId: ctx.user.id,
      nonce: crypto.randomUUID(),
      exp: Date.now() + STATE_TTL_MS,
    });
    const url = new URL("https://slack.com/oauth/v2/authorize");
    url.searchParams.set("client_id", env.SLACK_CLIENT_ID);
    url.searchParams.set("scope", SLACK_SCOPES);
    url.searchParams.set("redirect_uri", env.SLACK_REDIRECT_URI);
    url.searchParams.set("state", state);
    return { url: url.toString() };
  }),

  // Complete the OAuth handshake and persist the install.
  registerInstallation: slackAdminProcedure
    .input(z.object({ state: z.string().min(1), code: z.string().min(1) }))
    .mutation(async ({ input, ctx }) => {
      const env = requireSlackEnv();

      const parsed = verifyState(input.state);
      if (!parsed || parsed.workspaceId !== ctx.workspace.id || parsed.userId !== ctx.user.id) {
        throw new TRPCError({ code: "BAD_REQUEST", message: "Invalid or expired install state" });
      }

      const result = await exchangeOAuthCode(
        env.SLACK_CLIENT_ID,
        env.SLACK_CLIENT_SECRET,
        input.code,
        env.SLACK_REDIRECT_URI,
      ).catch((err) => {
        console.error("slack oauth exchange failed", err);
        throw new TRPCError({ code: "BAD_REQUEST", message: "Slack authorization failed" });
      });

      // Install-hijack hardening: a team already bound to another workspace
      // cannot be rebound here.
      const existing = await db.query.slackInstallations.findFirst({
        where: (table, { eq }) => eq(table.teamId, result.teamId),
        columns: { workspaceId: true },
      });
      if (existing && existing.workspaceId !== ctx.workspace.id) {
        throw new TRPCError({
          code: "CONFLICT",
          message: "This Slack workspace is already connected to another Unkey workspace",
        });
      }

      const { encrypted } = await vault.encrypt({
        keyring: ctx.workspace.id,
        data: result.botToken,
      });

      await db
        .insert(schema.slackInstallations)
        .values({
          id: newId("slack"),
          workspaceId: ctx.workspace.id,
          teamId: result.teamId,
          botToken: encrypted,
          botUserId: result.botUserId,
          installedByUserId: ctx.user.id,
          createdAt: Date.now(),
          updatedAt: null,
        })
        .onDuplicateKeyUpdate({
          set: {
            botToken: encrypted,
            botUserId: result.botUserId,
            installedByUserId: ctx.user.id,
            updatedAt: Date.now(),
          },
        })
        .catch((err) => {
          console.error("failed to persist slack installation", err);
          throw new TRPCError({
            code: "INTERNAL_SERVER_ERROR",
            message: "Failed to save Slack installation",
          });
        });

      return { teamName: result.teamName };
    }),

  // Channels the bot can post to, for the channel picker.
  listChannels: slackAdminProcedure.query(async ({ ctx }) => {
    const installation = await findInstallation(ctx.workspace.id);
    const token = await decryptBotToken(ctx.workspace.id, installation.botToken);
    const channels = await slackListChannels(token).catch((err) => {
      console.error("slack conversations.list failed", err);
      throw new TRPCError({
        code: "INTERNAL_SERVER_ERROR",
        message: "Failed to list Slack channels",
      });
    });
    return { channels };
  }),

  // Connect (or repoint) a project to a channel.
  selectChannel: slackAdminProcedure
    .input(
      z.object({
        projectId: z.string().min(1),
        channelId: z.string().min(1),
        channelName: z.string().min(1),
      }),
    )
    .mutation(async ({ input, ctx }) => {
      const installation = await findInstallation(ctx.workspace.id);

      // Confirm the project belongs to the workspace.
      const project = await db.query.projects.findFirst({
        where: (table, { and, eq }) =>
          and(eq(table.id, input.projectId), eq(table.workspaceId, ctx.workspace.id)),
        columns: { id: true },
      });
      if (!project) {
        throw new TRPCError({ code: "NOT_FOUND", message: "Project not found" });
      }

      await db
        .insert(schema.slackProjectConnections)
        .values({
          id: newId("slack"),
          workspaceId: ctx.workspace.id,
          projectId: input.projectId,
          installationId: installation.id,
          channelId: input.channelId,
          channelName: input.channelName,
          includePreviews: false,
          approvalPolicy: "anyone",
          createdAt: Date.now(),
          updatedAt: null,
        })
        .onDuplicateKeyUpdate({
          set: {
            installationId: installation.id,
            channelId: input.channelId,
            channelName: input.channelName,
            updatedAt: Date.now(),
          },
        })
        .catch((err) => {
          console.error("failed to persist slack connection", err);
          throw new TRPCError({
            code: "INTERNAL_SERVER_ERROR",
            message: "Failed to save Slack channel",
          });
        });

      return {};
    }),

  // Update environment scope and approval policy for a project.
  updateConfig: slackAdminProcedure
    .input(
      z.object({
        projectId: z.string().min(1),
        includePreviews: z.boolean().optional(),
        approvalPolicy: z.enum(["anyone", "admins_only"]).optional(),
      }),
    )
    .mutation(async ({ input, ctx }) => {
      const set: {
        includePreviews?: boolean;
        approvalPolicy?: "anyone" | "admins_only";
        updatedAt: number;
      } = {
        updatedAt: Date.now(),
      };
      if (input.includePreviews !== undefined) {
        set.includePreviews = input.includePreviews;
      }
      if (input.approvalPolicy !== undefined) {
        set.approvalPolicy = input.approvalPolicy;
      }

      await db
        .update(schema.slackProjectConnections)
        .set(set)
        .where(
          and(
            eq(schema.slackProjectConnections.projectId, input.projectId),
            eq(schema.slackProjectConnections.workspaceId, ctx.workspace.id),
          ),
        );
      return {};
    }),

  // Post a test message to the connected channel.
  sendTestMessage: slackAdminProcedure
    .input(z.object({ projectId: z.string().min(1) }))
    .mutation(async ({ input, ctx }) => {
      const connection = await db.query.slackProjectConnections.findFirst({
        where: (table, { and, eq }) =>
          and(eq(table.projectId, input.projectId), eq(table.workspaceId, ctx.workspace.id)),
      });
      if (!connection) {
        throw new TRPCError({
          code: "NOT_FOUND",
          message: "This project has no Slack channel connected",
        });
      }
      const installation = await findInstallation(ctx.workspace.id);
      const token = await decryptBotToken(ctx.workspace.id, installation.botToken);
      await postMessage(
        token,
        connection.channelId,
        "✅ Unkey is connected. Deployment notifications for this project will appear here.",
      ).catch((err) => {
        console.error("slack test message failed", err);
        throw new TRPCError({
          code: "INTERNAL_SERVER_ERROR",
          message: "Failed to send test message",
        });
      });
      return {};
    }),

  // Disconnect a single project's channel.
  disconnect: slackAdminProcedure
    .input(z.object({ projectId: z.string().min(1) }))
    .mutation(async ({ input, ctx }) => {
      await db
        .delete(schema.slackProjectConnections)
        .where(
          and(
            eq(schema.slackProjectConnections.projectId, input.projectId),
            eq(schema.slackProjectConnections.workspaceId, ctx.workspace.id),
          ),
        );
      return {};
    }),

  // Revoke the whole workspace install; cascades to all project connections so
  // no further notifications are sent (R5).
  revoke: slackAdminProcedure.mutation(async ({ ctx }) => {
    await db
      .delete(schema.slackProjectConnections)
      .where(eq(schema.slackProjectConnections.workspaceId, ctx.workspace.id));
    await db
      .delete(schema.slackInstallations)
      .where(eq(schema.slackInstallations.workspaceId, ctx.workspace.id));
    return {};
  }),
});
