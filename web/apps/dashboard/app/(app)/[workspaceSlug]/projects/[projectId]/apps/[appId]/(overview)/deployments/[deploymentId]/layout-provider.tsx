"use client";

import { LoadingState } from "@/components/loading-state";
import { TOP_NAV_HEIGHT } from "@/components/navigation/top-nav";
import { type Deployment, deploymentSchema } from "@/lib/collections/deploy/deployments";
import { trpc } from "@/lib/trpc/client";
import { notFound, useParams } from "next/navigation";
import { createContext, useContext } from "react";
import { useProjectData } from "../../data-provider";

type DeploymentLayoutContextType = {
  deployment: Deployment;
};

const DeploymentLayoutContext = createContext<DeploymentLayoutContextType | null>(null);

type DeploymentLayoutProviderProps = {
  children: React.ReactNode;
  deploymentId?: string;
};

export const DeploymentLayoutProvider = ({
  children,
  deploymentId: deploymentIdProp,
}: DeploymentLayoutProviderProps) => {
  const params = useParams();
  const deploymentId =
    deploymentIdProp ??
    (typeof params?.deploymentId === "string" ? params.deploymentId : undefined);

  if (!deploymentId) {
    throw new Error("DeploymentLayoutProvider requires a deploymentId (via prop or route params)");
  }

  const { getDeploymentById, isDeploymentsLoading, projectId } = useProjectData();
  const fromCollection = getDeploymentById(deploymentId);
  const needsFetch = fromCollection === undefined;

  const fetchByIdQuery = trpc.deploy.deployment.getById.useQuery(
    { deploymentId, projectId },
    { enabled: needsFetch },
  );

  const parsed = fetchByIdQuery.data ? deploymentSchema.safeParse(fetchByIdQuery.data) : undefined;
  const resolved = fromCollection ?? (parsed?.success ? parsed.data : undefined);

  if (!resolved) {
    const isWaitingForCollection = isDeploymentsLoading;
    const isWaitingForFetch = needsFetch && !fetchByIdQuery.isFetched;

    if (isWaitingForCollection || isWaitingForFetch) {
      return (
        <div className="flex flex-col" style={{ height: `calc(100dvh - ${TOP_NAV_HEIGHT}px)` }}>
          <LoadingState message="Loading deployment..." />
        </div>
      );
    }

    if (needsFetch) {
      if (fetchByIdQuery.error?.data?.code === "NOT_FOUND") {
        notFound();
      }
      if (fetchByIdQuery.isError) {
        throw fetchByIdQuery.error;
      }
      if (fetchByIdQuery.data && !parsed?.success) {
        throw new Error(`Invalid deployment payload for ${deploymentId}`);
      }
      notFound();
    }

    return (
      <div className="flex flex-col" style={{ height: `calc(100dvh - ${TOP_NAV_HEIGHT}px)` }}>
        <LoadingState message="Loading deployment..." />
      </div>
    );
  }

  return (
    <DeploymentLayoutContext.Provider value={{ deployment: resolved }}>
      {children}
    </DeploymentLayoutContext.Provider>
  );
};

export const useDeployment = () => {
  const context = useContext(DeploymentLayoutContext);
  if (!context) {
    throw new Error("useDeployment must be used within a deployment route");
  }
  return context;
};
