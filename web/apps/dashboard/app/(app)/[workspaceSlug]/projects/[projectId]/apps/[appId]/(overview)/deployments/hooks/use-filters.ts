import {
  parseAsFilterValueArray,
  parseAsRelativeTime,
} from "@/components/logs/validation/utils/nuqs-parsers";
import { parseAsInteger, useQueryStates } from "nuqs";
import { useCallback, useEffect, useMemo, useRef } from "react";
import {
  type DeploymentListFilterField,
  type DeploymentListFilterOperator,
  type DeploymentListFilterUrlValue,
  type DeploymentListFilterValue,
  type DeploymentListQuerySearchParams,
  deploymentListFilterFieldConfig,
} from "../filters.schema";

const parseAsFilterValArray = parseAsFilterValueArray<DeploymentListFilterOperator>(["is"]);

export const queryParamsPayload = {
  status: parseAsFilterValArray,
  environment: parseAsFilterValArray,
  branch: parseAsFilterValArray,
  startTime: parseAsInteger,
  endTime: parseAsInteger,
  since: parseAsRelativeTime,
  page: parseAsInteger.withDefault(1),
} as const;

const arrayFields = ["status", "environment", "branch"] as const;
const timeFields = ["startTime", "endTime", "since"] as const;

export const useFilters = () => {
  const [searchParams, setSearchParams] = useQueryStates(queryParamsPayload, {
    history: "push",
  });

  const filters = useMemo(() => {
    const activeFilters: DeploymentListFilterValue[] = [];

    // Handle array filters
    arrayFields.forEach((field) => {
      searchParams[field]?.forEach((item) => {
        activeFilters.push({
          id: crypto.randomUUID(),
          field,
          operator: item.operator,
          value: item.value,
          metadata: deploymentListFilterFieldConfig[field].getColorClass
            ? {
                colorClass: deploymentListFilterFieldConfig[field].getColorClass(
                  item.value as string,
                ),
              }
            : undefined,
        });
      });
    });

    // Handle time filters
    ["startTime", "endTime", "since"].forEach((field) => {
      const value = searchParams[field as keyof DeploymentListQuerySearchParams];
      if (value !== null && value !== undefined) {
        activeFilters.push({
          id: crypto.randomUUID(),
          field: field as DeploymentListFilterField,
          operator: "is",
          value: value as string | number,
        });
      }
    });

    return activeFilters;
  }, [searchParams]);

  const updateFilters = useCallback(
    (newFilters: DeploymentListFilterValue[]) => {
      const newParams: Partial<DeploymentListQuerySearchParams> = Object.fromEntries([
        ...arrayFields.map((field) => [field, null]),
        ...timeFields.map((field) => [field, null]),
      ]);
      newParams.page = 1;

      const filterGroups = arrayFields.reduce(
        (acc, field) => {
          acc[field] = [];
          return acc;
        },
        {} as Record<(typeof arrayFields)[number], DeploymentListFilterUrlValue[]>,
      );

      newFilters.forEach((filter) => {
        if (arrayFields.includes(filter.field as (typeof arrayFields)[number])) {
          filterGroups[filter.field as (typeof arrayFields)[number]].push({
            value: filter.value as string,
            operator: filter.operator,
          });
        } else if (filter.field === "startTime" || filter.field === "endTime") {
          newParams[filter.field] = filter.value as number;
        } else if (filter.field === "since") {
          newParams.since = filter.value as string;
        }
      });

      // Set array filters
      arrayFields.forEach((field) => {
        newParams[field] = filterGroups[field].length > 0 ? filterGroups[field] : null;
      });

      setSearchParams(newParams);
    },
    [setSearchParams],
  );

  const removeFilter = useCallback(
    (id: string) => {
      const newFilters = filters.filter((f) => f.id !== id);
      updateFilters(newFilters);
    },
    [filters, updateFilters],
  );

  const toggleArrayFilter = useCallback(
    (field: (typeof arrayFields)[number], value: string) => {
      const existing = filters.find((f) => f.field === field && f.value === value);
      if (existing) {
        updateFilters(filters.filter((f) => f.id !== existing.id));
      } else {
        updateFilters([...filters, { field, id: crypto.randomUUID(), operator: "is", value }]);
      }
    },
    [filters, updateFilters],
  );

  const filterKey = useMemo(
    () => filters.map((f) => `${f.field}:${f.operator}:${f.value}`).join("|"),
    [filters],
  );
  const prevFilterKeyRef = useRef(filterKey);
  useEffect(() => {
    if (prevFilterKeyRef.current !== filterKey) {
      prevFilterKeyRef.current = filterKey;
      setSearchParams({ page: 1 });
    }
  }, [filterKey, setSearchParams]);

  return {
    filters,
    removeFilter,
    updateFilters,
    toggleArrayFilter,
    page: searchParams.page,
    setPage: (page: number) => setSearchParams({ page }),
  };
};
