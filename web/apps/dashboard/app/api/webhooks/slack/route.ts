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
  parseApprovalBlockId,
  verifySignature,
} from "@/lib/slack";
import { createVaultClient } from "@/lib/vault-client";
import { Code, ConnectError } from "@connectrpc/connect";
import { after } from "next/server";

const vault = createVaultClient(VaultService);

// Minimal shape of the Slack block_actions interaction payload we consume.
type SlackInteraction = {
  type?: string;
  user?: { id?: string; username?: string; name?: string };
  team?: { id?: string };
  response_url?: string;
  actions?: Array<{ action_id?: string; block_id?: string; value?: string }>;
};

// postEphemeral shows a message only to the clicker. Slack does NOT render the
// HTTP ack body for block_actions, so all feedback must be POSTed to the
// interaction's response_url. Best-effort: a failure here only costs the user a
// confirmation message, never the decision itself.
async function postEphemeral(responseUrl: string, text: string): Promise<void> {
  if (!responseUrl) {
    return;
  }
  try {
    await fetch(responseUrl, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ response_type: "ephemeral", replace_original: false, text }),
    });
  } catch (err) {
    console.error("failed to post ephemeral slack message via response_url", err);
  }
}

// Everything needed to resolve an interaction, extracted from the raw request so
// the work can run after the HTTP ack (see POST) and be unit-tested directly.
export type ApprovalInteraction = {
  deploymentId: string;
  claimedWorkspaceId: string;
  decision: string; // "approve" | "reject"
  responseUrl: string;
  slackTeamId: string;
  slackUserId: string;
  slackUsername: string;
  forwardedFor: string;
  userAgent: string | undefined;
};

