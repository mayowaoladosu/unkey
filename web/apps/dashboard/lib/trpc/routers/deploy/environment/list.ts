import { and, db, desc, eq, inArray, sql } from "@/lib/db";
import { ratelimit, withRatelimit, workspaceProcedure } from "@/lib/trpc/trpc";
import { deployments, environments } from "@unkey/db/src/schema";
import { z } from "zod";

export const listManagedEnvironments = workspaceProcedure
  .use(withRatelimit(ratelimit.read))
  .input(z.object({ projectId: z.string(), appId: z.string() }))
  .query(async ({ ctx, input }) => {
    const rows = await db.query.environments.findMany({
      where: and(
        eq(environments.workspaceId, ctx.workspace.id),
        eq(environments.projectId, input.projectId),
        eq(environments.appId, input.appId),
      ),
      orderBy: [desc(environments.createdAt)],
    });
    if (rows.length === 0) {
      return [];
    }

    const ids = rows.map((row) => row.id);
    const deploymentCounts = await db
      .select({ environmentId: deployments.environmentId, count: sql<number>`count(*)` })
      .from(deployments)
      .where(
        and(
          eq(deployments.workspaceId, ctx.workspace.id),
          eq(deployments.appId, input.appId),
          inArray(deployments.environmentId, ids),
        ),
      )
      .groupBy(deployments.environmentId);
    const countByEnvironment = new Map(
      deploymentCounts.map((row) => [row.environmentId, Number(row.count)]),
    );

    return rows.map((row) => ({
      id: row.id,
      slug: row.slug,
      description: row.description,
      deleteProtection: row.deleteProtection ?? false,
      createdAt: row.createdAt,
      deploymentCount: countByEnvironment.get(row.id) ?? 0,
      isDefault: row.slug === "production" || row.slug === "preview",
    }));
  });
