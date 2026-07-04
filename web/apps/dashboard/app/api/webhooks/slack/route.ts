import { DeployService } from "@/gen/proto/ctrl/v1/deployment_pb";
import { VaultService } from "@/gen/proto/vault/v1/service_pb";
import { insertAuditLogs } from "@/lib/audit";
import { auth } from "@/lib/auth/server";
import { createCtrlClient } from "@/lib/ctrl-client";
import { db } from "@/lib/db";
import { slackAppEnv } from "@/lib/env";
import {
  authorizeSlackApproval,
  getUserEmail,
  isTeamBound,
  parseApprovalBlockId,
  verifySignature,
} from "@/lib/slack";
import { createVaultClient } from "@/lib/vault-client";
import { Code, ConnectError } from "@connectrpc/connect";

const vault = createVaultClient(VaultService);

// Minimal shape of the Slack block_actions interaction payload we consume.
type SlackInteraction = {
  type?: string;
  user?: { id?: string; username?: string; name?: string };
  team?: { id?: string };
  response_url?: string;
  actions?: Array<{ action_id?: string; block_id?: string; value?: string }>;
};

// ephemeral returns a 200 with an ephemeral message shown only to the clicker.
function ephemeral(text: string): Response {
  return Response.json({ response_type: "ephemeral", replace_original: false, text });
}

// updateOriginalMessage replaces the approval prompt with its resolved state via
// the interaction's response_url (no bot token required).
async function updateOriginalMessage(responseUrl: string, text: string): Promise<void> {
  try {
    await fetch(responseUrl, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ replace_original: true, text }),
    });
  } catch (err) {
    console.error("failed to update slack message via response_url", err);
  }
}

