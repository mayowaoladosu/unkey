import { DeployService, DeploymentTrigger } from "@/gen/proto/ctrl/v1/deployment_pb";
import { insertAuditLogs } from "@/lib/audit";
import { createCtrlClient } from "@/lib/ctrl-client";
import { and, db, eq, schema } from "@/lib/db";
import { ratelimit, withRatelimit, workspaceProcedure } from "@/lib/trpc/trpc";
import { TRPCError } from "@trpc/server";
import { z } from "zod";
import { resolveProductionSource } from "./promotion-source";

export const promoteToProduction = workspaceProcedure
  .use(withRatelimit(ratelimit.update))
  .input(z.object({ deploymentId: z.string().min(1, "Deployment ID is required") }))
  .mutation(async ({ input, ctx }) => {
    const source = await db.query.deployments.findFirst({
      where: (table, { and, eq }) =>
        and(eq(table.id, input.deploymentId), eq(table.workspaceId, ctx.workspace.id)),
      columns: {
        id: true,
        projectId: true,
        appId: true,
        status: true,
        image: true,
        gitCommitSha: true,
        gitBranch: true,
        gitCommitMessage: true,
        gitCommitAuthorHandle: true,
        gitCommitAuthorAvatarUrl: true,
        gitCommitTimestamp: true,
        forkRepositoryFullName: true,
      },
      with: {
        project: { columns: { name: true } },
        environment: { columns: { slug: true } },
      },
    });

    if (!source) {
      throw new TRPCError({ code: "NOT_FOUND", message: "Deployment not found" });
    }
    if (source.status !== "ready") {
      throw new TRPCError({
        code: "PRECONDITION_FAILED",
        message: "Only a ready deployment can be promoted",
      });
    }
    if (!source.environment || source.environment.slug === "production") {
      throw new TRPCError({
        code: "PRECONDITION_FAILED",
        message: "This deployment is already in production",
      });
    }

    const production = await db.query.environments.findFirst({
      where: and(
        eq(schema.environments.workspaceId, ctx.workspace.id),
        eq(schema.environments.projectId, source.projectId),
        eq(schema.environments.appId, source.appId),
        eq(schema.environments.slug, "production"),
      ),
      columns: { id: true },
    });
    if (!production) {
      throw new TRPCError({
        code: "PRECONDITION_FAILED",
        message: "Production environment is not configured",
      });
    }

    const repoConnection = await db.query.githubRepoConnections.findFirst({
      where: (table, { and, eq }) =>
        and(eq(table.appId, source.appId), eq(table.workspaceId, ctx.workspace.id)),
      columns: { appId: true },
    });
    const productionSource = resolveProductionSource(source, repoConnection !== undefined);
    if (!productionSource) {
      throw new TRPCError({
        code: "PRECONDITION_FAILED",
        message: "The deployment has no reusable image or Git source",
      });
    }

    const ctrl = createCtrlClient(DeployService);
    const result = await ctrl
      .createDeployment({
        projectId: source.projectId,
        appId: source.appId,
        environmentSlug: "production",
        trigger: DeploymentTrigger.DASHBOARD,
        triggeredBy: ctx.user.id,
        triggerReason: `Promoted from ${source.id}`,
        ...productionSource,
      })
      .catch((error) => {
        console.error("Preview promotion failed", error);
        throw new TRPCError({
          code: "INTERNAL_SERVER_ERROR",
          message:
            error instanceof Error ? error.message : "Failed to start production deployment",
        });
      });

    await insertAuditLogs(db, {
      workspaceId: ctx.workspace.id,
      actor: { type: "user", id: ctx.user.id },
      event: "deployment.promote",
      description: `Promoted preview ${source.id} to production as ${result.deploymentId}`,
      resources: [
        { type: "deployment", id: source.id, name: source.project.name },
        { type: "deployment", id: result.deploymentId, name: source.project.name },
      ],
      context: ctx.audit,
    });

    return { deploymentId: result.deploymentId };
  });
