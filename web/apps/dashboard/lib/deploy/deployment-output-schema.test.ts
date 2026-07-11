import { describe, expect, it } from "vitest";
import { deploymentOutputsSchema } from "./deployment-output-schema";

describe("deploymentOutputsSchema", () => {
  it("accepts a public service with private worker and cron consumers", () => {
    const parsed = deploymentOutputsSchema.safeParse([
      { kind: "container", name: "api", port: 8080, public: true },
      {
        kind: "worker",
        name: "emails",
        command: ["node", "worker.js"],
        bindings: [{ name: "API", resource: "api", protocol: "http" }],
      },
      { kind: "cron", name: "cleanup", command: ["node", "cleanup.js"], schedule: "0 * * * *" },
    ]);
    expect(parsed.success).toBe(true);
  });

  it("rejects duplicate public outputs and dangling bindings", () => {
    const parsed = deploymentOutputsSchema.safeParse([
      { kind: "container", name: "api", port: 8080, public: true },
      {
        kind: "function",
        name: "admin",
        runtime: "nodejs22",
        handler: "admin.handler",
        public: true,
        bindings: [{ name: "DATABASE", resource: "missing" }],
      },
    ]);
    expect(parsed.success).toBe(false);
    if (!parsed.success) {
      expect(parsed.error.issues.map((issue) => issue.message)).toEqual(
        expect.arrayContaining([
          "Only one resource can be public",
          "Binding DATABASE targets an unknown resource",
        ]),
      );
    }
  });
});
