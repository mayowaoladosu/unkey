import { describe, expect, it } from "vitest";
import { resolvePendingRedeployTarget } from "./pending-redeploy-target";

const environments = [
  { id: "env_production", slug: "production" },
  { id: "env_preview", slug: "preview" },
  { id: "env_staging", slug: "staging" },
];

const deployments = [
  { id: "d_staging_latest", environmentId: "env_staging" },
  { id: "d_production", environmentId: "env_production" },
  { id: "d_staging_old", environmentId: "env_staging" },
];

describe("resolvePendingRedeployTarget", () => {
  it("targets the edited custom environment instead of production", () => {
    expect(
      resolvePendingRedeployTarget(["env_staging"], environments, deployments),
    ).toEqual({
      environment: environments[2],
      deployment: deployments[0],
    });
  });

  it("prefers production when a shared settings change touched multiple environments", () => {
    expect(
      resolvePendingRedeployTarget(
        ["env_preview", "env_production"],
        environments,
        deployments,
      )?.environment,
    ).toEqual(environments[0]);
  });

  it("does not fall back to an unrelated deployment", () => {
    expect(
      resolvePendingRedeployTarget(["env_preview"], environments, deployments),
    ).toEqual({
      environment: environments[1],
      deployment: undefined,
    });
  });
});
