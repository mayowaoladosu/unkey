"use client";

import { useWorkspaceNavigation } from "@/hooks/use-workspace-navigation";
import { githubUrl } from "@/lib/github-url";
import { routes } from "@/lib/navigation/routes";
import {
  ArrowRight,
  BracketsSquareDots,
  Check,
  Earth,
  ExternalLink,
  Github,
  Layers2,
} from "@unkey/icons";
import { Button, CopyButton } from "@unkey/ui";
import { useRouter } from "next/navigation";
import { useProjectData } from "../../../../data-provider";
import { getDomainPriority } from "../../../../../components/domain-priority";
import { useDeployment } from "../../layout-provider";

export function DeploymentReadyHero({
  environmentSlug,
  onDismiss,
}: {
  environmentSlug: string;
  onDismiss: () => void;
}) {
  const { deployment } = useDeployment();
  const workspace = useWorkspaceNavigation();
  const router = useRouter();
  const {
    getDomainsForDeployment,
    customDomains,
    project,
    isDomainsLoading,
    isCustomDomainsLoading,
  } = useProjectData();
  const scope = {
    workspaceSlug: workspace.slug,
    projectId: deployment.projectId,
    appId: deployment.appId,
    deploymentId: deployment.id,
  };
  const domains = getDomainPriority({
    domains: getDomainsForDeployment(deployment.id),
    customDomains,
    environmentId: deployment.environmentId,
    deploymentId: deployment.id,
    currentDeploymentId: project?.currentDeploymentId ?? null,
  });
  const primaryDomain = domains.primary;
  const domainLoading = isDomainsLoading || isCustomDomainsLoading;
  const isPreview = environmentSlug === "preview" || deployment.prNumber !== null;
  const title = deployment.prNumber
    ? `Preview #${deployment.prNumber} is ready`
    : isPreview
      ? "Preview deployment is ready"
      : "Production is live";
  const sourceUrl = githubUrl.deployment({
    repoFullName: project?.repositoryFullName,
    forkRepoFullName: deployment.forkRepositoryFullName,
    prNumber: deployment.prNumber,
    sha: deployment.gitCommitSha,
  });

  return (
    <section className="relative overflow-hidden rounded-2xl border border-successA-6 bg-linear-to-br from-successA-3 via-gray-1 to-accentA-2 p-6 shadow-sm sm:p-8">
      <div
        className="pointer-events-none absolute inset-0 opacity-40"
        style={{
          backgroundImage:
            "radial-gradient(circle at 1px 1px, var(--gray-a5) 1px, transparent 0)",
          backgroundSize: "20px 20px",
          maskImage: "linear-gradient(to right, black, transparent 70%)",
        }}
      />
      <div className="relative grid gap-8 lg:grid-cols-[minmax(0,1fr)_300px] lg:items-end">
        <div>
          <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.16em] text-success-11">
            <span className="grid size-5 place-items-center rounded-full bg-success-9 text-white">
              <Check className="size-3" />
            </span>
            Deployment complete
          </div>
          <h2 className="mt-4 text-2xl font-semibold tracking-tight text-gray-12 sm:text-3xl">
            {title}
          </h2>
          <p className="mt-2 max-w-2xl text-sm leading-6 text-gray-10">
            The immutable manifest is materialized, health checks passed, and traffic is assigned
            to this deployment. Every resource can now be inspected independently.
          </p>

          <div className="mt-5 flex min-h-10 flex-wrap items-center gap-2">
            {domainLoading ? (
              <div className="h-10 w-72 animate-pulse rounded-lg bg-grayA-3" />
            ) : primaryDomain ? (
              <div className="flex min-w-0 items-center gap-1 rounded-lg border border-grayA-5 bg-gray-1 p-1 pl-3 shadow-sm">
                <Earth className="size-3.5 shrink-0 text-gray-9" />
                <a
                  href={primaryDomain.url}
                  target="_blank"
                  rel="noreferrer"
                  className="max-w-[320px] truncate px-1 text-xs font-medium text-gray-12 hover:underline"
                >
                  {primaryDomain.hostname}
                </a>
                <CopyButton
                  value={primaryDomain.url}
                  variant="ghost"
                  toastMessage="Deployment URL"
                />
              </div>
            ) : (
              <span className="rounded-lg border border-warningA-5 bg-warningA-2 px-3 py-2 text-xs text-warning-11">
                The deployment is healthy. Its URL is still being assigned.
              </span>
            )}
            {primaryDomain ? (
              <a href={primaryDomain.url} target="_blank" rel="noreferrer">
                <Button type="button" variant="primary">
                  Open deployment <ExternalLink iconSize="sm-regular" />
                </Button>
              </a>
            ) : null}
            {sourceUrl ? (
              <a href={sourceUrl} target="_blank" rel="noreferrer">
                <Button type="button" variant="outline">
                  <Github iconSize="sm-regular" /> View source
                </Button>
              </a>
            ) : null}
          </div>
        </div>

        <div className="rounded-xl border border-grayA-5 bg-gray-1/90 p-2 shadow-sm backdrop-blur">
          <NextAction
            icon={<BracketsSquareDots className="size-4" />}
            title="Inspect logs and events"
            description="Verify startup and runtime output"
            onClick={() => router.push(routes.projects.apps.deploymentLogs(scope))}
          />
          <NextAction
            icon={<Layers2 className="size-4" />}
            title="Review resources"
            description="Services, functions, jobs, and artifacts"
            onClick={() => router.push(routes.projects.apps.deploymentResources(scope))}
          />
          <NextAction
            icon={<Earth className="size-4" />}
            title="Add a custom domain"
            description="Configure DNS and automatic HTTPS"
            onClick={() =>
              router.push(
                routes.projects.apps.settings({
                  workspaceSlug: workspace.slug,
                  projectId: deployment.projectId,
                  appId: deployment.appId,
                }),
              )
            }
          />
          <button
            type="button"
            className="mt-1 flex w-full items-center justify-center gap-1 rounded-lg px-3 py-2 text-xs font-medium text-gray-9 transition hover:bg-grayA-2 hover:text-gray-11"
            onClick={onDismiss}
          >
            Continue to overview <ArrowRight className="size-3" />
          </button>
        </div>
      </div>
    </section>
  );
}

function NextAction({
  icon,
  title,
  description,
  onClick,
}: {
  icon: React.ReactNode;
  title: string;
  description: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="group flex w-full items-center gap-3 rounded-lg p-3 text-left transition hover:bg-grayA-2"
    >
      <span className="grid size-8 shrink-0 place-items-center rounded-lg border border-grayA-5 bg-gray-1 text-gray-10 group-hover:text-gray-12">
        {icon}
      </span>
      <span className="min-w-0 flex-1">
        <span className="block text-xs font-medium text-gray-12">{title}</span>
        <span className="mt-0.5 block truncate text-[11px] text-gray-9">{description}</span>
      </span>
      <ArrowRight className="size-3 text-gray-8 transition group-hover:translate-x-0.5 group-hover:text-gray-11" />
    </button>
  );
}