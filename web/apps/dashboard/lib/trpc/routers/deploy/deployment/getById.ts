import { and, db, eq } from "@/lib/db";
import { workspaceProcedure } from "@/lib/trpc/trpc";
import { TRPCError } from "@trpc/server";
import { deployments } from "@unkey/db/src/schema";
import { z } from "zod";
import { deploymentSelectFields, enrichDeploymentRows } from "./deployment-query-helpers";

export const getById = workspaceProcedure
  .input(
    z.object({
      deploymentId: z.string(),
      projectId: z.string(),
    }),
  )
  .query(async ({ input, ctx }) => {
    try {
      const [deployment] = await db
        .select({
          ...deploymentSelectFields,
          appId: deployments.appId,
        })
        .from(deployments)
        .where(
          and(
            eq(deployments.id, input.deploymentId),
            eq(deployments.workspaceId, ctx.workspace.id),
            eq(deployments.projectId, input.projectId),
          ),
        )
        .limit(1);

      if (!deployment) {
        throw new TRPCError({
          code: "NOT_FOUND",
          message: "Deployment not found",
        });
      }

      const [enriched] = await enrichDeploymentRows(ctx.workspace.id, [deployment]);
      if (!enriched) {
        throw new TRPCError({
          code: "NOT_FOUND",
          message: "Deployment not found",
        });
      }

      return enriched;
    } catch (error) {
      if (error instanceof TRPCError) {
        throw error;
      }

      throw new TRPCError({
        code: "INTERNAL_SERVER_ERROR",
        message: "Failed to fetch deployment",
      });
    }
  });
