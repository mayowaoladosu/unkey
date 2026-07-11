"use client";

import { ProjectDataProvider } from "@/app/(app)/[workspaceSlug]/projects/[projectId]/apps/[appId]/(overview)/data-provider";
import { useState } from "react";
import { ConfigureDeploymentContent } from "./content";
import { OnboardingEnvironmentSettingsProvider } from "./environment-provider";
import { ConfigureDeploymentFallback } from "./fallback";

type ConfigureDeploymentStepProps = {
  projectId: string;
  appId: string;
  source: { type: "github" } | { type: "image"; image: string };
  onDeploymentCreated: (deploymentId: string) => void;
};

export const ConfigureDeploymentStep = ({
  projectId,
  appId,
  source,
  onDeploymentCreated,
}: ConfigureDeploymentStepProps) => {
  const [settingsStatus, setSettingsStatus] = useState<"loading" | "ready" | "error">("loading");

  return (
    <ProjectDataProvider projectId={projectId} appId={appId}>
      <OnboardingEnvironmentSettingsProvider onSettingsStatusChange={setSettingsStatus}>
        <ConfigureDeploymentContent
          projectId={projectId}
          appId={appId}
          source={source}
          onDeploymentCreated={onDeploymentCreated}
        />
      </OnboardingEnvironmentSettingsProvider>
      <ConfigureDeploymentFallback settingsStatus={settingsStatus} />
    </ProjectDataProvider>
  );
};
