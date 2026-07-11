import { describe, expect, it } from "vitest";
import { buildAppLinks, buildDeploymentLinks } from "./leaves";

describe("sidebar navigation leaves", () => {
  it("exposes resource authoring in app navigation", () => {
    const links = buildAppLinks("workspace", "project", "app", ["projects", "project", "apps", "app", "services"], true);
    expect(links.find((link) => link.key === "services")).toMatchObject({
      label: "Services & Resources",
      isActive: true,
      href: "/workspace/projects/project/apps/app/services",
    });
  });

  it("keeps deployment navigation entirely in the sidebar", () => {
    const base = "/workspace/projects/project/apps/app/deployments/deployment";
    const links = buildDeploymentLinks("workspace", "project", "app", "deployment", `${base}/logs`);
    expect(links.map((link) => link.key)).toEqual([
      "deployment-overview",
      "deployment-logs",
      "deployment-resources",
      "deployment-source",
      "deployment-network",
      "all-deployments",
    ]);
    expect(links.find((link) => link.key === "deployment-logs")?.isActive).toBe(true);
    expect(links.find((link) => link.key === "all-deployments")?.separatorAbove).toBe(true);
  });
});
