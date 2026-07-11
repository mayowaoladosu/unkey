"use client";

import { shortenId } from "@/lib/shorten-id";
import { trpc } from "@/lib/trpc/client";
import { Cube, Earth, Layers2 } from "@unkey/icons";
import { Button, CopyButton, DialogContainer, toast } from "@unkey/ui";
import { type ReactNode, useState } from "react";
import { Section, SectionHeader } from "../../../../../../components/section";
import { Card } from "../../../../../components/card";
import { useProjectData } from "../../../../../data-provider";
import { useDeployment } from "../../../layout-provider";

export function DeploymentResourcesSection() {
	const { deployment } = useDeployment();
	const { refetchAll } = useProjectData();
	const [rollbackOpen, setRollbackOpen] = useState(false);
	const [selectedResourceId, setSelectedResourceId] = useState<string | null>(
		null,
	);
	const resources = trpc.deploy.deployment.resources.useQuery({
		deploymentId: deployment.id,
		projectId: deployment.projectId,
	});
	const rollbackTarget = resources.data?.targets.find(
		(target) =>
			target.kind === "live" && target.isCurrent && target.previousDeploymentId,
	);
	const rollback = trpc.deploy.deployment.rollback.useMutation({
		onSuccess: async () => {
			await resources.refetch();
			refetchAll();
			setRollbackOpen(false);
			toast.success("Rollback completed", {
				description: `Traffic now points to ${shortenId(rollbackTarget?.previousDeploymentId ?? "")}.`,
			});
		},
		onError: (error) => {
			toast.error("Rollback failed", { description: error.message });
		},
	});

	return (
		<Section>
			<SectionHeader
				icon={<Layers2 iconSize="md-regular" className="text-gray-9" />}
				title="Resources & targets"
			/>
			<Card className="overflow-hidden">
				{resources.isPending ? (
					<div className="grid gap-3 p-4">
						<div className="h-4 w-52 animate-pulse rounded bg-grayA-3" />
						<div className="h-16 animate-pulse rounded-md bg-grayA-2" />
					</div>
				) : resources.isError || !resources.data ? (
					<div className="p-4 text-xs text-error-11">
						Deployment resources could not be loaded. The deployment remains
						available.
					</div>
				) : (
					<div className="divide-y divide-gray-4">
						<ResourceBlock
							icon={<Cube iconSize="sm-medium" />}
							title="Immutable manifest"
							empty="No manifest was recorded for this historical deployment."
							hasContent={resources.data.manifest !== null}
						>
							{resources.data.manifest ? (
								<div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
									<ResourceValue
										label="Adapter"
										value={resources.data.manifest.adapterId}
									/>
									<ResourceValue
										label="Output"
										value={resources.data.manifest.outputMode}
									/>
									<ResourceValue
										label="Schema"
										value={`v${resources.data.manifest.schemaVersion}`}
									/>
									<ResourceValue
										label="Fingerprint"
										value={shortenDigest(resources.data.manifest.fingerprint)}
										copyValue={resources.data.manifest.fingerprint}
									/>
								</div>
							) : null}
						</ResourceBlock>

						<ResourceBlock
							icon={<Layers2 iconSize="sm-medium" />}
							title={`Runtime resources (${resources.data.resources.length})`}
							empty="No materialized runtime resources were recorded."
							hasContent={resources.data.resources.length > 0}
						>
							<div className="grid gap-2 lg:grid-cols-2">
								{resources.data.resources.map((resource) => {
									const running = resource.instances.filter(
										(instance) => instance.status === "running",
									).length;
									return (
										<button
											type="button"
											key={resource.id}
											onClick={() =>
												setSelectedResourceId((current) =>
													current === resource.id ? null : resource.id,
												)
											}
											className={`rounded-md border p-3 text-left transition-colors ${
												selectedResourceId === resource.id
													? "border-accent-8 bg-accentA-2"
													: "border-grayA-4 bg-grayA-2 hover:border-grayA-6"
											}`}
										>
											<div className="flex items-start justify-between gap-3">
												<div className="min-w-0">
													<div className="flex flex-wrap items-center gap-1.5">
														<p className="truncate text-xs font-medium text-gray-12">
															{resource.name}
														</p>
														<ResourceBadge>{resource.kind}</ResourceBadge>
														{resource.public ? (
															<ResourceBadge accent>Public</ResourceBadge>
														) : null}
													</div>
													<p className="mt-1 truncate font-mono text-[10px] text-gray-9">
														{resource.k8sName ?? resource.id}
													</p>
												</div>
												<span className="shrink-0 text-[11px] text-gray-10">
													{resource.kind === "static"
														? "edge"
														: `${running}/${resource.instances.length} running`}
												</span>
											</div>
											<div className="mt-3 grid grid-cols-3 gap-2 text-[10px] text-gray-10">
												<span>{resource.cpuMillicores}m CPU</span>
												<span>{resource.memoryMib} MiB</span>
												<span>
													{resource.port > 0 ? `:${resource.port}` : "no port"}
												</span>
											</div>
										</button>
									);
								})}
							</div>
							{selectedResourceId ? (
								<ResourceObservability
									deploymentId={deployment.id}
									projectId={deployment.projectId}
									resource={resources.data.resources.find(
										(resource) => resource.id === selectedResourceId,
									)}
								/>
							) : null}
						</ResourceBlock>

						<ResourceBlock
							icon={<Cube iconSize="sm-medium" />}
							title={`Artifacts (${resources.data.artifacts.length})`}
							empty="No materialized artifacts were recorded."
							hasContent={resources.data.artifacts.length > 0}
						>
							<div className="grid gap-2">
								{resources.data.artifacts.map((artifact) => (
									<div
										key={artifact.id}
										className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-grayA-4 bg-grayA-2 px-3 py-2"
									>
										<div className="min-w-0">
											<p className="text-xs font-medium text-gray-12">
												{artifact.name}
											</p>
											<p className="truncate text-[11px] text-gray-10">
												{artifact.kind.replaceAll("_", " ")} ·{" "}
												{formatBytes(artifact.sizeBytes)} ·{" "}
												{artifact.contentType}
											</p>
										</div>
										<div className="flex items-center gap-1 font-mono text-[11px] text-gray-10">
											{shortenDigest(artifact.digest)}
											<CopyButton
												value={artifact.digest}
												variant="ghost"
												toastMessage="Artifact digest"
											/>
										</div>
									</div>
								))}
							</div>
						</ResourceBlock>

						<ResourceBlock
							icon={<Earth iconSize="sm-medium" />}
							title={`Aliases (${resources.data.aliases.length})`}
							empty="No aliases currently point to this deployment."
							hasContent={resources.data.aliases.length > 0}
						>
							<div className="grid gap-2">
								{resources.data.aliases.map((alias) => (
									<div
										key={alias.id}
										className="flex min-w-0 items-center justify-between gap-3 rounded-md border border-grayA-4 px-3 py-2"
									>
										<a
											href={`https://${alias.fqdn}`}
											target="_blank"
											rel="noreferrer"
											className="truncate text-xs font-medium text-gray-12 hover:underline"
										>
											{alias.fqdn}
										</a>
										<ResourceBadge>
											{alias.mutable ? alias.sticky : "immutable"}
										</ResourceBadge>
									</div>
								))}
							</div>
						</ResourceBlock>

						<ResourceBlock
							icon={<Layers2 iconSize="sm-medium" />}
							title={`Environment targets (${resources.data.targets.length})`}
							empty="No mutable targets have been assigned yet."
							hasContent={resources.data.targets.length > 0}
						>
							{rollbackTarget?.previousDeploymentId ? (
								<div className="mb-3 flex flex-wrap items-center justify-between gap-3 rounded-md border border-warningA-5 bg-warningA-2 px-3 py-2">
									<div>
										<p className="text-xs font-medium text-gray-12">
											Instant rollback available
										</p>
										<p className="mt-0.5 text-[11px] text-gray-10">
											Restore {shortenId(rollbackTarget.previousDeploymentId)}{" "}
											without rebuilding.
										</p>
									</div>
									<Button
										type="button"
										variant="outline"
										onClick={() => setRollbackOpen(true)}
									>
										Roll back
									</Button>
								</div>
							) : null}
							<div className="grid gap-2 sm:grid-cols-2">
								{resources.data.targets.map((target) => (
									<div
										key={target.id}
										className="rounded-md border border-grayA-4 px-3 py-2"
									>
										<div className="flex items-center justify-between gap-2">
											<p className="truncate text-xs font-medium text-gray-12">
												{target.kind}: {target.key}
											</p>
											{target.isCurrent ? (
												<ResourceBadge accent>Current</ResourceBadge>
											) : target.isPrevious ? (
												<ResourceBadge>Previous</ResourceBadge>
											) : null}
										</div>
										<p className="mt-1 font-mono text-[11px] text-gray-10">
											{shortenId(target.deploymentId)}
										</p>
									</div>
								))}
							</div>
							{resources.data.history.length > 0 ? (
								<div className="mt-3 border-t border-gray-4 pt-3">
									<p className="mb-2 text-[11px] font-medium uppercase tracking-wide text-gray-9">
										Recent assignments
									</p>
									<div className="grid gap-1.5">
										{resources.data.history.slice(0, 5).map((assignment) => (
											<div
												key={assignment.id}
												className="flex flex-wrap items-center justify-between gap-2 text-[11px]"
											>
												<span className="text-gray-11">
													{assignment.targetKind}:{assignment.targetKey} ·{" "}
													{assignment.reason}
												</span>
												<span className="font-mono text-gray-9">
													{shortenId(assignment.deploymentId)} ·{" "}
													{new Date(assignment.createdAt).toLocaleString()}
												</span>
											</div>
										))}
									</div>
								</div>
							) : null}
						</ResourceBlock>
					</div>
				)}
			</Card>
			<DialogContainer
				isOpen={rollbackOpen}
				onOpenChange={setRollbackOpen}
				title="Roll back live traffic"
				subTitle="Move the live and environment aliases to the retained deployment. No source rebuild will run."
				footer={
					<Button
						type="button"
						variant="primary"
						size="xlg"
						className="w-full rounded-lg"
						disabled={
							!rollbackTarget?.previousDeploymentId || rollback.isPending
						}
						loading={rollback.isPending}
						onClick={() => {
							if (rollbackTarget?.previousDeploymentId) {
								rollback.mutate({
									targetDeploymentId: rollbackTarget.previousDeploymentId,
								});
							}
						}}
					>
						Roll back to{" "}
						{shortenId(rollbackTarget?.previousDeploymentId ?? "deployment")}
					</Button>
				}
			>
				<div className="grid gap-3 rounded-lg border border-grayA-4 bg-grayA-2 p-4 text-xs">
					<div className="flex items-center justify-between gap-3">
						<span className="text-gray-9">Current</span>
						<span className="font-mono text-gray-12">
							{shortenId(deployment.id)}
						</span>
					</div>
					<div className="flex items-center justify-between gap-3">
						<span className="text-gray-9">Restore</span>
						<span className="font-mono text-gray-12">
							{shortenId(rollbackTarget?.previousDeploymentId ?? "")}
						</span>
					</div>
				</div>
			</DialogContainer>
		</Section>
	);
}

