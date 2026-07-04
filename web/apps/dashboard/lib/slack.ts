import crypto from "node:crypto";

// Thin, hand-rolled Slack Web API client for the dashboard side of the Slack
// integration (OAuth install, channel picker, identity resolution, and posting
// test/approval-resolution messages). Mirrors the hand-rolled GitHub helper
// style rather than pulling in the slack-go / @slack/web-api SDKs.

const SLACK_API = "https://slack.com/api";
// Reject interaction requests whose signed timestamp is older than this, to
// bound replay of a captured payload.
const SIGNATURE_MAX_AGE_SECONDS = 5 * 60;

type SlackErrorResponse = { ok: false; error?: string };
type SlackOk<T> = T & { ok: true };

async function slackGet<T>(
  botToken: string,
  method: string,
  params: Record<string, string>,
): Promise<SlackOk<T>> {
  const url = new URL(`${SLACK_API}/${method}`);
  for (const [k, v] of Object.entries(params)) {
    url.searchParams.set(k, v);
  }
  const res = await fetch(url, {
    headers: { Authorization: `Bearer ${botToken}` },
  });
  return parseSlack<T>(method, res);
}

async function slackPostForm<T>(
  method: string,
  params: Record<string, string>,
): Promise<SlackOk<T>> {
  const res = await fetch(`${SLACK_API}/${method}`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams(params).toString(),
  });
  return parseSlack<T>(method, res);
}

