import { ActorType } from "@/gen/proto/ctrl/v1/actor_pb";
import { createProjectRequestSchema } from "@/lib/collections/deploy/projects";
import { and, db, eq, schema } from "@/lib/db";
import { getRepositoryById, getRepositoryTree } from "@/lib/github";
import { ratelimit, withRatelimit, workspaceProcedure } from "@/lib/trpc/trpc";
import { TRPCError } from "@trpc/server";
import { z } from "zod";
import { getCtrlClients } from "../../ctrl";

const sourceSchema = z.discriminatedUnion("type", [
  z.object({
    type: z.literal("github"),
    installationId: z.number().int().positive(),
    repositoryId: z.number().int().positive(),
    branch: z.string().trim().min(1).max(255),
  }),
  z.object({
    type: z.literal("image"),
    image: z
      .string()
      .trim()
      .min(1, "An image reference is required")
      .regex(/^\S+$/, "Image reference cannot contain whitespace"),
  }),
]);

export const initializeProjectRequestSchema = createProjectRequestSchema.extend({
  source: sourceSchema,
});

/**
 * Creates the dashboard's internal project + first app destination as one
 * user-facing operation. The source is verified before anything is persisted;
 * a failure after project creation compensates by deleting the project tree.
 */
export const initializeProject = workspaceProcedure
  .input(initializeProjectRequestSchema)
  .use(withRatelimit(ratelimit.create))
  .mutation(async ({ ctx, input }) => {
    const workspaceId = ctx.workspace.id;
    const actor = {
      id: ctx.user.id,
      type: ActorType.USER,
      remoteIp: ctx.audit.location,
      userAgent: ctx.audit.userAgent ?? "",
    };

    const existingProject = await db.query.projects.findFirst({
      where: (table, { and, eq }) =>
        and(eq(table.workspaceId, workspaceId), eq(table.slug, input.slug)),
      columns: { id: true },
    });
    if (existingProject) {
      throw new TRPCError({
        code: "CONFLICT",
        message: `A project with slug "${input.slug}" already exists in this workspace`,
      });
    }

    let verifiedRepository:
      | {
          id: number;
          fullName: string;
          installationId: number;
          branch: string;
        }
      | undefined;

    if (input.source.type === "github") {
      const githubSource = input.source;
      const installation = await db.query.githubAppInstallations.findFirst({
        where: (table, { and, eq }) =>
          and(
            eq(table.workspaceId, workspaceId),
            eq(table.installationId, githubSource.installationId),
          ),
        columns: { pk: true },
      });
      if (!installation) {
        throw new TRPCError({
          code: "NOT_FOUND",
          message: "GitHub installation not found for this workspace",
        });
      }

      const repository = await getRepositoryById(
        githubSource.installationId,
        githubSource.repositoryId,
      ).catch((error) => {
        console.error("Failed to verify GitHub repository", error);
        throw new TRPCError({
          code: "BAD_REQUEST",
          message: "Unable to verify the selected GitHub repository",
        });
      });
      if (!repository) {
        throw new TRPCError({
          code: "NOT_FOUND",
          message: "GitHub repository not found",
        });
      }

      const [owner, repo] = repository.full_name.split("/");
      if (!owner || !repo) {
        throw new TRPCError({
          code: "BAD_REQUEST",
          message: "GitHub returned an invalid repository name",
        });
      }
      await getRepositoryTree(githubSource.installationId, owner, repo, githubSource.branch).catch(
        (error) => {
          console.error("Failed to verify GitHub branch", error);
          throw new TRPCError({
            code: "BAD_REQUEST",
            message: `Branch "${githubSource.branch}" is not available in ${repository.full_name}`,
          });
        },
      );

      verifiedRepository = {
        id: repository.id,
        fullName: repository.full_name,
        installationId: githubSource.installationId,
        branch: githubSource.branch,
      };
    }

    const ctrl = getCtrlClients();
    let projectId: string | null = null;

    try {
      const project = await ctrl.project.createProject({
        workspaceId,
        name: input.name,
        slug: input.slug,
        actor,
      });
      projectId = project.id;
      const createdProjectId = project.id;

      const app = await ctrl.app.createApp({
        workspaceId,
        projectId: createdProjectId,
        name: input.name,
        slug: input.slug,
        actor,
      });

      if (verifiedRepository) {
        await db.transaction(async (tx) => {
          await tx.insert(schema.githubRepoConnections).values({
            workspaceId,
            projectId: createdProjectId,
            appId: app.id,
            installationId: verifiedRepository.installationId,
            repositoryId: verifiedRepository.id,
            repositoryFullName: verifiedRepository.fullName,
            createdAt: Date.now(),
            updatedAt: null,
          });
          await tx
            .update(schema.apps)
            .set({ defaultBranch: verifiedRepository.branch, updatedAt: Date.now() })
            .where(
              and(
                eq(schema.apps.id, app.id),
                eq(schema.apps.projectId, createdProjectId),
                eq(schema.apps.workspaceId, workspaceId),
              ),
            );
        });
      }

      return {
        projectId: createdProjectId,
        appId: app.id,
        repositoryFullName: verifiedRepository?.fullName ?? null,
        source: input.source,
      };
    } catch (error) {
      if (projectId) {
        try {
          await ctrl.project.deleteProject({ projectId, actor });
        } catch (compensationError) {
          console.error("Failed to compensate project initialization", {
            projectId,
            compensationError,
          });
        }
      }

      if (error instanceof TRPCError) {
        throw error;
      }
      console.error("Failed to initialize deploy project", error);
      throw new TRPCError({
        code: "INTERNAL_SERVER_ERROR",
        message: "Failed to prepare this project for deployment",
      });
    }
  });