type DeploymentResource = {
	id: string;
	name: string;
	kind: "service" | "function" | "worker" | "cron" | "static";
	k8sName: string | null;
	image: string | null;
	command: string[];
	port: number;
	public: boolean;
	schedule: string | null;
	runtime: string | null;
	handler: string | null;
	bindings: Array<{
		name: string;
		resourceId: string;
		resourceName: string;
		protocol: "http" | "tcp";
		host: string;
		port: number;
	}>;
	instances: Array<{
		id: string;
		k8sName: string;
		status: "inactive" | "pending" | "running" | "failed";
		regionName: string;
	}>;
};

function ResourceObservability({
	deploymentId,
	projectId,
	resource,
}: {
	deploymentId: string;
	projectId: string;
	resource: DeploymentResource | undefined;
}) {
	const isCompute = resource !== undefined && resource.kind !== "static";
	const summary = trpc.deploy.metrics.getDeploymentResourceSummary.useQuery(
		{
			resourceId: deploymentId,
			deploymentResourceId: resource?.id,
		},
		{ enabled: isCompute },
	);
	const logs = trpc.deploy.deployment.runtimeLogs.useQuery(
		{ deploymentId, resourceId: resource?.id, limit: 8 },
		{ enabled: isCompute },
	);
	const events = trpc.deploy.deployment.instanceEvents.useQuery(
		{
			projectId,
			deploymentId,
			resourceIds: resource ? [resource.id] : [],
			limit: 8,
		},
		{ enabled: isCompute },
	);

	if (!resource) {
		return null;
	}

	return (
		<div className="mt-3 grid gap-3 rounded-lg border border-grayA-5 bg-gray-1 p-3">
			<div className="flex flex-wrap items-start justify-between gap-3">
				<div>
					<p className="text-xs font-medium text-gray-12">
						{resource.name} observability
					</p>
					<p className="mt-0.5 text-[11px] text-gray-9">
						{resource.kind === "cron"
							? resource.schedule
							: resource.kind === "function"
								? `${resource.runtime} · ${resource.handler}`
								: (resource.image ?? "Static artifact")}
					</p>
				</div>
				{resource.bindings.length > 0 ? (
					<div className="flex flex-wrap gap-1">
						{resource.bindings.map((binding) => (
							<ResourceBadge key={`${resource.id}:${binding.name}`}>
								{binding.name} → {binding.resourceName}:{binding.port}
							</ResourceBadge>
						))}
					</div>
				) : null}
			</div>

			{resource.command.length > 0 ? (
				<div className="rounded-md bg-grayA-2 px-3 py-2 font-mono text-[10px] text-gray-11">
					{resource.command.join(" ")}
				</div>
			) : null}

			{isCompute ? (
				<>
					<div className="grid gap-2 sm:grid-cols-3">
						<ResourceValue
							label="Active instances"
							value={String(
								summary.data?.active_instances ?? resource.instances.length,
							)}
						/>
						<ResourceValue
							label="CPU now"
							value={`${Math.round(summary.data?.current_cpu_millicores ?? 0)}m`}
						/>
						<ResourceValue
							label="Memory now"
							value={formatBytes(summary.data?.current_memory_bytes ?? 0)}
						/>
					</div>
					<div className="grid gap-3 xl:grid-cols-2">
						<ObservabilityList
							title="Recent logs"
							pending={logs.isPending}
							empty="No logs for this resource yet."
							rows={(logs.data?.logs ?? []).map((log) => ({
								key: `${log.time}:${log.instance_id}:${log.message}`,
								lead: new Date(log.time).toLocaleTimeString(),
								value: log.message,
								badge: log.severity,
							}))}
						/>
						<ObservabilityList
							title="Lifecycle events"
							pending={events.isPending}
							empty="No lifecycle events for this resource yet."
							rows={(events.data?.events ?? []).map((event) => ({
								key: `${event.time}:${event.eventFingerprint}`,
								lead: new Date(event.time).toLocaleTimeString(),
								value: event.reason || event.message || event.eventKind,
								badge: event.eventKind,
							}))}
						/>
					</div>
				</>
			) : (
				<p className="text-[11px] text-gray-9">
					Static output is served directly from its immutable edge artifact.
				</p>
			)}
		</div>
	);
}

