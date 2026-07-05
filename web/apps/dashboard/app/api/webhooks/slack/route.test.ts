// @vitest-environment node

import { Code, ConnectError } from "@connectrpc/connect";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// Hoisted so the (hoisted) vi.mock factories below can reference them.
const { decrypt, authorizeDeployment, rejectDeployment, insertAuditLogs, getUserEmail } =
  vi.hoisted(() => ({
    decrypt: vi.fn(),
    authorizeDeployment: vi.fn(),
    rejectDeployment: vi.fn(),
    insertAuditLogs: vi.fn(),
    getUserEmail: vi.fn(),
  }));

vi.mock("@/lib/env", () => ({
  slackAppEnv: vi.fn(),
}));

vi.mock("@/lib/db", () => ({
  db: {
    query: {
      slackInstallations: { findFirst: vi.fn() },
      deployments: { findFirst: vi.fn() },
      slackProjectConnections: { findMany: vi.fn() },
      slackProjectSettings: { findFirst: vi.fn() },
      workspaces: { findFirst: vi.fn() },
    },
  },
}));

vi.mock("@/lib/auth/server", () => ({
  auth: {
    findUser: vi.fn(),
    listMemberships: vi.fn(),
  },
}));

vi.mock("@/lib/vault-client", () => ({
  createVaultClient: () => ({ decrypt }),
}));

vi.mock("@/lib/ctrl-client", () => ({
  createCtrlClient: () => ({ authorizeDeployment, rejectDeployment }),
}));

vi.mock("@/lib/audit", () => ({
  insertAuditLogs: (...args: unknown[]) => insertAuditLogs(...args),
}));

// getUserEmail hits the Slack Web API; stub it. Keep the real authorization
// helpers (authorizeSlackApproval etc.).
vi.mock("@/lib/slack", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/slack")>()),
  getUserEmail: (...args: unknown[]) => getUserEmail(...args),
}));

import { auth } from "@/lib/auth/server";
import { db } from "@/lib/db";
import { type ApprovalInteraction, handleApprovalInteraction } from "./route";

const mockInstall = vi.mocked(db.query.slackInstallations.findFirst);
const mockDeployment = vi.mocked(db.query.deployments.findFirst);
const mockConnections = vi.mocked(db.query.slackProjectConnections.findMany);
const mockSettings = vi.mocked(db.query.slackProjectSettings.findFirst);
const mockWorkspace = vi.mocked(db.query.workspaces.findFirst);
const mockFindUser = vi.mocked(auth.findUser);
const mockListMemberships = vi.mocked(auth.listMemberships);

let ephemerals: string[];

function baseInteraction(overrides: Partial<ApprovalInteraction> = {}): ApprovalInteraction {
  return {
    deploymentId: "dep_1",
    claimedWorkspaceId: "ws_1",
    decision: "approve",
    responseUrl: "https://hooks.slack.test/response",
    slackTeamId: "T_1",
    slackUserId: "U_1",
    slackUsername: "octocat",
    forwardedFor: "1.2.3.4",
    userAgent: "slack",
    ...overrides,
  };
}

// The happy-path lookups every non-refusing test needs.
function seedConnected(policy: "anyone" | "admins_only" = "anyone") {
  mockInstall.mockResolvedValue({ botToken: "cipher", teamId: "T_1" } as never);
  mockDeployment.mockResolvedValue({
    id: "dep_1",
    projectId: "proj_1",
    project: { name: "acme" },
  } as never);
  mockConnections.mockResolvedValue([{ id: "conn_1" }] as never);
  mockSettings.mockResolvedValue({ approvalPolicy: policy } as never);
  insertAuditLogs.mockResolvedValue(undefined);
}

