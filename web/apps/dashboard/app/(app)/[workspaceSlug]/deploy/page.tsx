"use client";

import { queryClient } from "@/lib/collections/client";
import { routes } from "@/lib/navigation/routes";
import { slugify } from "@/lib/slugify";
import { trpc } from "@/lib/trpc/client";
import {
  ArrowRight,
  Check,
  CloudUp,
  CodeBranch,
  Cube,
  Github,
  Harddrive,
  Magnifier,
} from "@unkey/icons";
import {
  Button,
  Input,
  PageBody,
  PageContainer,
  PageHeader,
  PageHeaderContent,
  PageHeaderTitle,
  toast,
} from "@unkey/ui";
import { cn } from "@unkey/ui/src/lib/utils";
import { useParams, useRouter } from "next/navigation";
import { useMemo, useState } from "react";

type SourceMode = "github" | "image";

type Repository = {
  id: number;
  name: string;
  fullName: string;
  private: boolean;
  defaultBranch: string;
  installationId: number;
  pushedAt: string | null;
  language: string | null;
};

const fieldClass =
  "h-10 w-full rounded-lg border border-grayA-5 bg-gray-1 px-3 text-[13px] text-gray-12 outline-none transition focus:border-accent-8 focus:ring-2 focus:ring-accentA-4";

