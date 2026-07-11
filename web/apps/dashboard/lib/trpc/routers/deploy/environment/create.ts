import { db } from "@/lib/db";
import { ratelimit, withRatelimit, workspaceProcedure } from "@/lib/trpc/trpc";
import { TRPCError } from "@trpc/server";
import { z } from "zod";
import { getCtrlClients } from "../../ctrl";
import { actorFromContext, environmentSlugSchema, lifecycleError } from "./shared";

export const createEnvironment = workspaceProcedure
  .use(withRatelimit(ratelimit.create))
  .input(
    z.object({
      projectId: z.string(),
      appId: z.string(),
      sourceEnvironmentId: z.string(),
      slug: environmentSlugSchema,
      description: z.string().trim().max(255).default(""),
      deleteProtection: z.boolean().default(true),
    }),
  )
  .mutation(async ({ ctx, input }) => {
    const source = await db.query.environments.findFirst({
      where: (table, { and, eq }) =>
        and(
          eq(table.id, input.sourceEnvironmentId),
          eq(table.workspaceId, ctx.workspace.id),
          eq(table.projectId, input.projectId),
          eq(table.appId, input.appId),
        ),
      columns: { id: true },
    });
    if (!source) {
      throw new TRPCError({ code: "NOT_FOUND", message: "Source environment not found" });
    }

    try {
      const response = await getCtrlClients().environment.createEnvironment({
        workspaceId: ctx.workspace.id,
        projectId: input.projectId,
        appId: input.appId,
        sourceEnvironmentId: input.sourceEnvironmentId,
        slug: input.slug,
        description: input.description,
        deleteProtection: input.deleteProtection,
        actor: actorFromContext(ctx),
      });
      return { id: response.id };
    } catch (error) {
      console.error("Failed to create environment", error);
      throw lifecycleError(error, "Failed to create environment");
    }
  });
