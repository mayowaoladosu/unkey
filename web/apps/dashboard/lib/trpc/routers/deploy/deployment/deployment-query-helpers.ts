import type { InstanceStatus } from "@/lib/collections/deploy/instance-status";
import { and, db, eq, inArray, sql } from "@/lib/db";
import type { LastExit } from "@/lib/types/deploy";
import {
  appRegionalSettings,
  apps,
  deploymentSteps,
  deployments,
  instances,
  openapiSpecs,
  regions,
} from "@unkey/db/src/schema";
import { type FlagCode, mapRegionToFlag } from "../network/utils";

export const deploymentSelectFields = {
  id: deployments.id,
  projectId: deployments.projectId,
  environmentId: deployments.environmentId,
  gitCommitSha: deployments.gitCommitSha,
  gitBranch: deployments.gitBranch,
  gitCommitMessage: deployments.gitCommitMessage,
  gitCommitAuthorHandle: deployments.gitCommitAuthorHandle,
  gitCommitAuthorAvatarUrl: deployments.gitCommitAuthorAvatarUrl,
  gitCommitTimestamp: deployments.gitCommitTimestamp,
  prNumber: deployments.prNumber,
  forkRepositoryFullName: deployments.forkRepositoryFullName,
  image: deployments.image,
  status: deployments.status,
  desiredState: deployments.desiredState,
  trigger: deployments.trigger,
  triggeredBy: deployments.triggeredBy,
  triggerReason: deployments.triggerReason,
  cpuMillicores: deployments.cpuMillicores,
  memoryMib: deployments.memoryMib,
  storageMib: deployments.storageMib,
  port: deployments.port,
  upstreamProtocol: deployments.upstreamProtocol,
  healthcheck: deployments.healthcheck,
  shutdownSignal: deployments.shutdownSignal,
  createdAt: deployments.createdAt,
  updatedAt: deployments.updatedAt,
} as const;

export function mapInstanceRow(row: {
  id: string;
  regionId: string;
  regionName: string;
  regionPlatform: string;
  status: InstanceStatus;
}) {
  return {
    id: row.id,
    region: { id: row.regionId, name: row.regionName, platform: row.regionPlatform },
    flagCode: mapRegionToFlag(row.regionName),
    status: row.status,
  };
}

export function normalizeDeploymentRow(deployment: {
  gitBranch: string | null;
  prNumber: number | null;
  forkRepositoryFullName: string | null;
  gitCommitAuthorAvatarUrl: string | null;
  gitCommitTimestamp: number | null;
}) {
  return {
    gitBranch: deployment.gitBranch ?? "",
    prNumber: deployment.prNumber ?? null,
    forkRepositoryFullName: deployment.forkRepositoryFullName ?? null,
    gitCommitAuthorAvatarUrl:
      deployment.gitCommitAuthorAvatarUrl ?? "https://github.com/identicons/dummy-user.png",
    gitCommitTimestamp: deployment.gitCommitTimestamp,
  };
}

// The overview resolves the live deployment by id from the collection, so it
// must be present even when older than the newest-N window the list returns.
export async function fetchCurrentDeploymentOutsideWindow(
  workspaceId: string,
  input: { projectId: string; appId: string },
  loadedRows: { id: string }[],
) {
  const [app] = await db
    .select({ currentDeploymentId: apps.currentDeploymentId })
    .from(apps)
    .where(
      and(
        eq(apps.workspaceId, workspaceId),
        eq(apps.projectId, input.projectId),
        eq(apps.id, input.appId),
      ),
    );
  const currentId = app?.currentDeploymentId;
  if (!currentId || loadedRows.some((d) => d.id === currentId)) {
    return null;
  }
  const [deployment] = await db
    .select({ ...deploymentSelectFields, appId: deployments.appId })
    .from(deployments)
    .where(
      and(
        eq(deployments.workspaceId, workspaceId),
        eq(deployments.projectId, input.projectId),
        eq(deployments.id, currentId),
      ),
    );
  return deployment ?? null;
}

type DeploymentRow = {
  id: string;
  appId: string;
  environmentId: string;
  projectId: string;
  gitCommitSha: string | null;
  gitBranch: string | null;
  gitCommitMessage: string | null;
  gitCommitAuthorHandle: string | null;
  gitCommitAuthorAvatarUrl: string | null;
  gitCommitTimestamp: number | null;
  prNumber: number | null;
  forkRepositoryFullName: string | null;
  image: string | null;
  status: (typeof deployments.$inferSelect)["status"];
  desiredState: (typeof deployments.$inferSelect)["desiredState"];
  trigger: (typeof deployments.$inferSelect)["trigger"];
  triggeredBy: string | null;
  triggerReason: string | null;
  cpuMillicores: number;
  memoryMib: number;
  storageMib: number;
  port: number;
  upstreamProtocol: (typeof deployments.$inferSelect)["upstreamProtocol"];
  healthcheck: (typeof deployments.$inferSelect)["healthcheck"];
  shutdownSignal: (typeof deployments.$inferSelect)["shutdownSignal"];
  createdAt: number;
  updatedAt: number | null;
};

type InstanceQueryRow = {
  id: string;
  deploymentId: string;
  regionId: string;
  regionName: string;
  regionPlatform: string;
  status: InstanceStatus;
  containerStatus: {
    restartCount?: number;
    lastTerminationState?: {
      exitCode?: number;
      signal?: number;
      reason?: string;
      finishedAt?: number;
    } | null;
    waiting?: { reason?: string } | null;
  } | null;
};

