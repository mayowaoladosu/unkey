"use client";

import { useProjectData } from "@/app/(app)/[workspaceSlug]/projects/[projectId]/apps/[appId]/(overview)/data-provider";
import { DeploymentEnvVars } from "@/app/(app)/[workspaceSlug]/projects/[projectId]/apps/[appId]/(overview)/env-vars/deployment-env-vars";
import { DeploymentSettings } from "@/app/(app)/[workspaceSlug]/projects/[projectId]/apps/[appId]/(overview)/settings/deployment-settings";
import { useEnvironmentSettings } from "@/app/(app)/[workspaceSlug]/projects/[projectId]/apps/[appId]/(overview)/settings/environment-provider";
import { trpc } from "@/lib/trpc/client";
import {
  BracketsSquareDots,
  Check,
  CloudUp,
  CodeBranch,
  Cube,
  Earth,
  Github,
  Harddrive,
  Layers2,
} from "@unkey/icons";
import { useState } from "react";
import { DeployAction } from "../deploy-action";
import { FrameworkDetectionCard } from "./framework-detection-card";

type DeploymentSource = { type: "github" } | { type: "image"; image: string };

export const ConfigureDeploymentContent = ({
  projectId,
  appId,
  source,
  onDeploymentCreated,
}: {
  projectId: string;
  appId: string;
  source: DeploymentSource;
  onDeploymentCreated: (deploymentId: string) => void;
}) => {
  const [settingsRevision, setSettingsRevision] = useState(0);
  const { settings, isSaving } = useEnvironmentSettings();
  const { environments } = useProjectData();
  const github = trpc.github.getInstallations.useQuery(
    { projectId, appId },
    { enabled: source.type === "github", refetchOnWindowFocus: false },
  );

  const repository = github.data?.repoConnection?.repositoryFullName ?? "Connected repository";
  const branch = github.data?.defaultBranch ?? "default branch";
  const resourceCount = settings.outputs.length || 1;
  const resourceLabel = settings.outputs.length > 0 ? "explicit resources" : "detected service";

  return (
    <div className="mx-auto w-full max-w-6xl pb-16">
      <div className="mb-6 flex flex-col gap-4 rounded-xl border border-grayA-5 bg-gray-1 p-5 shadow-sm sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-center gap-3">
          <span className="grid size-10 shrink-0 place-items-center rounded-lg border border-grayA-5 bg-grayA-2">
            {source.type === "github" ? (
              <Github iconSize="lg-medium" />
            ) : (
              <Harddrive iconSize="lg-medium" />
            )}
          </span>
          <div className="min-w-0">
            <p className="truncate text-sm font-semibold text-gray-12">
              {source.type === "github" ? repository : source.image}
            </p>
            <p className="mt-1 flex items-center gap-1.5 text-xs text-gray-9">
              {source.type === "github" ? (
                <>
                  <CodeBranch className="size-3" /> {branch}
                </>
              ) : (
                "Prebuilt OCI image"
              )}
            </p>
          </div>
        </div>
        <div className="flex items-center gap-2 rounded-full border border-successA-5 bg-successA-2 px-3 py-1.5 text-xs font-medium text-success-11">
          <Check className="size-3" /> Source connected
        </div>
      </div>

      <div className="grid min-w-0 gap-6 xl:grid-cols-[minmax(0,1fr)_320px]">
        <main className="min-w-0 space-y-8">
          {source.type === "github" ? (
            <section>
              <SectionHeading
                icon={<CloudUp iconSize="md-medium" />}
                title="Build and source"
                description="Review what was detected. Railpack verifies the checked-out commit when the build starts."
              />
              <FrameworkDetectionCard
                onDefaultsApplied={() => setSettingsRevision((value) => value + 1)}
              />
            </section>
          ) : null}

          <section>
            <SectionHeading
              icon={<Layers2 iconSize="md-medium" />}
              title={
                source.type === "github"
                  ? "Build, resources, and routing"
                  : "Resources and routing"
              }
              description="Configure the production topology, including services, functions, workers, cron jobs, static outputs, regions, health checks, and domains."
            />
            <DeploymentSettings
              key={settingsRevision}
              githubReadOnly
              sections={{
                ...(source.type === "github" ? { build: true as const } : {}),
                runtime: true,
              }}
            />
          </section>

          <section className="rounded-xl border border-grayA-5 bg-gray-1 p-5 shadow-sm">
            <SectionHeading
              icon={<BracketsSquareDots iconSize="md-medium" />}
              title="Environment variables"
              description="Add secrets and configuration before the first build. Paste rows or import a complete .env file."
              compact
            />
            <DeploymentEnvVars />
          </section>
        </main>

        <aside className="h-fit rounded-xl border border-grayA-5 bg-gray-1 p-5 shadow-sm xl:sticky xl:top-4">
          <div className="flex items-center gap-2">
            <Cube iconSize="md-medium" className="text-gray-11" />
            <h2 className="text-sm font-semibold text-gray-12">Production summary</h2>
          </div>

          <div className="mt-5 divide-y divide-grayA-4 rounded-lg border border-grayA-4 bg-grayA-2 px-3">
            <SummaryRow
              icon={<Layers2 className="size-3.5" />}
              label="Resources"
              value={`${resourceCount} ${resourceLabel}`}
            />
            <SummaryRow
              icon={<Earth className="size-3.5" />}
              label="Regions"
              value={
                settings.regions.length > 0
                  ? `${settings.regions.length} selected`
                  : "Select a region"
              }
            />
            <SummaryRow
              icon={<Cube className="size-3.5" />}
              label="Compute"
              value={`${settings.cpuMillicores}m · ${settings.memoryMib} MiB`}
            />
            <SummaryRow
              icon={<CloudUp className="size-3.5" />}
              label="Environments"
              value={`${environments.length} configured`}
            />
          </div>

          <div className="my-5 space-y-2.5 text-xs text-gray-10">
            <ChecklistItem>Immutable deployment manifest</ChecklistItem>
            <ChecklistItem>Health-checked rollout</ChecklistItem>
            <ChecklistItem>Automatic HTTPS and deployment URL</ChecklistItem>
            <ChecklistItem>Rollback target retained</ChecklistItem>
          </div>

          {isSaving ? (
            <div className="mb-3 rounded-lg border border-warningA-5 bg-warningA-2 px-3 py-2 text-xs text-warning-11">
              Saving configuration before deployment…
            </div>
          ) : null}

          <DeployAction
            projectId={projectId}
            appId={appId}
            source={source}
            disabled={isSaving || settings.regions.length === 0}
            onDeploymentCreated={onDeploymentCreated}
          />
        </aside>
      </div>
    </div>
  );
};

function SectionHeading({
  icon,
  title,
  description,
  compact = false,
}: {
  icon: React.ReactNode;
  title: string;
  description: string;
  compact?: boolean;
}) {
  return (
    <div className={compact ? "mb-5" : "mb-4"}>
      <div className="flex items-center gap-2 text-gray-12">
        {icon}
        <h2 className="text-sm font-semibold">{title}</h2>
      </div>
      <p className="mt-1.5 max-w-3xl text-xs leading-5 text-gray-9">{description}</p>
    </div>
  );
}

function SummaryRow({
  icon,
  label,
  value,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
}) {
  return (
    <div className="flex items-center justify-between gap-3 py-3 text-xs">
      <span className="flex items-center gap-2 text-gray-9">
        {icon} {label}
      </span>
      <span className="text-right font-medium text-gray-11">{value}</span>
    </div>
  );
}

function ChecklistItem({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-2">
      <span className="grid size-4 shrink-0 place-items-center rounded-full bg-successA-3 text-success-11">
        <Check className="size-2.5" />
      </span>
      {children}
    </div>
  );
}
