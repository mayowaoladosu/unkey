"use client";

import type { Deployment } from "@/lib/collections/deploy/deployments";
import { shortenId } from "@/lib/shorten-id";
import {
	ArrowDottedRotateAnticlockwise,
	Ban,
	Bolt,
	CloudUp,
	Minus,
} from "@unkey/icons";
import {
	Button,
	PageHeader,
	PageHeaderActions,
	PageHeaderContent,
	PageHeaderTitle,
} from "@unkey/ui";
import dynamic from "next/dynamic";
import { useState } from "react";
import {
	isCancellableDeploymentStatus,
	isRedeployableDeploymentStatus,
} from "../components/table/components/actions/deployment-action-eligibility";
import { StopDialog } from "../components/table/components/actions/stop-dialog";
import { WakeDialog } from "../components/table/components/actions/wake-dialog";
import { useProjectData } from "../../data-provider";
import { useDeployment } from "./layout-provider";
import { useDeploymentStatus } from "./use-deployment-status";

const RedeployDialog = dynamic(
	() =>
		import("../components/table/components/actions/redeploy-dialog").then(
			(m) => m.RedeployDialog,
		),
	{ ssr: false },
);

const CancelDialog = dynamic(
	() =>
		import("../components/table/components/actions/cancel-dialog").then(
			(m) => m.CancelDialog,
		),
	{ ssr: false },
);

const PromoteToProductionDialog = dynamic(
	() =>
		import(
			"../components/table/components/actions/promote-to-production-dialog"
		).then((m) => m.PromoteToProductionDialog),
	{ ssr: false },
);

export function DeploymentDetailHeader() {
	const { deployment } = useDeployment();
	// Keyed by id so dialog and cancelled state reset when navigation swaps
	// the deployment under this layout (e.g. Redeploy pushes to the new one).
	return (
		<DeploymentDetailHeaderContent
			key={deployment.id}
			deployment={deployment}
		/>
	);
}

function DeploymentDetailHeaderContent({
	deployment,
}: { deployment: Deployment }) {
	const { derivedStatus } = useDeploymentStatus(deployment);
	const { environments } = useProjectData();
	const [isRedeployOpen, setIsRedeployOpen] = useState(false);
	const [isCancelOpen, setIsCancelOpen] = useState(false);
	const [isStopOpen, setIsStopOpen] = useState(false);
	const [isWakeOpen, setIsWakeOpen] = useState(false);
	const [isPromoteToProductionOpen, setIsPromoteToProductionOpen] = useState(false);
	const [cancelled, setCancelled] = useState(false);
	const canCancel = isCancellableDeploymentStatus(derivedStatus) && !cancelled;
	const canRedeploy = isRedeployableDeploymentStatus(derivedStatus);
	const environment = environments.find((item) => item.id === deployment.environmentId);
	const isNonProduction = environment !== undefined && environment.slug !== "production";
	const canStop = isNonProduction && derivedStatus === "ready";
	const canWake = isNonProduction && derivedStatus === "stopped";
	const canPromoteToProduction = isNonProduction && derivedStatus === "ready";

	const title = deployment.gitCommitMessage || shortenId(deployment.id);

	return (
		<PageHeader>
			<PageHeaderContent>
				<PageHeaderTitle className="truncate" title={title}>
					{title}
				</PageHeaderTitle>
			</PageHeaderContent>
			<PageHeaderActions>
				{canPromoteToProduction && (
					<Button variant="primary" onClick={() => setIsPromoteToProductionOpen(true)}>
						<CloudUp iconSize="sm-medium" />
						Promote to production
					</Button>
				)}
				{canWake && (
					<Button variant="primary" onClick={() => setIsWakeOpen(true)}>
						<Bolt iconSize="sm-medium" />
						Wake deployment
					</Button>
				)}
				{canStop && (
					<Button variant="outline" onClick={() => setIsStopOpen(true)}>
						<Minus iconSize="sm-medium" />
						Stop deployment
					</Button>
				)}
				{canCancel && (
					<Button variant="outline" onClick={() => setIsCancelOpen(true)}>
						<Ban iconSize="sm-medium" />
						Cancel deployment
					</Button>
				)}
				{canRedeploy && (
					<Button variant="outline" onClick={() => setIsRedeployOpen(true)}>
						<ArrowDottedRotateAnticlockwise iconSize="sm-regular" />
						Redeploy
					</Button>
				)}
			</PageHeaderActions>
			{canRedeploy && (
				<RedeployDialog
					isOpen={isRedeployOpen}
					onClose={() => setIsRedeployOpen(false)}
					selectedDeployment={deployment}
				/>
			)}
			{canCancel && (
				<CancelDialog
					isOpen={isCancelOpen}
					onClose={() => setIsCancelOpen(false)}
					onCancelled={() => setCancelled(true)}
					deployment={deployment}
				/>
			)}
			{canStop && (
				<StopDialog
					isOpen={isStopOpen}
					onClose={() => setIsStopOpen(false)}
					deployment={deployment}
				/>
			)}
			{canWake && (
				<WakeDialog
					isOpen={isWakeOpen}
					onClose={() => setIsWakeOpen(false)}
					deployment={deployment}
				/>
			)}
			{canPromoteToProduction && (
				<PromoteToProductionDialog
					isOpen={isPromoteToProductionOpen}
					onClose={() => setIsPromoteToProductionOpen(false)}
					deployment={deployment}
				/>
			)}
		</PageHeader>
	);
}