export default function DeployPage() {
  const params = useParams();
  const router = useRouter();
  const workspaceSlug = typeof params.workspaceSlug === "string" ? params.workspaceSlug : "";
  const [mode, setMode] = useState<SourceMode>("github");
  const [search, setSearch] = useState("");
  const [selectedRepository, setSelectedRepository] = useState<Repository | null>(null);
  const [selectedBranch, setSelectedBranch] = useState("");
  const [projectName, setProjectName] = useState("");
  const [projectSlug, setProjectSlug] = useState("");
  const [image, setImage] = useState("");

  const installations = trpc.github.hasInstallations.useQuery();
  const hasGithubInstallation = installations.data?.hasInstallation === true;
  const repositories = trpc.github.listWorkspaceRepositories.useQuery(undefined, {
    enabled: hasGithubInstallation,
    refetchOnWindowFocus: false,
  });
  const repositoryDetails = trpc.github.getWorkspaceRepositoryDetails.useQuery(
    {
      installationId: selectedRepository?.installationId ?? 0,
      repositoryId: selectedRepository?.id ?? 0,
    },
    {
      enabled: selectedRepository !== null,
      refetchOnWindowFocus: false,
    },
  );
  const prepareInstallation = trpc.github.prepareInstallation.useMutation();
  const initialize = trpc.deploy.project.initialize.useMutation({
    onSuccess: async (result) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["projects"] }),
        queryClient.invalidateQueries({ queryKey: ["apps", result.projectId] }),
      ]);
      router.push(
        routes.projects.apps.new({
          workspaceSlug,
          projectId: result.projectId,
          appId: result.appId,
          step: "configure-deployment",
          source: mode,
          image: mode === "image" ? image.trim() : undefined,
        }),
      );
    },
    onError: (error) => {
      toast.error("Unable to prepare this project", { description: error.message });
    },
  });

  const visibleRepositories = useMemo(() => {
    const needle = search.trim().toLowerCase();
    if (!needle) {
      return repositories.data?.repositories ?? [];
    }
    return (repositories.data?.repositories ?? []).filter((repository) =>
      `${repository.fullName} ${repository.language ?? ""}`.toLowerCase().includes(needle),
    );
  }, [repositories.data?.repositories, search]);

  const selectRepository = (repository: Repository) => {
    setSelectedRepository(repository);
    setSelectedBranch(repository.defaultBranch);
    setProjectName(repository.name);
    setProjectSlug(normalizeProjectSlug(repository.name));
  };

  const selectMode = (nextMode: SourceMode) => {
    setMode(nextMode);
    if (nextMode === "github" && selectedRepository) {
      setProjectName(selectedRepository.name);
      setProjectSlug(normalizeProjectSlug(selectedRepository.name));
    }
  };

  const startGithubInstall = async () => {
    try {
      const { state } = await prepareInstallation.mutateAsync({ returnTo: "deploy" });
      window.location.href = `https://github.com/apps/${process.env.NEXT_PUBLIC_GITHUB_APP_NAME}/installations/new?state=${encodeURIComponent(state)}`;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to connect GitHub");
    }
  };

  const validName = projectName.trim().length > 0 && projectName.trim().length <= 256;
  const validSlug =
    projectSlug.length >= 3 && projectSlug.length <= 256 && /^[a-z0-9-]+$/.test(projectSlug);
  const validSource =
    mode === "github"
      ? selectedRepository !== null && selectedBranch.length > 0
      : image.trim().length > 0 && !/\s/.test(image);
  const canContinue = validName && validSlug && validSource && !initialize.isLoading;

  const continueToConfiguration = () => {
    if (!canContinue) {
      return;
    }
    initialize.mutate({
      name: projectName.trim(),
      slug: projectSlug,
      source:
        mode === "github" && selectedRepository
          ? {
              type: "github",
              installationId: selectedRepository.installationId,
              repositoryId: selectedRepository.id,
              branch: selectedBranch,
            }
          : { type: "image", image: image.trim() },
    });
  };

  return (
    <PageContainer>
      <PageHeader>
        <PageHeaderContent>
          <PageHeaderTitle>Deploy project</PageHeaderTitle>
        </PageHeaderContent>
      </PageHeader>
      <PageBody>
        <div className="mx-auto w-full max-w-6xl pb-16">
          <div className="mb-8 max-w-2xl">
            <div className="mb-3 flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-gray-9">
              <CloudUp iconSize="sm-regular" /> New deployment
            </div>
            <h1 className="text-3xl font-semibold tracking-tight text-gray-12">
              Choose what you want to ship
            </h1>
            <p className="mt-2 text-sm leading-6 text-gray-10">
              Start from a GitHub repository or an existing container image. Build, resources,
              secrets, regions, and routing are reviewed together before the first workload starts.
            </p>
          </div>

          <div className="grid gap-6 lg:grid-cols-[180px_minmax(0,1fr)]">
            <aside className="space-y-2 lg:sticky lg:top-4 lg:self-start">
              <SourceChoice
                active={mode === "github"}
                icon={<Github iconSize="md-medium" />}
                title="GitHub"
                description="Build from source"
                onClick={() => selectMode("github")}
              />
              <SourceChoice
                active={mode === "image"}
                icon={<Harddrive iconSize="md-medium" />}
                title="Container image"
                description="Deploy an OCI image"
                onClick={() => selectMode("image")}
              />
              <div className="mt-5 border-t border-grayA-4 pt-5">
                <ProgressItem complete label="Source" />
                <ProgressItem label="Configure" />
                <ProgressItem label="Deploy" />
              </div>
            </aside>

            <div className="grid min-w-0 gap-5 xl:grid-cols-[minmax(0,1fr)_340px]">
              <section className="min-w-0 rounded-xl border border-grayA-5 bg-gray-1 shadow-sm">
                {mode === "github" ? (
                  <GithubSource
                    connected={hasGithubInstallation}
                    loading={
                      installations.isLoading || (hasGithubInstallation && repositories.isLoading)
                    }
                    error={repositories.error?.message}
                    repositories={visibleRepositories}
                    selectedRepository={selectedRepository}
                    search={search}
                    onSearch={setSearch}
                    onSelect={selectRepository}
                    onConnect={startGithubInstall}
                    connecting={prepareInstallation.isLoading}
                    onRetry={() => repositories.refetch()}
                  />
                ) : (
                  <ImageSource image={image} onImageChange={setImage} />
                )}
              </section>

              <aside className="h-fit rounded-xl border border-grayA-5 bg-gray-1 p-5 shadow-sm xl:sticky xl:top-4">
                <div className="flex items-center gap-2">
                  <Cube iconSize="md-medium" className="text-gray-11" />
                  <h2 className="text-sm font-semibold text-gray-12">Project destination</h2>
                </div>

                {validSource ? (
                  <div className="mt-5 space-y-4">
                    <Field label="Project name">
                      <input
                        className={fieldClass}
                        value={projectName}
                        onChange={(event) => {
                          setProjectName(event.target.value);
                          setProjectSlug(normalizeProjectSlug(event.target.value));
                        }}
                        placeholder="my-project"
                      />
                    </Field>
                    <Field label="Project slug" hint="Used in dashboard URLs">
                      <input
                        className={fieldClass}
                        value={projectSlug}
                        onChange={(event) => setProjectSlug(event.target.value.toLowerCase())}
                        placeholder="my-project"
                      />
                    </Field>
                    {mode === "github" && selectedRepository ? (
                      <Field label="Production branch" hint="First deployment source">
                        <input
                          className={fieldClass}
                          value={selectedBranch}
                          list={`repository-branches-${selectedRepository.id}`}
                          onChange={(event) => setSelectedBranch(event.target.value)}
                          placeholder={selectedRepository.defaultBranch}
                        />
                        <datalist id={`repository-branches-${selectedRepository.id}`}>
                          {(repositoryDetails.data?.branches ?? [
                            { name: selectedRepository.defaultBranch, lastPushDate: null },
                          ]).map((branch) => (
                            <option key={branch.name} value={branch.name}>
                              {branch.name}
                            </option>
                          ))}
                        </datalist>
                        {repositoryDetails.isLoading ? (
                          <span className="text-[11px] font-normal text-gray-9">
                            Loading recently active branches…
                          </span>
                        ) : null}
                        {repositoryDetails.isError ? (
                          <span className="text-[11px] font-normal text-warning-11">
                            Branches could not be refreshed. The repository default remains selected.
                          </span>
                        ) : null}
                      </Field>
                    ) : null}

                    <div className="rounded-lg border border-grayA-4 bg-grayA-2 p-3 text-xs">
                      <SummaryRow
                        label="Source"
                        value={
                          mode === "github"
                            ? (selectedRepository?.fullName ?? "Repository")
                            : image.trim()
                        }
                      />
                      <SummaryRow
                        label="Branch"
                        value={mode === "github" ? selectedBranch || "—" : "—"}
                      />
                      <SummaryRow label="Environment" value="Production" />
                      <SummaryRow label="Build" value={mode === "github" ? "Auto-detect" : "Use image"} />
                    </div>

                    {!validSlug && projectSlug.length > 0 ? (
                      <p className="text-xs text-error-11">
                        Use at least three lowercase letters, numbers, or hyphens.
                      </p>
                    ) : null}

                    <Button
                      type="button"
                      variant="primary"
                      size="xlg"
                      className="w-full rounded-lg"
                      disabled={!canContinue}
                      loading={initialize.isLoading}
                      onClick={continueToConfiguration}
                    >
                      Continue to configuration
                      <ArrowRight iconSize="sm-regular" />
                    </Button>
                    <p className="text-center text-[11px] leading-5 text-gray-9">
                      No workload starts yet. Review every build and runtime setting next.
                    </p>
                  </div>
                ) : (
                  <div className="mt-5 rounded-lg border border-dashed border-grayA-5 px-4 py-8 text-center">
                    <p className="text-sm font-medium text-gray-11">
                      {mode === "github" ? "Select a repository" : "Enter an image reference"}
                    </p>
                    <p className="mt-1 text-xs leading-5 text-gray-9">
                      Project details and the production destination will appear here.
                    </p>
                  </div>
                )}
              </aside>
            </div>
          </div>
        </div>
      </PageBody>
    </PageContainer>
  );
}

