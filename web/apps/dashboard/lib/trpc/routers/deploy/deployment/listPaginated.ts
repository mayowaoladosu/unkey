import { and, count, db, desc, eq, gte, inArray, lte } from "@/lib/db";
import { workspaceProcedure } from "@/lib/trpc/trpc";
import { TRPCError } from "@trpc/server";
import { deployments, environments } from "@unkey/db/src/schema";
import { z } from "zod";
import { deploymentSelectFields, enrichDeploymentRows } from "./deployment-query-helpers";

const DEFAULT_PAGE_SIZE = 50;
const MAX_PAGE_SIZE = 100;

function buildListWhereConditions(
  workspaceId: string,
  input: {
    projectId: string;
    appId?: string;
    startTime?: number;
    endTime?: number;
    statuses?: string[];
    branches?: string[];
    environmentIds?: string[];
  },
) {
  return and(
    eq(deployments.workspaceId, workspaceId),
    eq(deployments.projectId, input.projectId),
    input.appId ? eq(deployments.appId, input.appId) : undefined,
    input.startTime ? gte(deployments.createdAt, input.startTime) : undefined,
    input.endTime ? lte(deployments.createdAt, input.endTime) : undefined,
    input.statuses && input.statuses.length > 0
      ? inArray(deployments.status, input.statuses as (typeof deployments.$inferSelect)["status"][])
      : undefined,
    input.branches && input.branches.length > 0
      ? inArray(deployments.gitBranch, input.branches)
      : undefined,
    input.environmentIds && input.environmentIds.length > 0
      ? inArray(deployments.environmentId, input.environmentIds)
      : undefined,
  );
}

export const listPaginatedDeployments = workspaceProcedure
  .input(
    z.object({
      projectId: z.string(),
      appId: z.string().optional(),
      startTime: z.number().int().optional(),
      endTime: z.number().int().optional(),
      page: z.number().int().min(1).default(1),
      limit: z.number().int().min(1).max(MAX_PAGE_SIZE).default(DEFAULT_PAGE_SIZE),
      statuses: z.array(z.string()).optional(),
      branches: z.array(z.string()).optional(),
      environmentSlugs: z.array(z.string()).optional(),
    }),
  )
  .query(async ({ ctx, input }) => {
    try {
      const pageSize = Math.min(input.limit, MAX_PAGE_SIZE);
      const page = input.page;

      let environmentIds: string[] | undefined;
      if (input.environmentSlugs && input.environmentSlugs.length > 0 && input.appId) {
        const envRows = await db
          .select({ id: environments.id })
          .from(environments)
          .where(
            and(
              eq(environments.workspaceId, ctx.workspace.id),
              eq(environments.projectId, input.projectId),
              eq(environments.appId, input.appId),
              inArray(environments.slug, input.environmentSlugs),
            ),
          );
        environmentIds = envRows.map((row) => row.id);
        if (environmentIds.length === 0) {
          return { deployments: [], total: 0, page, pageSize, totalPages: 1 };
        }
      }

      const whereConditions = buildListWhereConditions(ctx.workspace.id, {
        projectId: input.projectId,
        appId: input.appId,
        startTime: input.startTime,
        endTime: input.endTime,
        statuses: input.statuses,
        branches: input.branches,
        environmentIds,
      });

      const [totalResult, deploymentRows] = await Promise.all([
        db.select({ count: count() }).from(deployments).where(whereConditions),
        db
          .select({
            ...deploymentSelectFields,
            appId: deployments.appId,
          })
          .from(deployments)
          .where(whereConditions)
          .orderBy(desc(deployments.createdAt), desc(deployments.id))
          .limit(pageSize)
          .offset((page - 1) * pageSize),
      ]);

      const total = totalResult[0]?.count ?? 0;
      const totalPages = Math.max(1, Math.ceil(total / pageSize));

      if (deploymentRows.length === 0) {
        return { deployments: [], total, page, pageSize, totalPages };
      }

      const enriched = await enrichDeploymentRows(ctx.workspace.id, deploymentRows);

      return {
        deployments: enriched,
        total,
        page,
        pageSize,
        totalPages,
      };
    } catch (_error) {
      throw new TRPCError({
        code: "INTERNAL_SERVER_ERROR",
        message: "Failed to fetch deployments",
      });
    }
  });