export async function enrichDeploymentRows(workspaceId: string, deploymentRows: DeploymentRow[]) {
  if (deploymentRows.length === 0) {
    return [];
  }

  const deploymentIds = deploymentRows.map((d) => d.id);
  const appIds = [...new Set(deploymentRows.map((d) => d.appId))];
  const environmentIds = [...new Set(deploymentRows.map((d) => d.environmentId))];

  const [specRows, instanceRows, regionalSettingsRows, stepTimingRows] = await Promise.all([
    db
      .select({ deploymentId: openapiSpecs.deploymentId })
      .from(openapiSpecs)
      .where(inArray(openapiSpecs.deploymentId, deploymentIds)),
    db
      .select({
        id: instances.id,
        deploymentId: instances.deploymentId,
        regionId: regions.id,
        regionName: regions.name,
        regionPlatform: regions.platform,
        status: instances.status,
        containerStatus: instances.containerStatus,
      })
      .from(instances)
      .innerJoin(regions, eq(regions.id, instances.regionId))
      .where(inArray(instances.deploymentId, deploymentIds)),
    db
      .select({
        appId: appRegionalSettings.appId,
        environmentId: appRegionalSettings.environmentId,
        regionId: regions.id,
        regionName: regions.name,
        regionPlatform: regions.platform,
        replicas: appRegionalSettings.replicas,
      })
      .from(appRegionalSettings)
      .innerJoin(regions, eq(regions.id, appRegionalSettings.regionId))
      .where(
        and(
          eq(appRegionalSettings.workspaceId, workspaceId),
          inArray(appRegionalSettings.appId, appIds),
          inArray(appRegionalSettings.environmentId, environmentIds),
        ),
      ),
    db
      .select({
        deploymentId: deploymentSteps.deploymentId,
        maxEndedAt: sql<number | null>`max(${deploymentSteps.endedAt})`,
        openSteps: sql<number>`sum(case when ${deploymentSteps.endedAt} is null then 1 else 0 end)`,
      })
      .from(deploymentSteps)
      .where(inArray(deploymentSteps.deploymentId, deploymentIds))
      .groupBy(deploymentSteps.deploymentId),
  ]);

  const buildEndedAtByDeployment = new Map<string, number | null>();
  for (const row of stepTimingRows) {
    const openSteps = Number(row.openSteps ?? 0);
    const maxEndedAt = row.maxEndedAt == null ? null : Number(row.maxEndedAt);
    buildEndedAtByDeployment.set(row.deploymentId, openSteps > 0 ? null : maxEndedAt);
  }

  const specSet = new Set(specRows.map((s) => s.deploymentId));
  const instancesByDeployment = new Map<string, ReturnType<typeof mapInstanceRow>[]>();
  const lastExitByDeployment = new Map<string, LastExit>();
  for (const row of instanceRows as InstanceQueryRow[]) {
    const entry = mapInstanceRow(row);
    const list = instancesByDeployment.get(row.deploymentId);
    if (list) {
      list.push(entry);
    } else {
      instancesByDeployment.set(row.deploymentId, [entry]);
    }

    const status = row.containerStatus ?? {};
    const term = status.lastTerminationState ?? null;
    const waiting = status.waiting ?? null;
    const candidate: LastExit = {
      restartCount: status.restartCount ?? 0,
      exitCode: term?.exitCode ?? null,
      signal: term?.signal ?? null,
      reason: term?.reason ?? null,
      finishedAt: term?.finishedAt ?? null,
      statusReason: waiting?.reason ?? null,
    };
    if (candidate.reason === null && candidate.statusReason === null) {
      continue;
    }
    const prev = lastExitByDeployment.get(row.deploymentId);
    if (!prev) {
      lastExitByDeployment.set(row.deploymentId, candidate);
      continue;
    }
    const prevTs = prev.finishedAt ?? -1;
    const candTs = candidate.finishedAt ?? -1;
    if (candTs > prevTs || (candTs === prevTs && candidate.restartCount > prev.restartCount)) {
      lastExitByDeployment.set(row.deploymentId, candidate);
    }
  }

  const desiredStateByAppEnv = new Map<
    string,
    {
      desiredInstanceCount: number;
      desiredRegions: {
        region: { id: string; name: string; platform: string };
        flagCode: FlagCode;
      }[];
    }
  >();
  for (const row of regionalSettingsRows) {
    const key = `${row.appId}:${row.environmentId}`;
    const regionEntry = {
      region: { id: row.regionId, name: row.regionName, platform: row.regionPlatform },
      flagCode: mapRegionToFlag(row.regionName),
    };
    const replicaCount = row.replicas;
    const existing = desiredStateByAppEnv.get(key);
    if (existing) {
      existing.desiredInstanceCount += replicaCount;
      existing.desiredRegions.push(regionEntry);
    } else {
      desiredStateByAppEnv.set(key, {
        desiredInstanceCount: replicaCount,
        desiredRegions: [regionEntry],
      });
    }
  }

  return deploymentRows.map(({ appId, ...deployment }) => {
    const desired = desiredStateByAppEnv.get(`${appId}:${deployment.environmentId}`);
    return {
      ...deployment,
      appId,
      ...normalizeDeploymentRow(deployment),
      instances: instancesByDeployment.get(deployment.id) ?? [],
      buildEndedAt: buildEndedAtByDeployment.get(deployment.id) ?? null,
      lastExit: lastExitByDeployment.get(deployment.id) ?? null,
      desiredInstanceCount: desired?.desiredInstanceCount ?? 0,
      desiredRegions: desired?.desiredRegions ?? [],
      hasOpenApiSpec: specSet.has(deployment.id),
    };
  });
}