function SourceChoice({
  active,
  icon,
  title,
  description,
  onClick,
}: {
  active: boolean;
  icon: React.ReactNode;
  title: string;
  description: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "flex w-full items-center gap-3 rounded-lg border px-3 py-3 text-left transition",
        active
          ? "border-accentA-7 bg-accentA-3 text-accent-12 shadow-sm"
          : "border-transparent text-gray-10 hover:border-grayA-5 hover:bg-grayA-2 hover:text-gray-12",
      )}
    >
      <span className="grid size-8 shrink-0 place-items-center rounded-md border border-grayA-5 bg-gray-1">
        {icon}
      </span>
      <span className="min-w-0">
        <span className="block text-[13px] font-medium">{title}</span>
        <span className="block truncate text-[11px] text-gray-9">{description}</span>
      </span>
    </button>
  );
}

function ProgressItem({ label, complete = false }: { label: string; complete?: boolean }) {
  return (
    <div className="flex items-center gap-2 py-1.5 text-xs text-gray-9">
      <span
        className={cn(
          "grid size-4 place-items-center rounded-full border text-[9px]",
          complete ? "border-successA-7 bg-successA-3 text-success-11" : "border-grayA-5",
        )}
      >
        {complete ? <Check className="size-2.5" /> : null}
      </span>
      {label}
    </div>
  );
}

