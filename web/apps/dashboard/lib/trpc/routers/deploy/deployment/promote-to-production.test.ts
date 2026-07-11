import { describe, expect, it } from "vitest";
import { resolveProductionSource } from "./promotion-source";

const gitSource = {
  image: "registry.example.com/app@sha256:abc",
  gitCommitSha: "abc123",
  gitBranch: "feature/preview",
  gitCommitMessage: "Ship preview",
  gitCommitAuthorHandle: "octocat",
  gitCommitAuthorAvatarUrl: "https://example.com/avatar.png",
  gitCommitTimestamp: 1_700_000_000_000,
  forkRepositoryFullName: "contributor/app",
};

describe("resolveProductionSource", () => {
  it("reuses an immutable image while preserving Git metadata", () => {
    expect(resolveProductionSource(gitSource, true)).toEqual({
      dockerImage: "registry.example.com/app@sha256:abc",
      gitCommit: {
        commitSha: "abc123",
        branch: "feature/preview",
        commitMessage: "Ship preview",
        authorHandle: "octocat",
        authorAvatarUrl: "https://example.com/avatar.png",
        timestamp: BigInt(1_700_000_000_000),
        forkRepository: "contributor/app",
      },
    });
  });

  it("rebuilds pinned Git source when no reusable image exists", () => {
    const result = resolveProductionSource({ ...gitSource, image: null }, true);
    expect(result).toMatchObject({
      gitCommit: { commitSha: "abc123", branch: "feature/preview" },
    });
    expect(result).not.toHaveProperty("dockerImage");
  });

  it("promotes image-only deployments without inventing Git metadata", () => {
    expect(
      resolveProductionSource(
        {
          image: "ghcr.io/acme/api:1.0.0",
          gitCommitSha: null,
          gitBranch: null,
          gitCommitMessage: null,
          gitCommitAuthorHandle: null,
          gitCommitAuthorAvatarUrl: null,
          gitCommitTimestamp: null,
          forkRepositoryFullName: null,
        },
        false,
      ),
    ).toEqual({ dockerImage: "ghcr.io/acme/api:1.0.0" });
  });

  it("rejects deployments with no reusable source", () => {
    expect(resolveProductionSource({ ...gitSource, image: null }, false)).toBeNull();
  });
});