function ObservabilityList({
	title,
	pending,
	empty,
	rows,
}: {
	title: string;
	pending: boolean;
	empty: string;
	rows: Array<{ key: string; lead: string; value: string; badge: string }>;
}) {
	return (
		<div className="rounded-md border border-grayA-4 p-3">
			<p className="mb-2 text-[10px] font-medium uppercase tracking-wide text-gray-9">
				{title}
			</p>
			{pending ? (
				<div className="h-12 animate-pulse rounded bg-grayA-2" />
			) : rows.length === 0 ? (
				<p className="text-[11px] text-gray-9">{empty}</p>
			) : (
				<div className="grid gap-1.5">
					{rows.map((row) => (
						<div
							key={row.key}
							className="flex min-w-0 items-center gap-2 text-[10px]"
						>
							<span className="shrink-0 font-mono text-gray-8">{row.lead}</span>
							<ResourceBadge>{row.badge}</ResourceBadge>
							<span className="truncate text-gray-11">{row.value}</span>
						</div>
					))}
				</div>
			)}
		</div>
	);
}

function ResourceBlock({
	icon,
	title,
	empty,
	hasContent,
	children,
}: {
	icon: ReactNode;
	title: string;
	empty: string;
	hasContent: boolean;
	children: ReactNode;
}) {
	return (
		<div className="p-4">
			<div className="mb-3 flex items-center gap-2 text-gray-9">
				{icon}
				<h3 className="text-xs font-medium text-gray-12">{title}</h3>
			</div>
			{hasContent ? children : <p className="text-xs text-gray-9">{empty}</p>}
		</div>
	);
}