function GithubSource({
  connected,
  loading,
  error,
  repositories,
  selectedRepository,
  search,
  onSearch,
  onSelect,
  onConnect,
  connecting,
  onRetry,
}: {
  connected: boolean;
  loading: boolean;
  error?: string;
  repositories: Repository[];
  selectedRepository: Repository | null;
  search: string;
  onSearch: (value: string) => void;
  onSelect: (repository: Repository) => void;
  onConnect: () => void;
  connecting: boolean;
  onRetry: () => void;
}) {
  return (
    <>
      <div className="flex flex-col gap-3 border-b border-grayA-5 p-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 className="text-sm font-semibold text-gray-12">Import Git repository</h2>
          <p className="mt-1 text-xs text-gray-9">Recently updated repositories appear first.</p>
        </div>
        {connected ? (
          <Button type="button" variant="outline" size="sm" onClick={onConnect} loading={connecting}>
            <Github iconSize="sm-regular" /> Add account
          </Button>
        ) : null}
      </div>

      {!connected && !loading ? (
        <div className="grid min-h-100 place-items-center p-8 text-center">
          <div className="max-w-sm">
            <span className="mx-auto grid size-12 place-items-center rounded-xl border border-grayA-5 bg-grayA-2">
              <Github iconSize="xl-medium" />
            </span>
            <h3 className="mt-4 text-base font-semibold text-gray-12">Connect GitHub</h3>
            <p className="mt-2 text-sm leading-6 text-gray-10">
              Choose exactly which repositories LayerRail can read and deploy. Access stays scoped
              to this workspace.
            </p>
            <Button
              type="button"
              variant="primary"
              className="mt-5"
              onClick={onConnect}
              loading={connecting}
            >
              <Github iconSize="sm-regular" /> Continue with GitHub
            </Button>
          </div>
        </div>
      ) : (
        <div className="p-4">
          <Input
            value={search}
            onChange={(event) => onSearch(event.target.value)}
            placeholder="Search repositories..."
            leftIcon={<Magnifier className="size-3.5 text-gray-10" />}
          />

          {loading ? (
            <div className="mt-4 space-y-2">
              {[0, 1, 2, 3].map((item) => (
                <div key={item} className="h-[68px] animate-pulse rounded-lg bg-grayA-3" />
              ))}
            </div>
          ) : error ? (
            <div className="mt-4 rounded-lg border border-errorA-5 bg-errorA-2 p-6 text-center">
              <p className="text-sm font-medium text-error-11">Repositories could not be loaded</p>
              <p className="mt-1 text-xs text-error-10">{error}</p>
              <Button type="button" variant="outline" size="sm" className="mt-4" onClick={onRetry}>
                Retry
              </Button>
            </div>
          ) : repositories.length === 0 ? (
            <div className="mt-4 rounded-lg border border-dashed border-grayA-5 p-10 text-center">
              <p className="text-sm font-medium text-gray-11">No matching repositories</p>
              <p className="mt-1 text-xs text-gray-9">Change the search or add repository access.</p>
            </div>
          ) : (
            <div className="mt-4 max-h-[540px] space-y-1 overflow-y-auto pr-1">
              {repositories.map((repository) => {
                const selected = selectedRepository?.id === repository.id;
                return (
                  <button
                    type="button"
                    key={`${repository.installationId}:${repository.id}`}
                    onClick={() => onSelect(repository)}
                    className={cn(
                      "flex w-full items-center gap-3 rounded-lg border px-3 py-3 text-left transition",
                      selected
                        ? "border-accentA-7 bg-accentA-3"
                        : "border-transparent hover:border-grayA-5 hover:bg-grayA-2",
                    )}
                  >
                    <span className="grid size-9 shrink-0 place-items-center rounded-lg border border-grayA-5 bg-gray-1">
                      <Github iconSize="md-medium" />
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="flex items-center gap-2">
                        <span className="truncate text-[13px] font-medium text-gray-12">
                          {repository.fullName}
                        </span>
                        {repository.private ? (
                          <span className="rounded-full border border-grayA-5 px-1.5 py-0.5 text-[9px] uppercase tracking-wide text-gray-9">
                            Private
                          </span>
                        ) : null}
                      </span>
                      <span className="mt-1 flex items-center gap-3 text-[11px] text-gray-9">
                        <span className="flex items-center gap-1">
                          <CodeBranch className="size-3" /> {repository.defaultBranch}
                        </span>
                        {repository.language ? <span>{repository.language}</span> : null}
                        {repository.pushedAt ? (
                          <span>Updated {relativeDate(repository.pushedAt)}</span>
                        ) : null}
                      </span>
                    </span>
                    <span
                      className={cn(
                        "rounded-md px-2.5 py-1 text-xs font-medium",
                        selected ? "bg-accent-9 text-white" : "border border-grayA-5 text-gray-11",
                      )}
                    >
                      {selected ? "Selected" : "Import"}
                    </span>
                  </button>
                );
              })}
            </div>
          )}
        </div>
      )}
    </>
  );
}

