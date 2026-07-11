import { deploymentOutputsSchema } from "@/lib/deploy/deployment-output-schema";
import { and, db, eq } from "@/lib/db";
import { TRPCError } from "@trpc/server";
import { appRuntimeSettings, environments } from "@unkey/db/src/schema";
import { z } from "zod";
import { workspaceProcedure } from "../../../../trpc";

export const updateOutputs = workspaceProcedure
  .input(
    z.object({
      environmentId: z.string(),
      outputs: deploymentOutputsSchema,
    }),
  )
  .mutation(async ({ ctx, input }) => {
    const env = await db.query.environments.findFirst({
      where: and(
        eq(environments.id, input.environmentId),
        eq(environments.workspaceId, ctx.workspace.id),
      ),
      columns: { appId: true },
    });
    if (!env) {
      throw new TRPCError({ code: "NOT_FOUND", message: "Environment not found" });
    }

    const now = Date.now();
    await db
      .insert(appRuntimeSettings)
      .values({
        workspaceId: ctx.workspace.id,
        appId: env.appId,
        environmentId: input.environmentId,
        outputs: input.outputs,
        sentinelConfig: "{}",
        createdAt: now,
        updatedAt: now,
      })
      .onDuplicateKeyUpdate({
        set: { outputs: input.outputs, updatedAt: now },
      });
  });