async function slackPostJSON<T>(
  botToken: string,
  method: string,
  body: unknown,
): Promise<SlackOk<T>> {
  const res = await fetch(`${SLACK_API}/${method}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json; charset=utf-8",
      Authorization: `Bearer ${botToken}`,
    },
    body: JSON.stringify(body),
  });
  return parseSlack<T>(method, res);
}

async function parseSlack<T>(method: string, res: Response): Promise<SlackOk<T>> {
  if (!res.ok) {
    throw new Error(`slack ${method} returned HTTP ${res.status}`);
  }
  // The Web API always returns HTTP 200; application errors live in ok/error.
  const json = (await res.json()) as SlackOk<T> | SlackErrorResponse;
  if (!json.ok) {
    throw new Error(
      `slack ${method} failed: ${(json as SlackErrorResponse).error ?? "unknown_error"}`,
    );
  }
  return json as SlackOk<T>;
}

export type SlackInstallResult = {
  botToken: string;
  teamId: string;
  teamName: string;
  botUserId: string;
  authedUserId: string;
};

// exchangeOAuthCode completes the OAuth handshake, returning the per-workspace
// bot token and the installing team's identity.
export async function exchangeOAuthCode(
  clientId: string,
  clientSecret: string,
  code: string,
  redirectUri: string,
): Promise<SlackInstallResult> {
  const data = await slackPostForm<{
    access_token: string;
    bot_user_id: string;
    team: { id: string; name: string };
    authed_user: { id: string };
  }>("oauth.v2.access", {
    client_id: clientId,
    client_secret: clientSecret,
    code,
    redirect_uri: redirectUri,
  });
  return {
    botToken: data.access_token,
    teamId: data.team.id,
    teamName: data.team.name,
    botUserId: data.bot_user_id,
    authedUserId: data.authed_user.id,
  };
}

export type SlackChannel = { id: string; name: string };

// listChannels returns the channels the bot can post to (public + private it is
// a member of), following pagination.
export async function listChannels(botToken: string): Promise<SlackChannel[]> {
  const channels: SlackChannel[] = [];
  let cursor = "";
  do {
    const params: Record<string, string> = {
      types: "public_channel,private_channel",
      exclude_archived: "true",
      limit: "200",
    };
    if (cursor) {
      params.cursor = cursor;
    }
    const page = await slackGet<{
      channels: SlackChannel[];
      response_metadata?: { next_cursor?: string };
    }>(botToken, "conversations.list", params);
    for (const c of page.channels) {
      channels.push({ id: c.id, name: c.name });
    }
    cursor = page.response_metadata?.next_cursor ?? "";
  } while (cursor);
  return channels;
}

// getUserEmail resolves a Slack user id to their email (requires users:read.email).
// Returns null when the user has no email visible to the bot.
export async function getUserEmail(botToken: string, userId: string): Promise<string | null> {
  const data = await slackGet<{ user: { profile?: { email?: string } } }>(botToken, "users.info", {
    user: userId,
  });
  return data.user.profile?.email ?? null;
}

// postMessage posts a message and returns its channel + ts.
export async function postMessage(
  botToken: string,
  channel: string,
  text: string,
  blocks?: unknown[],
): Promise<{ channel: string; ts: string }> {
  const data = await slackPostJSON<{ channel: string; ts: string }>(botToken, "chat.postMessage", {
    channel,
    text,
    ...(blocks ? { blocks } : {}),
  });
  return { channel: data.channel, ts: data.ts };
}

// APPROVAL_BLOCK_PREFIX namespaces the approval actions block_id. It MUST match
// the Go producer's approvalBlockIDPrefix in svc/ctrl/worker/slackstatus/message.go.
export const APPROVAL_BLOCK_PREFIX = "slack_deploy_approval";

export type ApprovalRef = { deploymentId: string; workspaceId: string };

// parseApprovalBlockId parses "slack_deploy_approval:<deploymentId>:<workspaceId>".
// Returns null for any other block_id. These are lookup keys only — the handler
// re-verifies tenancy server-side (KTD9).
export function parseApprovalBlockId(blockId: string): ApprovalRef | null {
  const parts = blockId.split(":");
  if (parts.length !== 3 || parts[0] !== APPROVAL_BLOCK_PREFIX) {
    return null;
  }
  if (!parts[1] || !parts[2]) {
    return null;
  }
  return { deploymentId: parts[1], workspaceId: parts[2] };
}

// isTeamBound enforces KTD9: a signed interaction may act only if its Slack team
// matches the installation bound to the deployment's workspace. A valid
// signature alone only proves the request came from some install of the app.
export function isTeamBound(payloadTeamId: string, installationTeamId: string): boolean {
  return payloadTeamId !== "" && payloadTeamId === installationTeamId;
}

export type ApprovalPolicy = "anyone" | "admins_only";

// authorizeSlackApproval decides whether the actor may resolve a gated
// deployment under the project's policy (R10/AE3). In admins_only mode the actor
// must resolve to a workspace member with the admin role; the default open
// policy allows anyone whose interaction passed signature + team binding.
export function authorizeSlackApproval(
  policy: ApprovalPolicy,
  member: { role: string } | null | undefined,
): boolean {
  if (policy === "anyone") {
    return true;
  }
  return member?.role === "admin";
}

// verifySignature validates a Slack request signature over the raw body and
// rejects stale timestamps. Constant-time compare; returns false on any failure.
export function verifySignature(
  signingSecret: string,
  signatureHeader: string | null,
  timestampHeader: string | null,
  rawBody: string,
): boolean {
  if (!signatureHeader || !timestampHeader) {
    return false;
  }
  const timestamp = Number.parseInt(timestampHeader, 10);
  if (!Number.isFinite(timestamp)) {
    return false;
  }
  const nowSeconds = Math.floor(Date.now() / 1000);
  if (Math.abs(nowSeconds - timestamp) > SIGNATURE_MAX_AGE_SECONDS) {
    return false;
  }
  const base = `v0:${timestampHeader}:${rawBody}`;
  const expected = `v0=${crypto.createHmac("sha256", signingSecret).update(base).digest("hex")}`;
  const a = Buffer.from(signatureHeader);
  const b = Buffer.from(expected);
  return a.length === b.length && crypto.timingSafeEqual(a, b);
}