function ImageSource({ image, onImageChange }: { image: string; onImageChange: (value: string) => void }) {
  return (
    <>
      <div className="border-b border-grayA-5 p-4">
        <h2 className="text-sm font-semibold text-gray-12">Deploy a container image</h2>
        <p className="mt-1 text-xs text-gray-9">
          Start an OCI image that already contains your application and runtime.
        </p>
      </div>
      <div className="p-5">
        <Field label="Image reference" hint="Registry, repository, and tag or digest">
          <input
            className={fieldClass}
            value={image}
            onChange={(event) => onImageChange(event.target.value)}
            placeholder="ghcr.io/acme/api:latest"
            autoFocus
          />
        </Field>
        <div className="mt-5 grid gap-3 sm:grid-cols-3">
          <Capability title="No build" description="Pull the image directly" />
          <Capability title="Immutable source" description="Pin a digest for releases" />
          <Capability title="Full controls" description="Configure regions and resources" />
        </div>
        <div className="mt-5 rounded-lg border border-grayA-5 bg-grayA-2 p-4 text-xs leading-5 text-gray-10">
          The registry must be reachable by the build cluster. Pin a digest instead of a mutable
          tag when the release must be exactly reproducible.
        </div>
      </div>
    </>
  );
}

function Capability({ title, description }: { title: string; description: string }) {
  return (
    <div className="rounded-lg border border-grayA-5 p-3">
      <p className="text-xs font-medium text-gray-12">{title}</p>
      <p className="mt-1 text-[11px] leading-4 text-gray-9">{description}</p>
    </div>
  );
}

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <label className="grid gap-1.5 text-xs font-medium text-gray-11">
      <span className="flex items-center justify-between gap-2">
        {label}
        {hint ? <span className="font-normal text-gray-9">{hint}</span> : null}
      </span>
      {children}
    </label>
  );
}

function SummaryRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-3 py-1.5">
      <span className="text-gray-9">{label}</span>
      <span className="max-w-[190px] truncate text-right font-medium text-gray-11" title={value}>
        {value}
      </span>
    </div>
  );
}

function normalizeProjectSlug(value: string): string {
  return slugify(value).toLowerCase().replaceAll("_", "-").slice(0, 256);
}

function relativeDate(value: string): string {
  const timestamp = new Date(value).getTime();
  if (!Number.isFinite(timestamp)) {
    return "recently";
  }
  const days = Math.floor((Date.now() - timestamp) / 86_400_000);
  if (days <= 0) {
    return "today";
  }
  if (days === 1) {
    return "yesterday";
  }
  if (days < 30) {
    return `${days}d ago`;
  }
  return new Intl.DateTimeFormat("en", { month: "short", day: "numeric" }).format(timestamp);
}