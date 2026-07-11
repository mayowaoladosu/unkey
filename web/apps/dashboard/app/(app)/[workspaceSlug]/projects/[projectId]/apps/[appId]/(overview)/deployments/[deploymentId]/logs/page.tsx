"use client";

import { trpc } from "@/lib/trpc/client";
import { PageBody } from "@unkey/ui";
import { useState } from "react";
import { useDeployment } from "../layout-provider";

export default function DeploymentLogsPage() {
  const { deployment } = useDeployment();
  const [resourceId, setResourceId] = useState("");
  const resources = trpc.deploy.deployment.resources.useQuery({
    deploymentId: deployment.id,
    projectId: deployment.projectId,
  });
  const logs = trpc.deploy.deployment.runtimeLogs.useQuery({
    deploymentId: deployment.id,
    resourceId: resourceId || undefined,
    limit: 100,
  });
  const events = trpc.deploy.deployment.instanceEvents.useQuery({
    projectId: deployment.projectId,
    deploymentId: deployment.id,
    resourceIds: resourceId ? [resourceId] : [],
    limit: 100,
  });

  return (
    <PageBody>
      <div className="grid gap-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-sm font-medium text-gray-12">Logs & lifecycle events</h2>
            <p className="mt-1 text-xs text-gray-9">
              Runtime output and container transitions are labeled by deployment resource.
            </p>
          </div>
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
        </div>
        <div className="grid gap-4 xl:grid-cols-2">
          <TimelineCard
            title="Runtime logs"
            pending={logs.isPending}
            empty="No runtime logs in this deployment yet."
            rows={(logs.data?.logs ?? []).map((log) => ({
              key: `${log.time}:${log.instance_id}:${log.message}`,
              time: log.time,
              badge: log.severity,
              resource: log.resourceName || log.resourceKind || "legacy",
              message: log.message,
            }))}
          />
          <TimelineCard
            title="Lifecycle events"
            pending={events.isPending}
            empty="No lifecycle events in this deployment yet."
            rows={(events.data?.events ?? []).map((event) => ({
              key: `${event.time}:${event.eventFingerprint}`,
              time: event.time,
              badge: event.eventKind,
              resource: event.resourceName || event.resourceKind || "legacy",
              message: event.reason || event.message || event.eventKind,
            }))}
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
}: {
  title: string;
  pending: boolean;
  empty: string;
  rows: Array<{ key: string; time: number; badge: string; resource: string; message: string }>;
}) {
  return (
    <section className="overflow-hidden rounded-lg border border-grayA-4 bg-gray-1">
      <header className="border-b border-grayA-4 px-4 py-3 text-xs font-medium text-gray-12">
        {title}
      </header>
      {pending ? (
        <div className="m-4 h-24 animate-pulse rounded bg-grayA-2" />
      ) : rows.length === 0 ? (
        <p className="p-4 text-xs text-gray-9">{empty}</p>
      ) : (
        <div className="max-h-[65dvh] divide-y divide-grayA-3 overflow-y-auto">
          {rows.map((row) => (
            <div key={row.key} className="grid grid-cols-[72px_90px_80px_minmax(0,1fr)] gap-2 px-3 py-2 text-[10px]">
              <time className="font-mono text-gray-8">{new Date(row.time).toLocaleTimeString()}</time>
              <span className="truncate text-gray-9">{row.resource}</span>
              <span className="w-fit rounded-full bg-grayA-3 px-2 py-0.5 text-gray-10">{row.badge}</span>
              <span className="truncate font-mono text-gray-11" title={row.message}>{row.message}</span>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