export const POST = async (req: Request): Promise<Response> => {
  const env = slackAppEnv();
  if (!env) {
    return new Response("Slack integration not configured", { status: 503 });
  }

  // Signatures are computed over the raw body, so read it verbatim.
  const rawBody = await req.text();
  const signature = req.headers.get("x-slack-signature");
  const timestamp = req.headers.get("x-slack-request-timestamp");

  if (!verifySignature(env.SLACK_SIGNING_SECRET, signature, timestamp, rawBody)) {
    return new Response("invalid signature", { status: 401 });
  }

  // Body is application/x-www-form-urlencoded with a `payload` JSON field.
  const params = new URLSearchParams(rawBody);
  const payloadRaw = params.get("payload");
  if (!payloadRaw) {
    return new Response("missing payload", { status: 400 });
  }

  let interaction: SlackInteraction;
  try {
    interaction = JSON.parse(payloadRaw) as SlackInteraction;
  } catch {
    return new Response("invalid payload", { status: 400 });
  }

  const action = interaction.actions?.[0];
  if (interaction.type !== "block_actions" || !action?.block_id || !action.action_id) {
    // Nothing actionable — ack so Slack does not retry.
    return new Response(null, { status: 200 });
  }

  // action_id/block_id are lookup keys only; tenancy is re-derived server-side.
  const ref = parseApprovalBlockId(action.block_id);
  if (!ref) {
    return new Response(null, { status: 200 });
  }
  const deploymentId = ref.deploymentId;
  const claimedWorkspaceId = ref.workspaceId;
  const decision = action.action_id; // "approve" | "reject"
  const responseUrl = interaction.response_url ?? "";
  const slackTeamId = interaction.team?.id ?? "";
  const slackUserId = interaction.user?.id ?? "";
  const slackUsername = interaction.user?.username ?? interaction.user?.name ?? slackUserId;

  // --- Tenant binding (KTD9): the signed request only proves it came from some
  // install of the Unkey app. Require the payload team to match the
  // installation bound to the deployment's workspace, in both policy modes.
  const installation = await db.query.slackInstallations.findFirst({
    where: (table, { eq }) => eq(table.workspaceId, claimedWorkspaceId),
  });
  if (!installation || !isTeamBound(slackTeamId, installation.teamId)) {
    return ephemeral("This Slack workspace is not authorized to act on this deployment.");
  }

  // Deployment must exist and belong to the claimed workspace.
  const deployment = await db.query.deployments.findFirst({
    where: (table, { and, eq }) =>
      and(eq(table.id, deploymentId), eq(table.workspaceId, claimedWorkspaceId)),
    columns: { id: true, projectId: true },
    with: { project: { columns: { name: true } } },
  });
  if (!deployment) {
    return ephemeral("Deployment not found.");
  }

  const connection = await db.query.slackProjectConnections.findFirst({
    where: (table, { and, eq }) =>
      and(eq(table.projectId, deployment.projectId), eq(table.workspaceId, claimedWorkspaceId)),
    columns: { approvalPolicy: true },
  });

  // --- Authorization: default open, optional admins-only (WorkOS admin role).
  let actorUserId: string | null = null;
  let actorEmail: string | null = null;
  if (connection?.approvalPolicy === "admins_only") {
    const workspace = await db.query.workspaces.findFirst({
      where: (table, { eq }) => eq(table.id, claimedWorkspaceId),
      columns: { orgId: true },
    });
    if (!workspace) {
      return ephemeral("Workspace not found.");
    }

    let email: string | null = null;
    try {
      const token = (
        await vault.decrypt({ keyring: claimedWorkspaceId, encrypted: installation.botToken })
      ).plaintext;
      email = await getUserEmail(token, slackUserId);
    } catch (err) {
      console.error("failed to resolve slack user email", err);
    }
    if (!email) {
      return ephemeral("Could not verify your Unkey identity. Approve from the dashboard instead.");
    }

    const members = await auth.getOrganizationMemberList(workspace.orgId);
    const member = members.data.find((m) => m.user.email?.toLowerCase() === email.toLowerCase());
    if (!authorizeSlackApproval("admins_only", member)) {
      return ephemeral("Only workspace admins can approve or reject this deployment.");
    }
    actorUserId = member?.user.id ?? null;
    actorEmail = email;
  }

  // --- Apply the decision. The ctrl CAS is the authoritative already-resolved
  // guard (KTD9): FailedPrecondition means someone already resolved it.
  const ctrl = createCtrlClient(DeployService);
  try {
    if (decision === "approve") {
      await ctrl.authorizeDeployment({ deploymentId });
    } else if (decision === "reject") {
      await ctrl.rejectDeployment({ deploymentId });
    } else {
      return new Response(null, { status: 200 });
    }
  } catch (err) {
    if (err instanceof ConnectError && err.code === Code.FailedPrecondition) {
      await updateOriginalMessage(responseUrl, "This deployment has already been resolved.");
      return ephemeral("This deployment was already resolved.");
    }
    console.error("slack deployment action failed", err);
    return ephemeral("Something went wrong applying your decision. Try the dashboard.");
  }

  const verb = decision === "approve" ? "Approved" : "Rejected";
  const emoji = decision === "approve" ? "✅" : "❌";
  await updateOriginalMessage(responseUrl, `${emoji} ${verb} by ${slackUsername}`);

  // --- Audit. Use the resolved Unkey user when linkable; otherwise a dedicated
  // "slack" actor so Slack-driven decisions are distinguishable from genuine
  // system actions in the audit trail (mirrors SlackActor in pkg/auditlog).
  await insertAuditLogs(db, {
    workspaceId: claimedWorkspaceId,
    actor: actorUserId
      ? { type: "user", id: actorUserId, name: actorEmail ?? slackUsername }
      : {
          type: "slack",
          id: slackUserId,
          name: slackUsername,
          meta: { slackUserId, slackTeamId },
        },
    event: decision === "approve" ? "deployment.authorize" : "deployment.reject",
    description: `${verb} deployment ${deploymentId} from Slack for ${deployment.project.name}`,
    resources: [{ type: "project", id: deployment.projectId, name: deployment.project.name }],
    context: {
      location: req.headers.get("x-forwarded-for") ?? "",
      userAgent: req.headers.get("user-agent") ?? undefined,
    },
  }).catch((err) => {
    // Never fail the interaction on an audit write error; the action applied.
    console.error("failed to write slack action audit log", err);
  });

  return new Response(null, { status: 200 });
};
