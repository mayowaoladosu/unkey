import { and, db, eq } from "@/lib/db";
import { hasFrameworkDefaults } from "@/lib/deploy/framework-defaults";
import { detectionMatchesGitSource } from "@/lib/deploy/framework-detection-source";
import { TRPCError } from "@trpc/server";
import {
  appBuildSettings,
  appFrameworkDetections,
  apps,
  environments,
  githubRepoConnections,
} from "@unkey/db/src/schema";
import { z } from "zod";
import { workspaceProcedure } from "../../../../trpc";

export const applyFrameworkDefaults = workspaceProcedure
  .input(
    z.object({
      projectId: z.string().min(1),
      appId: z.string().min(1),
      fingerprint: z.string().regex(/^[0-9a-f]{64}$/),
    }),
  )
  .mutation(async ({ ctx, input }) => {
    return await db.transaction(async (tx) => {
      // Lock source identity and provenance before validating them. Repository
      // replacement, branch changes, disconnects, and re-detection must either
      // happen fully before this transaction or wait until after it commits.
      const [app] = await tx
        .select({ id: apps.id, defaultBranch: apps.defaultBranch })
        .from(apps)
        .where(
          and(
            eq(apps.id, input.appId),
            eq(apps.projectId, input.projectId),
            eq(apps.workspaceId, ctx.workspace.id),
          ),
        )
        .for("update");
      if (!app) {
        throw new TRPCError({ code: "NOT_FOUND", message: "App not found" });
      }

      const [detection] = await tx
        .select({
          fingerprint: appFrameworkDetections.fingerprint,
          repositoryFullName: appFrameworkDetections.repositoryFullName,
          branch: appFrameworkDetections.branch,
          defaults: appFrameworkDetections.defaults,
        })
        .from(appFrameworkDetections)
        .where(
          and(
            eq(appFrameworkDetections.appId, input.appId),
            eq(appFrameworkDetections.projectId, input.projectId),
            eq(appFrameworkDetections.workspaceId, ctx.workspace.id),
            eq(appFrameworkDetections.fingerprint, input.fingerprint),
          ),
        )
        .for("update");
      if (!detection) {
        throw new TRPCError({
          code: "PRECONDITION_FAILED",
          message:
            "Framework detection changed. Review the latest result before applying defaults.",
        });
      }

      const [repoConnection] = await tx
        .select({ repositoryFullName: githubRepoConnections.repositoryFullName })
        .from(githubRepoConnections)
        .where(
          and(
            eq(githubRepoConnections.appId, input.appId),
            eq(githubRepoConnections.projectId, input.projectId),
            eq(githubRepoConnections.workspaceId, ctx.workspace.id),
          ),
        )
        .for("update");
      if (!repoConnection || !app.defaultBranch) {
        throw new TRPCError({
          code: "PRECONDITION_FAILED",
          message: "Connect a GitHub repository and select a branch before applying defaults.",
        });
      }
      if (
        !detectionMatchesGitSource(
          {
            repositoryFullName: detection.repositoryFullName,
            branch: detection.branch,
          },
          {
            repositoryFullName: repoConnection.repositoryFullName,
            branch: app.defaultBranch,
          },
        )
      ) {
        throw new TRPCError({
          code: "PRECONDITION_FAILED",
          message:
            "The Git source changed. Run framework detection again before applying defaults.",
        });
      }
      if (!hasFrameworkDefaults(detection.defaults)) {
        throw new TRPCError({
          code: "PRECONDITION_FAILED",
          message: "No unambiguous framework defaults are available.",
        });
      }

      const appEnvironments = await tx
        .select({ id: environments.id })
        .from(environments)
        .where(
          and(
            eq(environments.appId, input.appId),
            eq(environments.projectId, input.projectId),
            eq(environments.workspaceId, ctx.workspace.id),
          ),
        )
        .for("update");
      if (appEnvironments.length === 0) {
        throw new TRPCError({ code: "NOT_FOUND", message: "No app environments found" });
      }

      const now = Date.now();
      const defaults = detection.defaults;

      for (const environment of appEnvironments) {
        // Null means the detector made no safe recommendation. Omit that
        // column so applying a later detection never clears a user override.
        await tx
          .insert(appBuildSettings)
          .values({
            workspaceId: ctx.workspace.id,
            appId: input.appId,
            environmentId: environment.id,
            ...(defaults.rootDirectory !== null ? { dockerContext: defaults.rootDirectory } : {}),
            ...(defaults.dockerfile !== null ? { dockerfile: defaults.dockerfile } : {}),
            ...(defaults.buildCommand !== null ? { buildCommand: defaults.buildCommand } : {}),
            createdAt: now,
            updatedAt: now,
          })
          .onDuplicateKeyUpdate({
            set: {
              ...(defaults.rootDirectory !== null ? { dockerContext: defaults.rootDirectory } : {}),
              ...(defaults.dockerfile !== null ? { dockerfile: defaults.dockerfile } : {}),
              ...(defaults.buildCommand !== null ? { buildCommand: defaults.buildCommand } : {}),
              updatedAt: now,
            },
          });
      }

      await tx
        .update(appFrameworkDetections)
        .set({
          appliedFingerprint: detection.fingerprint,
          appliedDefaults: defaults,
          appliedAt: now,
          updatedAt: now,
        })
        .where(
          and(
            eq(appFrameworkDetections.workspaceId, ctx.workspace.id),
            eq(appFrameworkDetections.appId, input.appId),
            eq(appFrameworkDetections.fingerprint, detection.fingerprint),
          ),
        );
      return { defaults };
    });
  });
