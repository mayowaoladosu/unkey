import crypto from "node:crypto";
import { describe, expect, it } from "vitest";
import {
  APPROVAL_BLOCK_PREFIX,
  authorizeSlackApproval,
  parseApprovalBlockId,
  verifySignature,
} from "./slack";

const SIGNING_SECRET = "test-signing-secret";

// sign builds a valid Slack signature for a body at a given timestamp, mirroring
// what Slack sends: v0=HMAC_SHA256(secret, "v0:{ts}:{body}").
function sign(secret: string, timestamp: string, body: string): string {
  const base = `v0:${timestamp}:${body}`;
  return `v0=${crypto.createHmac("sha256", secret).update(base).digest("hex")}`;
}

function nowSeconds(): string {
  return Math.floor(Date.now() / 1000).toString();
}

describe("verifySignature", () => {
  it("accepts a valid, fresh signature", () => {
    const ts = nowSeconds();
    const body = "payload=%7B%22type%22%3A%22block_actions%22%7D";
    const sig = sign(SIGNING_SECRET, ts, body);
    expect(verifySignature(SIGNING_SECRET, sig, ts, body)).toBe(true);
  });

  it("rejects a tampered body", () => {
    const ts = nowSeconds();
    const body = "payload=original";
    const sig = sign(SIGNING_SECRET, ts, body);
    expect(verifySignature(SIGNING_SECRET, sig, ts, "payload=tampered")).toBe(false);
  });

  it("rejects a wrong signing secret", () => {
    const ts = nowSeconds();
    const body = "payload=x";
    const sig = sign("attacker-secret", ts, body);
    expect(verifySignature(SIGNING_SECRET, sig, ts, body)).toBe(false);
  });

  it("rejects a stale timestamp (replay guard)", () => {
    const staleTs = (Math.floor(Date.now() / 1000) - 60 * 60).toString(); // 1h old
    const body = "payload=x";
    const sig = sign(SIGNING_SECRET, staleTs, body);
    expect(verifySignature(SIGNING_SECRET, sig, staleTs, body)).toBe(false);
  });

  it("rejects a future timestamp beyond the window", () => {
    const futureTs = (Math.floor(Date.now() / 1000) + 60 * 60).toString();
    const body = "payload=x";
    const sig = sign(SIGNING_SECRET, futureTs, body);
    expect(verifySignature(SIGNING_SECRET, sig, futureTs, body)).toBe(false);
  });

  it("rejects missing signature or timestamp headers", () => {
    const ts = nowSeconds();
    const body = "payload=x";
    const sig = sign(SIGNING_SECRET, ts, body);
    expect(verifySignature(SIGNING_SECRET, null, ts, body)).toBe(false);
    expect(verifySignature(SIGNING_SECRET, sig, null, body)).toBe(false);
  });

  it("rejects a non-numeric timestamp", () => {
    const body = "payload=x";
    const sig = sign(SIGNING_SECRET, "not-a-number", body);
    expect(verifySignature(SIGNING_SECRET, sig, "not-a-number", body)).toBe(false);
  });
});

describe("parseApprovalBlockId", () => {
  it("matches the Go producer's block_id prefix contract", () => {
    // Must equal approvalBlockIDPrefix in svc/ctrl/worker/slackstatus/message.go.
    expect(APPROVAL_BLOCK_PREFIX).toBe("slack_deploy_approval");
  });

  it("parses a well-formed approval block_id", () => {
    expect(parseApprovalBlockId("slack_deploy_approval:dep_1:ws_1")).toEqual({
      deploymentId: "dep_1",
      workspaceId: "ws_1",
    });
  });

  it("rejects a foreign or malformed block_id", () => {
    expect(parseApprovalBlockId("other:dep_1:ws_1")).toBeNull();
    expect(parseApprovalBlockId("slack_deploy_approval:dep_1")).toBeNull();
    expect(parseApprovalBlockId("slack_deploy_approval::ws_1")).toBeNull();
    expect(parseApprovalBlockId("slack_deploy_approval:dep_1:")).toBeNull();
  });
});

describe("authorizeSlackApproval (R10 / AE3)", () => {
  it("allows anyone under the open policy", () => {
    expect(authorizeSlackApproval("anyone", undefined)).toBe(true);
    expect(authorizeSlackApproval("anyone", { role: "basic_member" })).toBe(true);
  });

  it("allows only workspace admins under admins_only", () => {
    expect(authorizeSlackApproval("admins_only", { role: "admin" })).toBe(true);
    expect(authorizeSlackApproval("admins_only", { role: "basic_member" })).toBe(false);
    expect(authorizeSlackApproval("admins_only", undefined)).toBe(false);
    expect(authorizeSlackApproval("admins_only", null)).toBe(false);
  });
});