// handleApprovalInteraction applies a Slack approve/reject click. It re-derives
// all tenancy and authorization server-side (the signed payload only proves the
// request came from some install of the Unkey app) and reports the outcome to
// the clicker via response_url. Runs after the HTTP ack, so it never returns a
// Response — it throws only on unexpected errors, which the caller reports.
export async function handleApprovalInteraction(i: ApprovalInteraction): Promise<void> {
  // --- Tenant binding (KTD9): look up the installation by BOTH the deployment's
  // workspace and the payload's team id (a workspace may have more than one
  // Slack team installed, `(workspace_id, team_id)` is unique). Finding a row is
  // itself the binding: no match means this team is not authorized to act on
  // this deployment's workspace.
  const installation = await db.query.slackInstallations.findFirst({
    where: (table, { and, eq }) =>
      and(eq(table.workspaceId, i.claimedWorkspaceId), eq(table.teamId, i.slackTeamId)),
  });
  if (!installation) {
    await postEphemeral(
      i.responseUrl,
      "This Slack workspace is not authorized to act on this deployment.",
    );
    return;
  }

  // Deployment must exist and belong to the claimed workspace.
  const deployment = await db.query.deployments.findFirst({
    where: (table, { and, eq }) =>
      and(eq(table.id, i.deploymentId), eq(table.workspaceId, i.claimedWorkspaceId)),
    columns: { id: true, projectId: true },
    with: { project: { columns: { name: true } } },
  });
  if (!deployment) {
    await postEphemeral(i.responseUrl, "Deployment not found.");
    return;
  }

  // A stale button (Slack keeps message history after a disconnect) must not
  // resolve a deployment for a project that no longer has Slack connected.
  const connections = await db.query.slackProjectConnections.findMany({
    where: (table, { and, eq }) =>
      and(eq(table.projectId, deployment.projectId), eq(table.workspaceId, i.claimedWorkspaceId)),
    columns: { id: true },
  });
  if (connections.length === 0) {
    await postEphemeral(
      i.responseUrl,
      "Slack is no longer connected for this project; approve or reject from the dashboard.",
    );
    return;
  }

  // Approval policy is a single project-scoped row; absent means the default.
  const settings = await db.query.slackProjectSettings.findFirst({
    where: (table, { and, eq }) =>
      and(eq(table.projectId, deployment.projectId), eq(table.workspaceId, i.claimedWorkspaceId)),
    columns: { approvalPolicy: true },
  });
  const adminsOnly = settings?.approvalPolicy === "admins_only";

  // --- Authorization: default open, optional admins-only (WorkOS admin role).
  let actorUserId: string | null = null;
  let actorEmail: string | null = null;
  if (adminsOnly) {
    const workspace = await db.query.workspaces.findFirst({
      where: (table, { eq }) => eq(table.id, i.claimedWorkspaceId),
      columns: { orgId: true },
    });
    if (!workspace) {
      await postEphemeral(i.responseUrl, "Workspace not found.");
      return;
    }

    let email: string | null = null;
    try {
      const token = (
        await vault.decrypt({ keyring: i.claimedWorkspaceId, encrypted: installation.botToken })
      ).plaintext;
      email = await getUserEmail(token, i.slackUserId);
    } catch (err) {
      console.error("failed to resolve slack user email", err);
    }
    if (!email) {
      await postEphemeral(
        i.responseUrl,
        "Could not verify your Unkey identity. Approve from the dashboard instead.",
      );
      return;
    }

    // Look the clicker up directly rather than paging the whole org: a large org
    // is truncated by the member-list page size, which would deny admins beyond
    // the first page. listMemberships is scoped to the one user.
    const user = await auth.findUser(email);
    const membership = user
      ? (await auth.listMemberships(user.id)).data.find(
          (m) => m.organization.id === workspace.orgId,
        )
      : undefined;
    if (!authorizeSlackApproval("admins_only", membership)) {
      await postEphemeral(
        i.responseUrl,
        "Only workspace admins can approve or reject this deployment.",
      );
      return;
    }
    actorUserId = user?.id ?? null;
    actorEmail = email;
  }

  // --- Apply the decision. resolvedBy attributes the decision on the retired
  // prompt; ctrl fires ResolveApproval, which is the single writer that edits
  // the prompt message(s) to their resolved state across every channel — the
  // route does not edit the prompt itself (avoids a racing text-only rewrite).
  // The ctrl CAS is the authoritative already-resolved guard (KTD9):
  // FailedPrecondition means someone already resolved it.
  const ctrl = createCtrlClient(DeployService);
  try {
    if (i.decision === "approve") {
      await ctrl.authorizeDeployment({ deploymentId: i.deploymentId, resolvedBy: i.slackUsername });
    } else if (i.decision === "reject") {
      await ctrl.rejectDeployment({ deploymentId: i.deploymentId, resolvedBy: i.slackUsername });
    } else {
      return;
    }
  } catch (err) {
    if (err instanceof ConnectError && err.code === Code.FailedPrecondition) {
      await postEphemeral(i.responseUrl, "This deployment was already resolved.");
      return;
    }
    console.error("slack deployment action failed", err);
    await postEphemeral(
      i.responseUrl,
      "Something went wrong applying your decision. Try the dashboard.",
    );
    return;
  }

  const verb = i.decision === "approve" ? "approved" : "rejected";

  // --- Audit. Use the resolved Unkey user when linkable; otherwise a dedicated
  // "slack" actor so Slack-driven decisions are distinguishable from genuine
  // system actions in the audit trail (mirrors SlackActor in pkg/auditlog).
  await insertAuditLogs(db, {
    workspaceId: i.claimedWorkspaceId,
    actor: actorUserId
      ? { type: "user", id: actorUserId, name: actorEmail ?? i.slackUsername }
      : {
          type: "slack",
          id: i.slackUserId,
          name: i.slackUsername,
          meta: { slackUserId: i.slackUserId, slackTeamId: i.slackTeamId },
        },
    event: i.decision === "approve" ? "deployment.authorize" : "deployment.reject",
    description: `Deployment ${i.deploymentId} ${verb} from Slack for ${deployment.project.name}`,
    resources: [{ type: "project", id: deployment.projectId, name: deployment.project.name }],
    context: {
      location: i.forwardedFor,
      userAgent: i.userAgent,
    },
  }).catch((err) => {
    // Never fail the interaction on an audit write error; the action applied.
    console.error("failed to write slack action audit log", err);
  });

  // Immediate confirmation to the clicker; the prompt itself is retired
  // (buttons removed, outcome + attribution rendered) by ResolveApproval.
  await postEphemeral(i.responseUrl, `Deployment ${verb}.`);
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

  // Applying the decision touches the DB, the Slack Web API, WorkOS, and ctrl —
  // in admins_only mode that chain can exceed Slack's 3-second ack deadline,
  // after which Slack retries and shows the user a timeout. Ack immediately and
  // finish the work after the response; all user feedback flows to response_url
  // regardless, so nothing depends on the ack body.
  const interactionData: ApprovalInteraction = {
    deploymentId: ref.deploymentId,
    claimedWorkspaceId: ref.workspaceId,
    decision: action.action_id,
    responseUrl: interaction.response_url ?? "",
    slackTeamId: interaction.team?.id ?? "",
    slackUserId: interaction.user?.id ?? "",
    slackUsername:
      interaction.user?.username ?? interaction.user?.name ?? interaction.user?.id ?? "",
    forwardedFor: req.headers.get("x-forwarded-for") ?? "",
    userAgent: req.headers.get("user-agent") ?? undefined,
  };

  after(async () => {
    try {
      await handleApprovalInteraction(interactionData);
    } catch (err) {
      console.error("slack interaction processing failed", err);
      await postEphemeral(
        interactionData.responseUrl,
        "Something went wrong applying your decision. Try the dashboard.",
      );
    }
  });

  return new Response(null, { status: 200 });
};
