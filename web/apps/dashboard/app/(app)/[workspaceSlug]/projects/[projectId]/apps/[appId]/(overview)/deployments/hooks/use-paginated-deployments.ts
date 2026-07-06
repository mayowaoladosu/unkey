import type { Deployment } from "@/lib/collections/deploy/deployments";
import type { Environment } from "@/lib/collections/deploy/environments";
import { parseDuration } from "@/lib/duration";
import { trpc } from "@/lib/trpc/client";
import { useCallback, useEffect, useMemo } from "react";
import { useProjectData } from "../../data-provider";
import { useFilters } from "./use-filters";

const PAGE_SIZE = 50;

function extractArrayFilterValues(
  filters: ReturnType<typeof useFilters>["filters"],
  field: "status" | "environment" | "branch",
) {
  return filters.flatMap((f) =>
    f.field === field && typeof f.value === "string" ? [f.value] : [],
  );
}

export function usePaginatedDeployments({ enabled = true }: { enabled?: boolean } = {}) {
  const { projectId, appId, environments } = useProjectData();
  const { filters, page, setPage } = useFilters();

  const environmentMap = useMemo(() => {
    const map = new Map<string, Environment>();
    for (const env of environments) {
      map.set(env.id, env);
    }
    return map;
  }, [environments]);

  const startTimeRaw = filters.find((f) => f.field === "startTime")?.value;
  const startTime = typeof startTimeRaw === "number" ? startTimeRaw : undefined;
  const endTimeRaw = filters.find((f) => f.field === "endTime")?.value;
  const endTime = typeof endTimeRaw === "number" ? endTimeRaw : undefined;
  const sinceRaw = filters.find((f) => f.field === "since")?.value;
  const since = typeof sinceRaw === "string" ? sinceRaw : undefined;
  const sinceMs = useMemo(() => (since ? Date.now() - parseDuration(since) : undefined), [since]);

  const effectiveStartTime = sinceMs ?? startTime;
  const statuses = extractArrayFilterValues(filters, "status");
  const branches = extractArrayFilterValues(filters, "branch");
  const environmentSlugs = extractArrayFilterValues(filters, "environment");

  const queryParams = useMemo(
    () => ({
      projectId,
      appId,
      page,
      limit: PAGE_SIZE,
      ...(effectiveStartTime !== undefined && { startTime: effectiveStartTime }),
      ...(endTime !== undefined && { endTime }),
      ...(statuses.length > 0 && { statuses }),
      ...(branches.length > 0 && { branches }),
      ...(environmentSlugs.length > 0 && { environmentSlugs }),
    }),
    [projectId, appId, page, effectiveStartTime, endTime, statuses, branches, environmentSlugs],
  );

  const { data, isLoading, isFetching } = trpc.deploy.deployment.listPaginated.useQuery(
    queryParams,
    {
      enabled,
      keepPreviousData: true,
      refetchInterval: enabled ? 20_000 : false,
    },
  );

  const paginatedData = data ?? {
    deployments: [] as Deployment[],
    total: 0,
    page: 1,
    pageSize: PAGE_SIZE,
    totalPages: 1,
  };

  const deployments = useMemo(() => {
    return paginatedData.deployments.map((deployment) => ({
      deployment,
      environment: environmentMap.get(deployment.environmentId),
    }));
  }, [paginatedData.deployments, environmentMap]);

  const totalPages = paginatedData.totalPages;
  const totalCount = paginatedData.total;

  useEffect(() => {
    if (data && page > totalPages) {
      setPage(totalPages);
    }
  }, [data, page, totalPages, setPage]);

  const onPageChange = useCallback(
    (newPage: number) => {
      if (newPage < 1 || newPage > totalPages) {
        return;
      }
      setPage(newPage);
    },
    [totalPages, setPage],
  );

  return {
    deployments: {
      isLoading,
      isFetching,
      data: deployments,
    },
    page,
    pageSize: PAGE_SIZE,
    totalPages,
    totalCount,
    onPageChange,
  };
}