describe("handleApprovalInteraction", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    ephemerals = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (_url: string, init?: RequestInit) => {
        if (init?.body) {
          ephemerals.push(JSON.parse(init.body as string).text);
        }
        return new Response(null, { status: 200 });
      }),
    );
    authorizeDeployment.mockResolvedValue({});
    rejectDeployment.mockResolvedValue({});
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("refuses when no installation binds the payload team to the workspace", async () => {
    mockInstall.mockResolvedValue(undefined as never);

    await handleApprovalInteraction(baseInteraction());

    expect(authorizeDeployment).not.toHaveBeenCalled();
    expect(ephemerals[0]).toContain("not authorized");
  });

  it("refuses when the deployment does not belong to the workspace", async () => {
    mockInstall.mockResolvedValue({ botToken: "cipher", teamId: "T_1" } as never);
    mockDeployment.mockResolvedValue(undefined as never);

    await handleApprovalInteraction(baseInteraction());

    expect(authorizeDeployment).not.toHaveBeenCalled();
    expect(ephemerals[0]).toContain("Deployment not found");
  });

  it("refuses when the project has no connected channels (disconnected)", async () => {
    mockInstall.mockResolvedValue({ botToken: "cipher", teamId: "T_1" } as never);
    mockDeployment.mockResolvedValue({
      id: "dep_1",
      projectId: "proj_1",
      project: { name: "acme" },
    } as never);
    mockConnections.mockResolvedValue([] as never);

    await handleApprovalInteraction(baseInteraction());

    expect(authorizeDeployment).not.toHaveBeenCalled();
    expect(ephemerals[0]).toContain("no longer connected");
  });

  it("approves for anyone policy and attributes the Slack user in the audit log", async () => {
    seedConnected("anyone");

    await handleApprovalInteraction(baseInteraction({ decision: "approve" }));

    expect(authorizeDeployment).toHaveBeenCalledWith({
      deploymentId: "dep_1",
      resolvedBy: "octocat",
    });
    expect(ephemerals.at(-1)).toBe("Deployment approved.");
    const auditActor = insertAuditLogs.mock.calls[0][1].actor;
    expect(auditActor.type).toBe("slack");
  });

  it("rejects for anyone policy", async () => {
    seedConnected("anyone");

    await handleApprovalInteraction(baseInteraction({ decision: "reject" }));

    expect(rejectDeployment).toHaveBeenCalledWith({ deploymentId: "dep_1", resolvedBy: "octocat" });
    expect(authorizeDeployment).not.toHaveBeenCalled();
    expect(ephemerals.at(-1)).toBe("Deployment rejected.");
  });

  it("denies a non-admin under admins_only", async () => {
    seedConnected("admins_only");
    mockWorkspace.mockResolvedValue({ orgId: "org_1" } as never);
    decrypt.mockResolvedValue({ plaintext: "xoxb-token" });
    getUserEmail.mockResolvedValue("dev@acme.test");
    mockFindUser.mockResolvedValue({ id: "user_1" } as never);
    mockListMemberships.mockResolvedValue({
      data: [{ organization: { id: "org_1" }, role: "basic_member" }],
    } as never);

    await handleApprovalInteraction(baseInteraction());

    expect(authorizeDeployment).not.toHaveBeenCalled();
    expect(ephemerals.at(-1)).toContain("Only workspace admins");
  });

  it("authorizes an admin under admins_only and audits them as a user", async () => {
    seedConnected("admins_only");
    mockWorkspace.mockResolvedValue({ orgId: "org_1" } as never);
    decrypt.mockResolvedValue({ plaintext: "xoxb-token" });
    getUserEmail.mockResolvedValue("boss@acme.test");
    mockFindUser.mockResolvedValue({ id: "user_admin" } as never);
    mockListMemberships.mockResolvedValue({
      data: [{ organization: { id: "org_1" }, role: "admin" }],
    } as never);

    await handleApprovalInteraction(baseInteraction());

    expect(authorizeDeployment).toHaveBeenCalledOnce();
    const auditActor = insertAuditLogs.mock.calls[0][1].actor;
    expect(auditActor.type).toBe("user");
    expect(auditActor.id).toBe("user_admin");
  });

  it("denies under admins_only when the clicker is not a member of the org", async () => {
    seedConnected("admins_only");
    mockWorkspace.mockResolvedValue({ orgId: "org_1" } as never);
    decrypt.mockResolvedValue({ plaintext: "xoxb-token" });
    getUserEmail.mockResolvedValue("stranger@evil.test");
    mockFindUser.mockResolvedValue(null as never);
    mockListMemberships.mockResolvedValue({ data: [] } as never);

    await handleApprovalInteraction(baseInteraction());

    expect(authorizeDeployment).not.toHaveBeenCalled();
    expect(mockListMemberships).not.toHaveBeenCalled();
    expect(ephemerals.at(-1)).toContain("Only workspace admins");
  });

  it("reports an already-resolved deployment (ctrl FailedPrecondition)", async () => {
    seedConnected("anyone");
    authorizeDeployment.mockRejectedValue(
      new ConnectError("already resolved", Code.FailedPrecondition),
    );

    await handleApprovalInteraction(baseInteraction());

    expect(ephemerals.at(-1)).toContain("already resolved");
  });
});
