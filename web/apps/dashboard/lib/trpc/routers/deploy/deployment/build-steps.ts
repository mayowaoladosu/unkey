import { clickhouse } from "@/lib/clickhouse";
import { db } from "@/lib/db";
import { ratelimit, withRatelimit, workspaceProcedure } from "@/lib/trpc/trpc";
import { TRPCError } from "@trpc/server";
import { buildStepLogSchema, buildStepSchema } from "@unkey/clickhouse/src/build-steps";
import { z } from "zod";

const MAX_LOG_PAGE_SIZE = 500;
const buildStepOutputSchema = buildStepSchema.omit({ error: true }).extend({
  error: z.string().nullable(),
});

export const getDeploymentBuildSteps = workspaceProcedure
  .use(withRatelimit(ratelimit.read))
  .input(
    z.object({
      deploymentId: z.string(),
    }),
  )
  .output(
    z.object({
      steps: z.array(buildStepOutputSchema),
    }),
  )
  .query(async ({ ctx, input }) => {
    // Validate deployment exists and belongs to workspace
    const deployment = await db.query.deployments.findFirst({
      where: (table, { and, eq }) =>
        and(eq(table.id, input.deploymentId), eq(table.workspaceId, ctx.workspace.id)),
    });
    if (!deployment) {
      throw new TRPCError({
        code: "NOT_FOUND",
        message: "Deployment not found",
      });
    }

    // Fetch steps from ClickHouse
    const stepsResult = await clickhouse.buildSteps.getSteps({
      workspaceId: deployment.workspaceId,
      projectId: deployment.projectId,
      deploymentId: input.deploymentId,
    });

    if (stepsResult.err) {
      throw new TRPCError({
        code: "INTERNAL_SERVER_ERROR",
        message: "Failed to fetch build steps",
      });
    }

    return { steps: stepsResult.val };
  });

export const getDeploymentBuildStepLogs = workspaceProcedure
  .use(withRatelimit(ratelimit.read))
  .input(
    z.object({
      deploymentId: z.string(),
      stepId: z.string().min(1),
      cursor: z.number().int().nonnegative().default(0),
      limit: z.number().int().min(1).max(MAX_LOG_PAGE_SIZE).default(200),
    }),
  )
  .output(
    z.object({
      // Rows are newest-first so page offsets remain stable once the step has
      // completed. The client reverses loaded pages for terminal-style output.
      logs: z.array(buildStepLogSchema.pick({ time: true, message: true })),
      nextCursor: z.number().int().nonnegative().nullable(),
    }),
  )
  .query(async ({ ctx, input }) => {
    const deployment = await db.query.deployments.findFirst({
      where: (table, { and, eq }) =>
        and(eq(table.id, input.deploymentId), eq(table.workspaceId, ctx.workspace.id)),
    });
    if (!deployment) {
      throw new TRPCError({
        code: "NOT_FOUND",
        message: "Deployment not found",
      });
    }

    const result = await clickhouse.buildSteps.getLogs({
      workspaceId: deployment.workspaceId,
      projectId: deployment.projectId,
      deploymentId: deployment.id,
      stepId: input.stepId,
      cursor: input.cursor,
      limit: input.limit + 1,
    });

    if (result.err) {
      throw new TRPCError({
        code: "INTERNAL_SERVER_ERROR",
        message: "Failed to fetch build step logs",
      });
    }

    const hasMore = result.val.length > input.limit;
    return {
      logs: result.val.slice(0, input.limit).map(({ time, message }) => ({ time, message })),
      nextCursor: hasMore ? input.cursor + input.limit : null,
    };
  });
