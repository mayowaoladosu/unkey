import { and, db, eq } from "@/lib/db";
import { ratelimit, withRatelimit, workspaceProcedure } from "@/lib/trpc/trpc";
import { TRPCError } from "@trpc/server";
import { environments } from "@unkey/db/src/schema";
import { z } from "zod";
import { getCtrlClients } from "../../ctrl";
import { actorFromContext, lifecycleError } from "./shared";

export const setEnvironmentDeleteProtection = workspaceProcedure
  .use(withRatelimit(ratelimit.update))
  .input(z.object({ environmentId: z.string(), enabled: z.boolean() }))
  .mutation(async ({ ctx, input }) => {
    const environment = await db.query.environments.findFirst({
      where: and(
        eq(environments.id, input.environmentId),
        eq(environments.workspaceId, ctx.workspace.id),
      ),
      columns: { id: true },
    });
    if (!environment) {
      throw new TRPCError({ code: "NOT_FOUND", message: "Environment not found" });
    }
    try {
      await getCtrlClients().environment.setEnvironmentDeleteProtection({
        environmentId: input.environmentId,
        enabled: input.enabled,
        actor: actorFromContext(ctx),
      });
    } catch (error) {
      throw lifecycleError(error, "Failed to update delete protection");
    }
  });
