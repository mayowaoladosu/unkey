import crypto from "node:crypto";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

let privateKeyPem = "";

beforeAll(() => {
  const { privateKey } = crypto.generateKeyPairSync("rsa", { modulusLength: 2048 });
  privateKeyPem = privateKey.export({ type: "pkcs8", format: "pem" }).toString();
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.resetModules();
  vi.clearAllMocks();
});

describe("getRepositoryFileContent", () => {
  it("decodes a bounded base64 file through installation authentication", async () => {
    vi.doMock("@/lib/env", () => ({
      githubAppEnv: () => ({
        GITHUB_APP_ID: "123",
        UNKEY_GITHUB_PRIVATE_KEY_PEM: privateKeyPem,
      }),
      githubOAuthEnv: () => null,
    }));
    const fetchMock = vi
      .fn<Parameters<typeof fetch>, ReturnType<typeof fetch>>()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ token: "installation-token", expires_at: "2099-01-01" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            type: "file",
            encoding: "base64",
            content: Buffer.from('{"name":"demo"}').toString("base64"),
            size: 15,
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      );
    vi.stubGlobal("fetch", fetchMock);

    const { getRepositoryFileContent } = await import("./github");
    const content = await getRepositoryFileContent(
      42,
      "acme",
      "demo",
      "feature/test",
      "package.json",
    );

    expect(content).toBe('{"name":"demo"}');
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "https://api.github.com/repos/acme/demo/contents/package.json?ref=feature%2Ftest",
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: "Bearer installation-token" }),
      }),
    );
  });

  it("returns null when the repository file does not exist", async () => {
    vi.doMock("@/lib/env", () => ({
      githubAppEnv: () => ({
        GITHUB_APP_ID: "123",
        UNKEY_GITHUB_PRIVATE_KEY_PEM: privateKeyPem,
      }),
      githubOAuthEnv: () => null,
    }));
    vi.stubGlobal(
      "fetch",
      vi
        .fn<Parameters<typeof fetch>, ReturnType<typeof fetch>>()
        .mockResolvedValueOnce(
          new Response(JSON.stringify({ token: "installation-token", expires_at: "2099-01-01" }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        )
        .mockResolvedValueOnce(new Response("missing", { status: 404 })),
    );

    const { getRepositoryFileContent } = await import("./github");

    await expect(
      getRepositoryFileContent(42, "acme", "demo", "main", "package.json"),
    ).resolves.toBeNull();
  });
});

describe("getRepositoryBlobContent", () => {
  it("reads the exact immutable blob selected from the repository tree", async () => {
    vi.doMock("@/lib/env", () => ({
      githubAppEnv: () => ({
        GITHUB_APP_ID: "123",
        UNKEY_GITHUB_PRIVATE_KEY_PEM: privateKeyPem,
      }),
      githubOAuthEnv: () => null,
    }));
    const fetchMock = vi
      .fn<Parameters<typeof fetch>, ReturnType<typeof fetch>>()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ token: "installation-token", expires_at: "2099-01-01" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            encoding: "base64",
            content: Buffer.from('{"name":"immutable"}').toString("base64"),
            size: 20,
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      );
    vi.stubGlobal("fetch", fetchMock);

    const { getRepositoryBlobContent } = await import("./github");

    await expect(getRepositoryBlobContent(42, "acme", "demo", "blob-abc123")).resolves.toBe(
      '{"name":"immutable"}',
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "https://api.github.com/repos/acme/demo/git/blobs/blob-abc123",
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: "Bearer installation-token" }),
      }),
    );
  });
});

describe("getRepositoryTree", () => {
  it("returns the immutable tree SHA and encodes branch names", async () => {
    vi.doMock("@/lib/env", () => ({
      githubAppEnv: () => ({
        GITHUB_APP_ID: "123",
        UNKEY_GITHUB_PRIVATE_KEY_PEM: privateKeyPem,
      }),
      githubOAuthEnv: () => null,
    }));
    const fetchMock = vi
      .fn<Parameters<typeof fetch>, ReturnType<typeof fetch>>()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ token: "installation-token", expires_at: "2099-01-01" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            sha: "abc123",
            tree: [{ path: "package.json", type: "blob", sha: "blob-abc123" }],
            truncated: false,
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      );
    vi.stubGlobal("fetch", fetchMock);

    const { getRepositoryTree } = await import("./github");

    await expect(getRepositoryTree(42, "acme", "demo", "feature/test")).resolves.toEqual({
      sha: "abc123",
      tree: [{ path: "package.json", type: "blob", sha: "blob-abc123" }],
      truncated: false,
    });
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "https://api.github.com/repos/acme/demo/git/trees/feature%2Ftest?recursive=1",
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: "Bearer installation-token" }),
      }),
    );
  });
});