function ResourceValue({
	label,
	value,
	copyValue,
}: {
	label: string;
	value: string;
	copyValue?: string;
}) {
	return (
		<div className="rounded-md border border-grayA-4 bg-grayA-2 px-3 py-2">
			<p className="text-[10px] uppercase tracking-wide text-gray-9">{label}</p>
			<div className="mt-1 flex items-center gap-1">
				<p className="truncate font-mono text-xs text-gray-12">{value}</p>
				{copyValue ? (
					<CopyButton value={copyValue} variant="ghost" toastMessage={label} />
				) : null}
			</div>
		</div>
	);
}

function ResourceBadge({
	children,
	accent = false,
}: { children: ReactNode; accent?: boolean }) {
	return (
		<span
			className={
				accent
					? "shrink-0 rounded-full bg-successA-3 px-2 py-0.5 text-[10px] font-medium text-success-11"
					: "shrink-0 rounded-full bg-grayA-3 px-2 py-0.5 text-[10px] font-medium text-gray-10"
			}
		>
			{children}
		</span>
	);
}

function shortenDigest(value: string) {
	return value.length > 16 ? `${value.slice(0, 8)}…${value.slice(-8)}` : value;
}

function formatBytes(value: number) {
	if (value < 1024) {
		return `${value} B`;
	}
	if (value < 1024 * 1024) {
		return `${(value / 1024).toFixed(1)} KiB`;
	}
	if (value < 1024 * 1024 * 1024) {
		return `${(value / (1024 * 1024)).toFixed(1)} MiB`;
	}
	return `${(value / (1024 * 1024 * 1024)).toFixed(1)} GiB`;
}
