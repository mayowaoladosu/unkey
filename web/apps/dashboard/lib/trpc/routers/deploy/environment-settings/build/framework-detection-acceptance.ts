import { and, db, eq } from "@/lib/db";
import { canAcceptDetectedOutput, hasFrameworkDefaults } from "@/lib/deploy/framework-defaults";
import { detectionMatchesGitSource } from "@/lib/deploy/framework-detection-source";
import { TRPCError } from "@trpc/server";
import {
  appBuildSettings,
  appFrameworkDetections,
  apps,
  environments,
  githubRepoConnections,
} from "@unkey/db/src/schema";

type AcceptanceInput = {
  workspaceId: string;
  projectId: string;
  appId: string;
  fingerprint: string;
  mode: "defaults" | "output";
};

export async function persistFrameworkDetectionAcceptance(input: AcceptanceInput) {
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
          eq(apps.workspaceId, input.workspaceId),
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
        detection: appFrameworkDetections.detection,
        defaults: appFrameworkDetections.defaults,
      })
      .from(appFrameworkDetections)
      .where(
        and(
          eq(appFrameworkDetections.appId, input.appId),
          eq(appFrameworkDetections.projectId, input.projectId),
          eq(appFrameworkDetections.workspaceId, input.workspaceId),
          eq(appFrameworkDetections.fingerprint, input.fingerprint),
        ),
      )
      .for("update");
    if (!detection) {
      throw new TRPCError({
        code: "PRECONDITION_FAILED",
        message: "Framework detection changed. Review the latest result before accepting it.",
      });
    }

    const [repoConnection] = await tx
      .select({ repositoryFullName: githubRepoConnections.repositoryFullName })
      .from(githubRepoConnections)
      .where(
        and(
          eq(githubRepoConnections.appId, input.appId),
          eq(githubRepoConnections.projectId, input.projectId),
          eq(githubRepoConnections.workspaceId, input.workspaceId),
        ),
      )
      .for("update");
    if (!repoConnection || !app.defaultBranch) {
      throw new TRPCError({
        code: "PRECONDITION_FAILED",
        message: "Connect a GitHub repository and select a branch before accepting detection.",
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
        message: "The Git source changed. Run framework detection again before accepting it.",
      });
    }

    const settingsUpdated = input.mode === "defaults";
    if (settingsUpdated && !hasFrameworkDefaults(detection.defaults)) {
      throw new TRPCError({
        code: "PRECONDITION_FAILED",
        message: "No unambiguous framework defaults are available.",
      });
    }
    if (
      input.mode === "output" &&
      !canAcceptDetectedOutput(detection.detection, detection.defaults)
    ) {
      throw new TRPCError({
        code: "PRECONDITION_FAILED",
        message: "No unambiguous static output is available to accept.",
      });
    }

    const now = Date.now();
    if (settingsUpdated) {
      const appEnvironments = await tx
        .select({ id: environments.id })
        .from(environments)
        .where(
          and(
            eq(environments.appId, input.appId),
            eq(environments.projectId, input.projectId),
            eq(environments.workspaceId, input.workspaceId),
          ),
        )
        .for("update");
      if (appEnvironments.length === 0) {
        throw new TRPCError({ code: "NOT_FOUND", message: "No app environments found" });
      }

      for (const environment of appEnvironments) {
        const defaults = detection.defaults;
        // Null means the detector made no safe recommendation. Omit that
        // column so applying a later detection never clears a user override.
        await tx
          .insert(appBuildSettings)
          .values({
            workspaceId: input.workspaceId,
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
    }

    await tx
      .update(appFrameworkDetections)
      .set({
        appliedFingerprint: detection.fingerprint,
        appliedDefaults: detection.defaults,
        appliedAt: now,
        updatedAt: now,
      })
      .where(
        and(
          eq(appFrameworkDetections.workspaceId, input.workspaceId),
          eq(appFrameworkDetections.appId, input.appId),
          eq(appFrameworkDetections.fingerprint, detection.fingerprint),
        ),
      );

    return { defaults: detection.defaults, settingsUpdated };
  });
}
