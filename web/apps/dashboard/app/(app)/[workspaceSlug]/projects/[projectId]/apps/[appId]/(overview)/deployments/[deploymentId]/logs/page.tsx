"use client";

import { isDeploymentInFlight } from "@/lib/collections/deploy/deployment-status";
import { trpc } from "@/lib/trpc/client";
import { cn } from "@/lib/utils";
import { PageBody, toast } from "@unkey/ui";
import { type ReactNode, useEffect, useMemo, useState } from "react";
import { DeploymentBuildStepsTable } from "../(deployment-progress)/build-steps-table/deployment-build-steps-table";
import { useDeployment } from "../layout-provider";
import { lifecycleEventRowKey } from "../lifecycle-event-row-key";

const RUNTIME_PAGE_SIZE = 100;
const DOWNLOAD_PAGE_SIZE = 1_000;

export default function DeploymentLogsPage() {
  const { deployment } = useDeployment();
  const utils = trpc.useUtils();
  const [resourceId, setResourceId] = useState("");
  const [page, setPage] = useState(1);
  const [anchorTime, setAnchorTime] = useState(Date.now);
  const [isLive, setIsLive] = useState(() => isDeploymentInFlight(deployment.status));
  const [downloading, setDownloading] = useState(false);

  const resources = trpc.deploy.deployment.resources.useQuery({
    deploymentId: deployment.id,
    projectId: deployment.projectId,
  });
  const buildSteps = trpc.deploy.deployment.buildSteps.useQuery(
    { deploymentId: deployment.id },
    { refetchInterval: isLive ? 1_000 : false },
  );
  const liveLogs = trpc.deploy.deployment.runtimeLogs.useQuery(
    {
      deploymentId: deployment.id,
      resourceId: resourceId || undefined,
      limit: RUNTIME_PAGE_SIZE,
    },
    {
      enabled: isLive,
      refetchInterval: isLive ? 2_000 : false,
      refetchOnWindowFocus: isLive,
    },
  );

  const historyInput = useMemo(
    () => ({
      projectId: deployment.projectId,
      appId: deployment.appId,
      deploymentId: [deployment.id],
      resourceId: resourceId ? [resourceId] : [],
      resourceKind: [],
      environmentId: {
        filters: [{ operator: "is" as const, value: deployment.environmentId }],
      },
      limit: RUNTIME_PAGE_SIZE,
      page,
      includeTotal: true,
      startTime: deployment.createdAt,
      endTime: anchorTime,
      since: null,
      severity: null,
      region: null,
      message: null,
      instanceId: null,
    }),
    [
      anchorTime,
      deployment.appId,
      deployment.createdAt,
      deployment.environmentId,
      deployment.id,
      deployment.projectId,
      page,
      resourceId,
    ],
  );
  const historyLogs = trpc.deploy.runtimeLogs.query.useQuery(historyInput, {
    enabled: !isLive,
    keepPreviousData: true,
    refetchOnWindowFocus: false,
  });
  const events = trpc.deploy.deployment.instanceEvents.useQuery(
    {
      projectId: deployment.projectId,
      deploymentId: deployment.id,
      resourceIds: resourceId ? [resourceId] : [],
      limit: 100,
    },
    { refetchInterval: isLive ? 2_000 : false },
  );

  useEffect(() => {
    setPage(1);
    setAnchorTime(Date.now());
  }, [resourceId]);

  const runtimeRows = isLive
    ? (liveLogs.data?.logs ?? []).map((log, index) => ({
        key: `${log.time}:${log.instance_id}:${log.message}:${index}`,
        time: log.time,
        badge: log.severity,
        resource: log.resourceName || log.resourceKind || "legacy",
        message: log.message,
      }))
    : (historyLogs.data?.logs ?? []).map((log, index) => ({
        key: `${log.time}:${log.instance_id}:${log.message}:${index}`,
        time: log.time,
        badge: log.severity,
        resource: log.resource_name || log.resource_kind || "legacy",
        message: log.message,
      }));
  const totalPages = Math.max(
    1,
    Math.ceil((historyLogs.data?.total ?? 0) / RUNTIME_PAGE_SIZE),
  );

  const downloadRuntimeLogs = async () => {
    setDownloading(true);
    try {
      const downloadInput = {
        ...historyInput,
        limit: DOWNLOAD_PAGE_SIZE,
        page: 1,
        includeTotal: true,
        endTime: Date.now(),
      };
      const firstPage = await utils.deploy.runtimeLogs.query.fetch(downloadInput);
      const rows = [...firstPage.logs];
      const pageCount = Math.ceil(firstPage.total / DOWNLOAD_PAGE_SIZE);
      for (let nextPage = 2; nextPage <= pageCount; nextPage++) {
        const result = await utils.deploy.runtimeLogs.query.fetch({
          ...downloadInput,
          page: nextPage,
          includeTotal: false,
        });
        rows.push(...result.logs);
      }

      const transcript = rows
        .reverse()
        .map(
          (log) =>
            `${new Date(log.time).toISOString()}  ${log.severity.padEnd(7)}  ${log.resource_name || log.resource_kind || "legacy"}  ${log.region}  ${log.instance_id}  ${log.message.replace(/\r?\n$/, "")}`,
        )
        .join("\n");
      const blob = new Blob([transcript, transcript ? "\n" : ""], { type: "text/plain" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `${deployment.id}-runtime.log`;
      document.body.append(link);
      link.click();
      link.remove();
      setTimeout(() => URL.revokeObjectURL(url), 1_000);
    } catch (error) {
      toast.error("Could not download runtime logs", {
        description: error instanceof Error ? error.message : "An unexpected error occurred",
      });
    } finally {
      setDownloading(false);
    }
  };

  const refresh = () => {
    if (isLive) {
      void Promise.all([liveLogs.refetch(), buildSteps.refetch(), events.refetch()]);
      return;
    }
    setAnchorTime(Date.now());
  };

  return (
    <PageBody>
      <div className="grid gap-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-sm font-medium text-gray-12">Logs & lifecycle events</h2>
            <p className="mt-1 text-xs text-gray-9">
              Complete build output, runtime logs, and container transitions for this release.
            </p>
          </div>
          <div className="flex items-center gap-2">
            <select
              className="h-9 min-w-48 rounded-md border border-grayA-5 bg-gray-1 px-3 text-xs text-gray-12"
              value={resourceId}
              onChange={(event) => setResourceId(event.target.value)}
            >
              <option value="">All resources</option>
              {(resources.data?.resources ?? [])
                .filter((resource) => resource.kind !== "static")
                .map((resource) => (
                  <option key={resource.id} value={resource.id}>
                    {resource.name} · {resource.kind}
                  </option>
                ))}
            </select>
            <button
              type="button"
              className={cn(
                "h-9 rounded-md border px-3 text-xs font-medium transition-colors",
                isLive
                  ? "border-successA-6 bg-successA-3 text-success-11"
                  : "border-grayA-5 bg-gray-1 text-gray-11 hover:bg-grayA-2",
              )}
              onClick={() => {
                setIsLive((value) => !value);
                setPage(1);
                setAnchorTime(Date.now());
              }}
            >
              {isLive ? "● Live" : "Resume live"}
            </button>
            <button
              type="button"
              className="h-9 rounded-md border border-grayA-5 bg-gray-1 px-3 text-xs font-medium text-gray-11 hover:bg-grayA-2"
              onClick={refresh}
            >
              Refresh
            </button>
          </div>
        </div>

        <section className="overflow-hidden rounded-lg border border-grayA-4 bg-gray-1">
          <header className="flex items-center justify-between border-b border-grayA-4 px-4 py-3">
            <div>
              <h3 className="text-xs font-medium text-gray-12">Build output</h3>
              <p className="mt-1 text-[11px] text-gray-9">
                Expand a step to inspect, page through, or download its complete transcript.
              </p>
            </div>
            {isLive ? <span className="text-[11px] text-success-11">Refreshing live</span> : null}
          </header>
          <DeploymentBuildStepsTable
            deploymentId={deployment.id}
            steps={buildSteps.data?.steps ?? []}
            isLoading={buildSteps.isLoading}
            fixedHeight={360}
          />
        </section>

        <div className="grid gap-4 xl:grid-cols-2">
          <TimelineCard
            title="Runtime logs"
            pending={isLive ? liveLogs.isLoading : historyLogs.isLoading}
            empty="No runtime logs in this deployment yet."
            rows={runtimeRows}
            headerActions={
              <button
                type="button"
                className="text-[11px] font-medium text-gray-10 hover:text-gray-12 disabled:opacity-50"
                disabled={downloading}
                onClick={() => void downloadRuntimeLogs()}
              >
                {downloading ? "Preparing…" : "Download full log"}
              </button>
            }
            footer={
              isLive ? (
                <span>Showing the latest {RUNTIME_PAGE_SIZE} lines · live</span>
              ) : (
                <div className="flex w-full items-center justify-between gap-3">
                  <span>
                    Page {page} of {totalPages} · {(historyLogs.data?.total ?? 0).toLocaleString()} lines
                  </span>
                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      className="font-medium text-gray-11 disabled:opacity-40"
                      disabled={page <= 1 || historyLogs.isFetching}
                      onClick={() => setPage((value) => Math.max(1, value - 1))}
                    >
                      Newer
                    </button>
                    <button
                      type="button"
                      className="font-medium text-gray-11 disabled:opacity-40"
                      disabled={page >= totalPages || historyLogs.isFetching}
                      onClick={() => setPage((value) => Math.min(totalPages, value + 1))}
                    >
                      Older
                    </button>
                  </div>
                </div>
              )
            }
          />
          <TimelineCard
            title="Lifecycle events"
            pending={events.isLoading}
            empty="No lifecycle events in this deployment yet."
            rows={(events.data?.events ?? []).map((event) => ({
              key: lifecycleEventRowKey(event),
              time: event.time,
              badge: event.eventKind,
              resource: event.resourceName || event.resourceKind || "legacy",
              message: event.reason || event.message || event.eventKind,
            }))}
            footer={<span>Latest 100 container transitions</span>}
          />
        </div>
      </div>
    </PageBody>
  );
}

function TimelineCard({
  title,
  pending,
  empty,
  rows,
  headerActions,
  footer,
}: {
  title: string;
  pending: boolean;
  empty: string;
  rows: Array<{ key: string; time: number; badge: string; resource: string; message: string }>;
  headerActions?: ReactNode;
  footer?: ReactNode;
}) {
  return (
    <section className="overflow-hidden rounded-lg border border-grayA-4 bg-gray-1">
      <header className="flex items-center justify-between gap-3 border-b border-grayA-4 px-4 py-3 text-xs font-medium text-gray-12">
        <span>{title}</span>
        {headerActions}
      </header>
      {pending ? (
        <div className="m-4 h-24 animate-pulse rounded bg-grayA-2" />
      ) : rows.length === 0 ? (
        <p className="p-4 text-xs text-gray-9">{empty}</p>
      ) : (
        <div className="max-h-[65dvh] divide-y divide-grayA-3 overflow-y-auto">
          {rows.map((row) => (
            <div
              key={row.key}
              className="grid grid-cols-[72px_90px_80px_minmax(0,1fr)] gap-2 px-3 py-2 text-[10px]"
            >
              <time className="font-mono text-gray-8">
                {new Date(row.time).toLocaleTimeString()}
              </time>
              <span className="truncate text-gray-9">{row.resource}</span>
              <span className="w-fit rounded-full bg-grayA-3 px-2 py-0.5 text-gray-10">
                {row.badge}
              </span>
              <span className="truncate font-mono text-gray-11" title={row.message}>
                {row.message}
              </span>
            </div>
          ))}
        </div>
      )}
      {footer ? (
        <footer className="flex min-h-10 items-center border-t border-grayA-4 px-4 py-2 text-[11px] text-gray-9">
          {footer}
        </footer>
      ) : null}
    </section>
  );
}
