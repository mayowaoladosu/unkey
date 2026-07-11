import { and, db, desc, eq } from "@/lib/db";
import { workspaceProcedure } from "@/lib/trpc/trpc";
import { TRPCError } from "@trpc/server";
import {
	deploymentArtifacts,
	deploymentManifests,
	deploymentResources,
	deploymentTargetAssignments,
	deploymentTargets,
	deployments,
	frontlineRoutes,
	instances,
	regions,
} from "@unkey/db/src/schema";
import { z } from "zod";

export const getDeploymentResources = workspaceProcedure
	.input(
		z.object({
			deploymentId: z.string().min(1),
			projectId: z.string().min(1),
		}),
	)
	.query(async ({ input, ctx }) => {
		const [deployment] = await db
			.select({
				id: deployments.id,
				appId: deployments.appId,
				environmentId: deployments.environmentId,
			})
			.from(deployments)
			.where(
				and(
					eq(deployments.id, input.deploymentId),
					eq(deployments.projectId, input.projectId),
					eq(deployments.workspaceId, ctx.workspace.id),
				),
			)
			.limit(1);
		if (!deployment) {
			throw new TRPCError({
				code: "NOT_FOUND",
				message: "Deployment not found",
			});
		}

		const [
			manifestRows,
			resourceRows,
			instanceRows,
			artifactRows,
			aliasRows,
			targetRows,
			historyRows,
		] = await Promise.all([
			db
				.select({
					schemaVersion: deploymentManifests.schemaVersion,
					fingerprint: deploymentManifests.fingerprint,
					adapterId: deploymentManifests.adapterId,
					outputMode: deploymentManifests.outputMode,
					manifest: deploymentManifests.manifest,
					createdAt: deploymentManifests.createdAt,
				})
				.from(deploymentManifests)
				.where(
					and(
						eq(deploymentManifests.deploymentId, deployment.id),
						eq(deploymentManifests.workspaceId, ctx.workspace.id),
					),
				)
				.limit(1),
			db
				.select({
					id: deploymentResources.id,
					name: deploymentResources.name,
					kind: deploymentResources.kind,
					k8sName: deploymentResources.k8sName,
					image: deploymentResources.image,
					command: deploymentResources.command,
					port: deploymentResources.port,
					upstreamProtocol: deploymentResources.upstreamProtocol,
					public: deploymentResources.public,
					schedule: deploymentResources.schedule,
					runtime: deploymentResources.runtime,
					handler: deploymentResources.handler,
					bindings: deploymentResources.bindings,
					allowedCallers: deploymentResources.allowedCallers,
					cpuMillicores: deploymentResources.cpuMillicores,
					memoryMib: deploymentResources.memoryMib,
					storageMib: deploymentResources.storageMib,
					createdAt: deploymentResources.createdAt,
				})
				.from(deploymentResources)
				.where(
					and(
						eq(deploymentResources.deploymentId, deployment.id),
						eq(deploymentResources.workspaceId, ctx.workspace.id),
					),
				)
				.orderBy(deploymentResources.kind, deploymentResources.name),
			db
				.select({
					id: instances.id,
					resourceId: instances.resourceId,
					k8sName: instances.k8sName,
					address: instances.address,
					status: instances.status,
					regionId: instances.regionId,
					regionName: regions.name,
					cpuMillicores: instances.cpuMillicores,
					memoryMib: instances.memoryMib,
					storageMib: instances.storageMib,
					containerStatus: instances.containerStatus,
				})
				.from(instances)
				.innerJoin(regions, eq(regions.id, instances.regionId))
				.where(
					and(
						eq(instances.deploymentId, deployment.id),
						eq(instances.workspaceId, ctx.workspace.id),
					),
				)
				.orderBy(regions.name, instances.k8sName),
			db
				.select({
					id: deploymentArtifacts.id,
					name: deploymentArtifacts.name,
					kind: deploymentArtifacts.kind,
					storageKey: deploymentArtifacts.storageKey,
					digest: deploymentArtifacts.digest,
					sizeBytes: deploymentArtifacts.sizeBytes,
					contentType: deploymentArtifacts.contentType,
					metadata: deploymentArtifacts.metadata,
					createdAt: deploymentArtifacts.createdAt,
				})
				.from(deploymentArtifacts)
				.where(
					and(
						eq(deploymentArtifacts.deploymentId, deployment.id),
						eq(deploymentArtifacts.workspaceId, ctx.workspace.id),
					),
				)
				.orderBy(deploymentArtifacts.kind, deploymentArtifacts.name),
			db
				.select({
					id: frontlineRoutes.id,
					fqdn: frontlineRoutes.fullyQualifiedDomainName,
					sticky: frontlineRoutes.sticky,
					targetId: frontlineRoutes.targetId,
					createdAt: frontlineRoutes.createdAt,
					updatedAt: frontlineRoutes.updatedAt,
				})
				.from(frontlineRoutes)
				.where(
					and(
						eq(frontlineRoutes.deploymentId, deployment.id),
						eq(frontlineRoutes.appId, deployment.appId),
						eq(frontlineRoutes.environmentId, deployment.environmentId),
					),
				)
				.orderBy(
					frontlineRoutes.sticky,
					frontlineRoutes.fullyQualifiedDomainName,
				),
			db
				.select({
					id: deploymentTargets.id,
					kind: deploymentTargets.kind,
					key: deploymentTargets.targetKey,
					deploymentId: deploymentTargets.deploymentId,
					previousDeploymentId: deploymentTargets.previousDeploymentId,
					createdAt: deploymentTargets.createdAt,
					updatedAt: deploymentTargets.updatedAt,
				})
				.from(deploymentTargets)
				.where(
					and(
						eq(deploymentTargets.environmentId, deployment.environmentId),
						eq(deploymentTargets.appId, deployment.appId),
						eq(deploymentTargets.workspaceId, ctx.workspace.id),
					),
				)
				.orderBy(deploymentTargets.kind, deploymentTargets.targetKey),
			db
				.select({
					id: deploymentTargetAssignments.id,
					targetId: deploymentTargetAssignments.targetId,
					targetKind: deploymentTargets.kind,
					targetKey: deploymentTargets.targetKey,
					deploymentId: deploymentTargetAssignments.deploymentId,
					previousDeploymentId:
						deploymentTargetAssignments.previousDeploymentId,
					reason: deploymentTargetAssignments.reason,
					createdAt: deploymentTargetAssignments.createdAt,
				})
				.from(deploymentTargetAssignments)
				.innerJoin(
					deploymentTargets,
					eq(deploymentTargets.id, deploymentTargetAssignments.targetId),
				)
				.where(
					and(
						eq(
							deploymentTargetAssignments.environmentId,
							deployment.environmentId,
						),
						eq(deploymentTargetAssignments.appId, deployment.appId),
						eq(deploymentTargetAssignments.workspaceId, ctx.workspace.id),
					),
				)
				.orderBy(
					desc(deploymentTargetAssignments.createdAt),
					desc(deploymentTargetAssignments.pk),
				)
				.limit(50),
		]);

		return {
			manifest: manifestRows[0] ?? null,
			resources: resourceRows.map((resource) => ({
				...resource,
				instances: instanceRows.filter(
					(instance) => instance.resourceId === resource.id,
				),
			})),
			artifacts: artifactRows,
			aliases: aliasRows.map((alias) => ({
				...alias,
				mutable: alias.targetId !== null,
			})),
			targets: targetRows.map((target) => ({
				...target,
				isCurrent: target.deploymentId === deployment.id,
				isPrevious: target.previousDeploymentId === deployment.id,
			})),
			history: historyRows,
		};
	});
