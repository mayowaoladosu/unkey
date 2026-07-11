import { and, db, eq } from "@/lib/db";
import { ratelimit, withRatelimit, workspaceProcedure } from "@/lib/trpc/trpc";
import { TRPCError } from "@trpc/server";
import { environments } from "@unkey/db/src/schema";
import { z } from "zod";
import { getCtrlClients } from "../../ctrl";
import { actorFromContext, environmentSlugSchema, lifecycleError } from "./shared";

export const updateEnvironment = workspaceProcedure
  .use(withRatelimit(ratelimit.update))
  .input(
    z.object({
      environmentId: z.string(),
      slug: environmentSlugSchema,
      description: z.string().trim().max(255).default(""),
    }),
  )
  .mutation(async ({ ctx, input }) => {
    const current = await db.query.environments.findFirst({
      where: and(
        eq(environments.id, input.environmentId),
        eq(environments.workspaceId, ctx.workspace.id),
      ),
      columns: { slug: true },
    });
    if (!current) {
      throw new TRPCError({ code: "NOT_FOUND", message: "Environment not found" });
    }
    if (current.slug === "production" || current.slug === "preview") {
      throw new TRPCError({
        code: "PRECONDITION_FAILED",
        message: "Default environments cannot be renamed",
      });
    }
    try {
      await getCtrlClients().environment.updateEnvironment({
        environmentId: input.environmentId,
        slug: input.slug,
        description: input.description,
        actor: actorFromContext(ctx),
      });
    } catch (error) {
      throw lifecycleError(error, "Failed to update environment");
    }
  });
