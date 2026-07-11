import { describe, expect, it } from "vitest";
import { routes } from "./index";

describe("routes.deploy", () => {
  it("builds the workspace launchpad path", () => {
    expect(routes.deploy.root({ workspaceSlug: "acme" })).toBe("/acme/deploy");
  });
});