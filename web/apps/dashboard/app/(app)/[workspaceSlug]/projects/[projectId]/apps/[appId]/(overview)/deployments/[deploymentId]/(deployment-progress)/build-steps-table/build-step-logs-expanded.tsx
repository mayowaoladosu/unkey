"use client";

import { isDeploymentInFlight } from "@/lib/collections/deploy/deployment-status";
import { trpc } from "@/lib/trpc/client";
import { cn } from "@/lib/utils";
import { TimestampInfo, toast } from "@unkey/ui";
import { useMemo, useState } from "react";
import { Fragment } from "react/jsx-runtime";
import { useDeployment } from "../../layout-provider";
import { TruncatedCell } from "../truncated-cell";
import type { BuildStepRow } from "./columns";

const PAGE_SIZE = 200;
const DOWNLOAD_PAGE_SIZE = 500;

export function BuildStepLogsExpanded({
  deploymentId,
  step,
}: {
  deploymentId: string;
  step: BuildStepRow;
}) {
  const { deployment } = useDeployment();
  const utils = trpc.useUtils();
  const inFlight = isDeploymentInFlight(deployment.status);
  const [downloading, setDownloading] = useState(false);
  const logsQuery = trpc.deploy.deployment.buildStepLogs.useInfiniteQuery(
    {
      deploymentId,
      stepId: step.step_id,
      limit: PAGE_SIZE,
    },
    {
      getNextPageParam: (lastPage) => lastPage.nextCursor ?? undefined,
      refetchInterval: inFlight ? 1_500 : false,
      refetchOnWindowFocus: inFlight,
    },
  );

  const logs = useMemo(
    () => (logsQuery.data?.pages.flatMap((page) => page.logs) ?? []).reverse(),
    [logsQuery.data],
  );

  const downloadFullLog = async () => {
    setDownloading(true);
    try {
      const rows: Array<{ time: number; message: string }> = [];
      let cursor = 0;

      for (;;) {
        const page = await utils.deploy.deployment.buildStepLogs.fetch({
          deploymentId,
          stepId: step.step_id,
          cursor,
          limit: DOWNLOAD_PAGE_SIZE,
        });
        rows.push(...page.logs);
        if (page.nextCursor === null || page.nextCursor <= cursor) {
          break;
        }
        cursor = page.nextCursor;
      }

      const transcript = rows
        .reverse()
        .map((log) => `${new Date(log.time).toISOString()}  ${log.message.replace(/\r?\n$/, "")}`)
        .join("\n");
      const blob = new Blob([transcript, transcript ? "\n" : ""], { type: "text/plain" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `${deploymentId}-${step.step_id.slice(0, 12)}.log`;
      document.body.append(link);
      link.click();
      link.remove();
      setTimeout(() => URL.revokeObjectURL(url), 1_000);
    } catch (error) {
      toast.error("Could not download build log", {
        description: error instanceof Error ? error.message : "An unexpected error occurred",
      });
    } finally {
      setDownloading(false);
    }
  };

  if (logsQuery.isLoading && logs.length === 0) {
    return (
      <tr>
        <td colSpan={6} className="px-8 py-4 text-sm text-gray-11">
          Loading build output…
        </td>
      </tr>
    );
  }

  if (logsQuery.isError) {
    return (
      <tr>
        <td colSpan={6} className="px-8 py-4 text-sm text-error-11">
          Build output could not be loaded. {logsQuery.error.message}
        </td>
      </tr>
    );
  }

  if (logs.length === 0) {
    return (
      <tr>
        <td colSpan={6} className="px-8 py-4 text-sm text-gray-11">
          No logs available for this step
        </td>
      </tr>
    );
  }

  const isError = Boolean(step.error);
  const borderClass = isError ? "border-error-7" : "border-accent-7";
  const bgClass = isError ? "bg-error-2" : "";

  return (
    <>
      <tr>
        <td colSpan={6} className={cn("border-l-2 px-4 py-2", borderClass, bgClass)}>
          <div className="flex items-center justify-between gap-3 text-[11px] text-gray-10">
            <span>
              {logs.length.toLocaleString()} lines loaded
              {inFlight ? " · refreshing live" : ""}
            </span>
            <div className="flex items-center gap-3">
              {logsQuery.hasNextPage ? (
                <button
                  type="button"
                  className="cursor-pointer font-medium text-gray-11 hover:text-gray-12 disabled:cursor-not-allowed disabled:opacity-50"
                  disabled={logsQuery.isFetchingNextPage}
                  onClick={(event) => {
                    event.stopPropagation();
                    void logsQuery.fetchNextPage();
                  }}
                >
                  {logsQuery.isFetchingNextPage ? "Loading…" : "Load earlier"}
                </button>
              ) : null}
              <button
                type="button"
                className="cursor-pointer font-medium text-gray-11 hover:text-gray-12 disabled:cursor-not-allowed disabled:opacity-50"
                disabled={downloading}
                onClick={(event) => {
                  event.stopPropagation();
                  void downloadFullLog();
                }}
              >
                {downloading ? "Preparing download…" : "Download full log"}
              </button>
            </div>
          </div>
        </td>
      </tr>
      {logs.map((log, idx) => (
        <Fragment key={`row-group-${log.time}-${idx}`}>
          <tr key={`spacer-${log.time}-${idx}`} style={{ height: "4px" }}>
            <td colSpan={6} className={cn("border-l-2 p-0", borderClass, bgClass)} />
          </tr>
          <tr key={`${log.time}-${idx}`}>
            <td className={cn("border-l-2 py-0", borderClass, bgClass)} />
            <td className={cn("py-0", bgClass)}>
              <TimestampInfo
                displayType="local_hours_with_millis"
                value={log.time}
                className="font-mono text-xs text-grayA-9 hover:underline decoration-dotted"
              />
            </td>
            <td className={cn("py-0", bgClass)} />
            <td colSpan={3} className={cn("h-[26px] py-px", bgClass)}>
              <TruncatedCell text={log.message} />
            </td>
          </tr>
        </Fragment>
      ))}
    </>
  );
}
