"use client";

import { collection } from "@/lib/collections";
import { routes } from "@/lib/navigation/routes";
import { trpc } from "@/lib/trpc/client";
import { Lock, Plus, Trash } from "@unkey/icons";
import { Button, DialogContainer, Input, PageBody, toast } from "@unkey/ui";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { useAppId, useProjectData } from "../data-provider";

export default function EnvironmentsPage() {
  const appId = useAppId();
  const { projectId } = useProjectData();
  const params = useParams<{ workspaceSlug: string }>();
  const utils = trpc.useUtils();
  const environments = trpc.deploy.environment.listManaged.useQuery({ projectId, appId });
  const [createOpen, setCreateOpen] = useState(false);
  const [editEnvironment, setEditEnvironment] = useState<ManagedEnvironment | null>(null);
  const [deleteEnvironment, setDeleteEnvironment] = useState<ManagedEnvironment | null>(null);

  const refresh = async () => {
    await Promise.all([
      utils.deploy.environment.listManaged.invalidate({ projectId, appId }),
      collection.environments.utils.refetch(),
      collection.environmentSettings.utils.refetch(),
    ]);
  };

  return (
    <PageBody>
      <div className="mx-auto grid w-full max-w-5xl gap-5 py-2">
        <header className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h1 className="text-base font-semibold text-gray-12">Environments</h1>
            <p className="mt-1 max-w-2xl text-xs leading-5 text-gray-9">
              Clone production or preview settings into durable staging, QA, demo, or development
              targets. Each environment keeps independent variables, regions, domains, and releases.
            </p>
          </div>
          <Button variant="primary" size="md" onClick={() => setCreateOpen(true)}>
            <Plus /> New environment
          </Button>
        </header>

        <section className="overflow-hidden rounded-xl border border-grayA-4 bg-gray-1">
          <div className="grid grid-cols-[minmax(0,1fr)_110px_120px_250px] gap-3 border-b border-grayA-4 bg-grayA-2 px-4 py-2.5 text-[10px] font-medium uppercase tracking-wide text-gray-9">
            <span>Environment</span>
            <span>Protection</span>
            <span>Deployments</span>
            <span className="text-right">Actions</span>
          </div>
          {environments.isLoading ? (
            <div className="m-4 h-32 animate-pulse rounded-lg bg-grayA-2" />
          ) : (environments.data?.length ?? 0) === 0 ? (
            <p className="p-5 text-xs text-gray-9">No environments found for this app.</p>
          ) : (
            <div className="divide-y divide-grayA-3">
              {environments.data?.map((environment) => (
                <div
                  key={environment.id}
                  className="grid grid-cols-[minmax(0,1fr)_110px_120px_250px] items-center gap-3 px-4 py-3"
                >
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="truncate text-sm font-medium capitalize text-gray-12">
                        {environment.slug}
                      </span>
                      {environment.isDefault ? (
                        <span className="rounded-full bg-grayA-3 px-2 py-0.5 text-[10px] text-gray-9">
                          default
                        </span>
                      ) : null}
                    </div>
                    <p className="mt-0.5 truncate text-[11px] text-gray-9">
                      {environment.description || "No description"}
                    </p>
                  </div>
                  <span className="flex items-center gap-1.5 text-xs text-gray-10">
                    {environment.deleteProtection ? (
                      <><Lock className="size-3.5" /> Protected</>
                    ) : (
                      <><Lock className="size-3.5 opacity-40" /> Off</>
                    )}
                  </span>
                  <span className="text-xs tabular-nums text-gray-10">
                    {environment.deploymentCount.toLocaleString()}
                  </span>
                  <div className="flex justify-end gap-2">
                    <Link
                      href={routes.projects.apps.settings({
                        workspaceSlug: params.workspaceSlug,
                        projectId,
                        appId,
                        environmentId: environment.id,
                      })}
                      className="inline-flex h-8 items-center rounded-md border border-grayA-5 px-2.5 text-xs text-gray-11 hover:bg-grayA-2"
                    >
                      Configure
                    </Link>
                    <Link
                      href={routes.projects.apps.deployments({
                        workspaceSlug: params.workspaceSlug,
                        projectId,
                        appId,
                      })}
                      className="inline-flex h-8 items-center rounded-md border border-grayA-5 px-2.5 text-xs text-gray-11 hover:bg-grayA-2"
                    >
                      Deploy
                    </Link>
                    <button
                      type="button"
                      className="h-8 rounded-md border border-grayA-5 px-2.5 text-xs text-gray-11 hover:bg-grayA-2 disabled:cursor-not-allowed disabled:opacity-40"
                      disabled={environment.isDefault}
                      onClick={() => setEditEnvironment(environment)}
                    >
                      Edit
                    </button>
                    <button
                      type="button"
                      aria-label={`Delete ${environment.slug}`}
                      className="grid size-8 place-items-center rounded-md border border-errorA-5 text-error-11 hover:bg-errorA-2 disabled:cursor-not-allowed disabled:opacity-40"
                      disabled={environment.isDefault || environment.deleteProtection}
                      onClick={() => setDeleteEnvironment(environment)}
                    >
                      <Trash className="size-3.5" />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>
      </div>

      <CreateEnvironmentDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        environments={environments.data ?? []}
        projectId={projectId}
        appId={appId}
        onSuccess={refresh}
      />
      <EditEnvironmentDialog
        environment={editEnvironment}
        onOpenChange={(open) => !open && setEditEnvironment(null)}
        onSuccess={refresh}
      />
      <DeleteEnvironmentDialog
        environment={deleteEnvironment}
        onOpenChange={(open) => !open && setDeleteEnvironment(null)}
        onSuccess={refresh}
      />
    </PageBody>
  );
}

type ManagedEnvironment = {
  id: string;
  slug: string;
  description: string;
  deleteProtection: boolean;
  createdAt: number;
  deploymentCount: number;
  isDefault: boolean;
};

function CreateEnvironmentDialog({
  open,
  onOpenChange,
  environments,
  projectId,
  appId,
  onSuccess,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  environments: ManagedEnvironment[];
  projectId: string;
  appId: string;
  onSuccess: () => Promise<void>;
}) {
  const preferredSource =
    environments.find((environment) => environment.slug === "production") ?? environments[0];
  const [slug, setSlug] = useState("");
  const [description, setDescription] = useState("");
  const [sourceEnvironmentId, setSourceEnvironmentId] = useState("");
  const [deleteProtection, setDeleteProtection] = useState(true);
  const sourceId = sourceEnvironmentId || preferredSource?.id || "";
  const create = trpc.deploy.environment.create.useMutation({
    onSuccess: async () => {
      toast.success("Environment created", { description: `${slug} is ready for deployment.` });
      onOpenChange(false);
      setSlug("");
      setDescription("");
      setSourceEnvironmentId("");
      setDeleteProtection(true);
      await onSuccess();
    },
    onError: (error) => toast.error("Environment could not be created", { description: error.message }),
  });

  return (
    <DialogContainer
      isOpen={open}
      onOpenChange={onOpenChange}
      title="New environment"
      subTitle="Clone settings into an independent deployment target"
      footer={
        <Button
          variant="primary"
          size="xlg"
          className="w-full"
          disabled={!slug.trim() || !sourceId || create.isLoading}
          loading={create.isLoading}
          onClick={() =>
            create.mutate({
              projectId,
              appId,
              sourceEnvironmentId: sourceId,
              slug: normalizeSlug(slug),
              description,
              deleteProtection,
            })
          }
        >
          Create environment
        </Button>
      }
    >
      <div className="grid gap-5 py-2">
        <Field label="Name" hint="Used in deployment URLs and CLI commands">
          <Input value={slug} onChange={(event) => setSlug(event.target.value)} placeholder="staging" />
        </Field>
        <Field label="Description" hint="Optional context for teammates">
          <Input
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            placeholder="Release candidate before production"
          />
        </Field>
        <Field label="Clone configuration from" hint="Build, runtime, regions, and autoscaling are copied">
          <select
            className="h-9 rounded-md border border-grayA-5 bg-gray-1 px-3 text-xs text-gray-12"
            value={sourceId}
            onChange={(event) => setSourceEnvironmentId(event.target.value)}
          >
            {environments.map((environment) => (
              <option key={environment.id} value={environment.id}>{environment.slug}</option>
            ))}
          </select>
        </Field>
        <label className="flex items-start gap-2.5 rounded-lg border border-grayA-5 bg-grayA-2 p-3 text-xs text-gray-11">
          <input
            type="checkbox"
            className="mt-0.5"
            checked={deleteProtection}
            onChange={(event) => setDeleteProtection(event.target.checked)}
          />
          <span><strong>Enable delete protection.</strong> Prevent accidental removal until explicitly disabled.</span>
        </label>
        <p className="text-[11px] leading-4 text-gray-9">
          Secrets are intentionally not cloned. Add environment-specific values after creation.
        </p>
      </div>
    </DialogContainer>
  );
}

function EditEnvironmentDialog({
  environment,
  onOpenChange,
  onSuccess,
}: {
  environment: ManagedEnvironment | null;
  onOpenChange: (open: boolean) => void;
  onSuccess: () => Promise<void>;
}) {
  const [slug, setSlug] = useState("");
  const [description, setDescription] = useState("");
  useEffect(() => {
    setSlug(environment?.slug ?? "");
    setDescription(environment?.description ?? "");
  }, [environment]);
  const update = trpc.deploy.environment.update.useMutation({
    onSuccess: async () => {
      toast.success("Environment updated");
      onOpenChange(false);
      await onSuccess();
    },
    onError: (error) => toast.error("Environment could not be updated", { description: error.message }),
  });
  const protection = trpc.deploy.environment.setDeleteProtection.useMutation({
    onSuccess: async (_, variables) => {
      toast.success(variables.enabled ? "Delete protection enabled" : "Delete protection disabled");
      onOpenChange(false);
      await onSuccess();
    },
    onError: (error) => toast.error("Protection could not be updated", { description: error.message }),
  });

  const currentSlug = slug;
  const currentDescription = description;
  return (
    <DialogContainer
      isOpen={Boolean(environment)}
      onOpenChange={onOpenChange}
      title="Environment settings"
      subTitle={environment?.slug ?? ""}
      footer={
        <div className="grid w-full grid-cols-2 gap-2">
          <Button
            variant="outline"
            size="xlg"
            disabled={!environment || protection.isLoading}
            loading={protection.isLoading}
            onClick={() =>
              environment &&
              protection.mutate({ environmentId: environment.id, enabled: !environment.deleteProtection })
            }
          >
            {environment?.deleteProtection ? "Disable protection" : "Enable protection"}
          </Button>
          <Button
            variant="primary"
            size="xlg"
            disabled={!environment || !currentSlug.trim() || update.isLoading}
            loading={update.isLoading}
            onClick={() =>
              environment &&
              update.mutate({
                environmentId: environment.id,
                slug: normalizeSlug(currentSlug),
                description: currentDescription,
              })
            }
          >
            Save changes
          </Button>
        </div>
      }
    >
      <div className="grid gap-5 py-2">
        <Field label="Name">
          <Input value={currentSlug} onChange={(event) => setSlug(event.target.value)} />
        </Field>
        <Field label="Description">
          <Input value={currentDescription} onChange={(event) => setDescription(event.target.value)} />
        </Field>
        <p className="text-xs text-gray-9">
          {environment?.deploymentCount.toLocaleString()} deployments are retained under this environment.
        </p>
      </div>
    </DialogContainer>
  );
}

function DeleteEnvironmentDialog({
  environment,
  onOpenChange,
  onSuccess,
}: {
  environment: ManagedEnvironment | null;
  onOpenChange: (open: boolean) => void;
  onSuccess: () => Promise<void>;
}) {
  const [confirmation, setConfirmation] = useState("");
  const remove = trpc.deploy.environment.delete.useMutation({
    onSuccess: async () => {
      toast.info("Environment deletion started", {
        description: "Deployments, routes, domains, and settings are being removed durably.",
      });
      onOpenChange(false);
      setConfirmation("");
      await onSuccess();
    },
    onError: (error) => toast.error("Environment could not be deleted", { description: error.message }),
  });
  const valid = useMemo(
    () => Boolean(environment && confirmation === environment.slug),
    [confirmation, environment],
  );

  return (
    <DialogContainer
      isOpen={Boolean(environment)}
      onOpenChange={onOpenChange}
      title="Delete environment"
      subTitle="This permanently removes every release and route in the environment"
      footer={
        <Button
          variant="primary"
          color="danger"
          size="xlg"
          className="w-full"
          disabled={!valid || remove.isLoading}
          loading={remove.isLoading}
          onClick={() => environment && remove.mutate({ environmentId: environment.id })}
        >
          Delete environment
        </Button>
      }
    >
      <div className="grid gap-4 py-2 text-xs text-gray-11">
        <p>
          This deletes {environment?.deploymentCount.toLocaleString()} deployments plus associated
          domains, routes, variables, settings, and retained rollback targets.
        </p>
        <Field label={`Type ${environment?.slug ?? "the environment name"} to confirm`}>
          <Input value={confirmation} onChange={(event) => setConfirmation(event.target.value)} />
        </Field>
      </div>
    </DialogContainer>
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
      <span>{label}</span>
      {children}
      {hint ? <span className="text-[11px] font-normal text-gray-9">{hint}</span> : null}
    </label>
  );
}

function normalizeSlug(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}
