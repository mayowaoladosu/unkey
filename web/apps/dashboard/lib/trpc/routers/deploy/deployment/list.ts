import { DEPLOYMENTS_DEFAULT_LIMIT } from "@/lib/collections/deploy/deployments";
import { and, db, desc, eq, gte, lte } from "@/lib/db";
import { workspaceProcedure } from "@/lib/trpc/trpc";
import { TRPCError } from "@trpc/server";
import { deployments } from "@unkey/db/src/schema";
import { z } from "zod";
import {
  deploymentSelectFields,
  enrichDeploymentRows,
  fetchCurrentDeploymentOutsideWindow,
} from "./deployment-query-helpers";

export const listDeployments = workspaceProcedure
  .input(
    z.object({
      projectId: z.string(),
      appId: z.string().optional(),
      startTime: z.number().int().optional(),
      endTime: z.number().int().optional(),
    }),
  )
  .query(async ({ ctx, input }) => {
    try {
      const deploymentRows = await db
        .select({
          ...deploymentSelectFields,
          appId: deployments.appId,
        })
        .from(deployments)
        .where(
          and(
            eq(deployments.workspaceId, ctx.workspace.id),
            eq(deployments.projectId, input.projectId),
            input.appId ? eq(deployments.appId, input.appId) : undefined,
            input.startTime ? gte(deployments.createdAt, input.startTime) : undefined,
            input.endTime ? lte(deployments.createdAt, input.endTime) : undefined,
          ),
        )
        .orderBy(desc(deployments.createdAt), desc(deployments.id))
        .limit(DEPLOYMENTS_DEFAULT_LIMIT);

      if (deploymentRows.length === 0) {
        return [];
      }

      if (
        input.appId !== undefined &&
        input.startTime === undefined &&
        input.endTime === undefined
      ) {
        const currentDeployment = await fetchCurrentDeploymentOutsideWindow(
          ctx.workspace.id,
          { projectId: input.projectId, appId: input.appId },
          deploymentRows,
        );
        if (currentDeployment) {
          deploymentRows.push(currentDeployment);
        }
      }

      return enrichDeploymentRows(ctx.workspace.id, deploymentRows);
    } catch (_error) {
      throw new TRPCError({
        code: "INTERNAL_SERVER_ERROR",
        message: "Failed to fetch deployments",
      });
    }
  });
