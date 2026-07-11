import { and, db, eq } from "@/lib/db";
import { ratelimit, withRatelimit, workspaceProcedure } from "@/lib/trpc/trpc";
import { TRPCError } from "@trpc/server";
import { environments } from "@unkey/db/src/schema";
import { z } from "zod";
import { getCtrlClients } from "../../ctrl";
import { actorFromContext, lifecycleError } from "./shared";

export const deleteEnvironment = workspaceProcedure
  .use(withRatelimit(ratelimit.delete))
  .input(z.object({ environmentId: z.string() }))
  .mutation(async ({ ctx, input }) => {
    const environment = await db.query.environments.findFirst({
      where: and(
        eq(environments.id, input.environmentId),
        eq(environments.workspaceId, ctx.workspace.id),
      ),
      columns: { slug: true, deleteProtection: true },
    });
    if (!environment) {
      throw new TRPCError({ code: "NOT_FOUND", message: "Environment not found" });
    }
    if (environment.slug === "production" || environment.slug === "preview") {
      throw new TRPCError({
        code: "PRECONDITION_FAILED",
        message: "Default environments cannot be deleted",
      });
    }
    if (environment.deleteProtection) {
      throw new TRPCError({
        code: "PRECONDITION_FAILED",
        message: "Disable delete protection before deleting this environment",
      });
    }
    try {
      await getCtrlClients().environment.deleteEnvironment({
        environmentId: input.environmentId,
        actor: actorFromContext(ctx),
      });
    } catch (error) {
      throw lifecycleError(error, "Failed to delete environment");
    }
  });
