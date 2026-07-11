import { Ok } from "@unkey/error";
import { describe, expect, it, vi } from "vitest";
import { getBuildStepLogs } from "./build-steps";
import type { Querier } from "./client";

describe("getBuildStepLogs", () => {
  it("pages one build step newest-first with a stable offset", async () => {
    const execute = vi.fn(async () =>
      Ok([
        { time: 30, step_id: "sha256:step", message: "third" },
        { time: 20, step_id: "sha256:step", message: "second" },
      ]),
    );
    const query = vi.fn(() => execute);
    const getLogs = getBuildStepLogs({ query } as unknown as Querier);

    const result = await getLogs({
      workspaceId: "ws_test",
      projectId: "proj_test",
      deploymentId: "d_test",
      stepId: "sha256:step",
      cursor: 200,
      limit: 201,
    });

    expect(result.err).toBeUndefined();
    expect(query).toHaveBeenCalledOnce();
    const request = query.mock.calls[0]?.[0];
    expect(request?.query).toContain("step_id = {stepId: String}");
    expect(request?.query).toContain("ORDER BY time DESC, message DESC");
    expect(request?.query).toContain("OFFSET {cursor: Int}");
    expect(execute).toHaveBeenCalledWith({
      workspaceId: "ws_test",
      projectId: "proj_test",
      deploymentId: "d_test",
      stepId: "sha256:step",
      cursor: 200,
      limit: 201,
    });
  });
});
