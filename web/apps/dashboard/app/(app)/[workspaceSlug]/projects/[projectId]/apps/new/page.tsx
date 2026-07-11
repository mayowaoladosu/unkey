"use client";
import { usePreventLeave } from "@/hooks/use-prevent-leave";
import { useWorkspaceNavigation } from "@/hooks/use-workspace-navigation";
import { routes } from "@/lib/navigation/routes";
import { trpc } from "@/lib/trpc/client";
import { StepWizard } from "@unkey/ui";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { OnboardingStepContainer } from "./onboarding-step-container";
import { OnboardingStepHeader } from "./onboarding-step-header";
import { ConfigureDeploymentStep } from "./steps/configure-deployment";
import { ConnectGithubStep } from "./steps/connect-github";
import { CreateAppStep } from "./steps/create-app";
import { DeploymentLiveStep } from "./steps/deployment-live";
import { SelectRepo } from "./steps/select-repo";

export default function AppSetupPage() {
  const { data: context } = trpc.deploy.project.creationContext.useQuery();
  const hasGithubInstallation = context?.hasGithubInstallation === true;
  const searchParams = useSearchParams();
  const params = useParams();
  const router = useRouter();
  const workspace = useWorkspaceNavigation();

  const projectId = typeof params?.projectId === "string" ? params.projectId : "";

  // Step id to start the wizard at (e.g. "select-repo"). When the GitHub
  // callback redirects here, earlier steps are already complete so we skip ahead.
  const initialStep = searchParams.get("step") ?? undefined;
  // App id created in the first step. Later steps need it; the GitHub install
  // round-trip carries it back via the signed state and ?appId= so it survives
  // the full-page redirect.
  const initialAppId = searchParams.get("appId") ?? undefined;
  const source =
    searchParams.get("source") === "image"
      ? { type: "image" as const, image: searchParams.get("image") ?? "" }
      : { type: "github" as const };

  const [appId, setAppId] = useState<string | null>(initialAppId ?? null);
  const [deploymentId, setDeploymentId] = useState<string | null>(null);

  const { bypass } = usePreventLeave(!deploymentId);

  const handleSkipGithubSetup = () => {
    bypass();
    router.replace(routes.projects.detail({ workspaceSlug: workspace.slug, projectId }));
  };

  return (
    <StepWizard.Root defaultStepId={initialStep}>
      <StepWizard.Step id="create-app" label="Create app">
        <OnboardingStepContainer>
          {deployYourAppHeader}
          <CreateAppStep projectId={projectId} onAppCreated={setAppId} />
        </OnboardingStepContainer>
      </StepWizard.Step>
      {!hasGithubInstallation && (
        <StepWizard.Step id="connect-github" label="Connect GitHub">
          {appId ? (
            <OnboardingStepContainer>
              {deployYourAppHeader}
              <ConnectGithubStep projectId={projectId} appId={appId} onBeforeNavigate={bypass} />
            </OnboardingStepContainer>
          ) : null}
        </StepWizard.Step>
      )}
      <StepWizard.Step id="select-repo" label="Select repository" kind="optional">
        {appId ? (
          <OnboardingStepContainer>
            <OnboardingStepHeader
              title="Select a repository"
              subtitle={
                <>
                  Choose a repository and a branch containing your app.
                  <br />
                  We'll detect how to build it automatically.
                </>
              }
            />
            <SelectRepo
              projectId={projectId}
              appId={appId}
              onBeforeNavigate={bypass}
              hasGithubInstallation={context?.hasGithubInstallation ?? false}
              onSkip={handleSkipGithubSetup}
            />
          </OnboardingStepContainer>
        ) : null}
      </StepWizard.Step>
      <StepWizard.Step id="configure-deployment" label="Configure and deploy">
        {appId ? (
          <OnboardingStepContainer>
            <OnboardingStepHeader
              title="Review and deploy"
              subtitle="Build, resources, secrets, regions, and routing are reviewed in one place."
            />
            <ConfigureDeploymentStep
              projectId={projectId}
              appId={appId}
              source={source}
              onDeploymentCreated={setDeploymentId}
            />
          </OnboardingStepContainer>
        ) : null}
      </StepWizard.Step>
      <StepWizard.Step id="deploying" label="Deploying" preventBack>
        {appId && deploymentId ? (
          <DeploymentLiveStep projectId={projectId} appId={appId} deploymentId={deploymentId} />
        ) : null}
      </StepWizard.Step>
    </StepWizard.Root>
  );
}

const deployYourAppHeader = (
  <OnboardingStepHeader
    title="Deploy your app"
    showIconRow
    subtitle={
      <>
        Connect a GitHub repo and get a live URL in minutes.
        <br />
        Unkey handles builds, infra, scaling, and routing.
      </>
    }
  />
);
