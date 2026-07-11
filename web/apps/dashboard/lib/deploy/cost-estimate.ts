export const DEPLOY_METER_RATES_CENTS = {
  cpuSecond: 0.0006944,
  memoryGiBSecond: 0.0003472,
  diskGiBSecond: 0.000006,
} as const;

const BILLING_HOURS_PER_MONTH = 730;
const SECONDS_PER_HOUR = 3_600;
const ALWAYS_ON_OUTPUT_KINDS = new Set(["container", "function", "worker"]);

type RegionCapacity = {
  replicasMin: number;
  replicasMax: number;
};

type OutputCapacity = {
  kind: "container" | "static" | "function" | "worker" | "cron";
};

export type MonthlyCapacityEstimateInput = {
  cpuMillicores: number;
  memoryMib: number;
  storageMib: number;
  regions: RegionCapacity[];
  outputs: OutputCapacity[];
};

export type MonthlyCapacityEstimate = {
  minCents: number;
  maxCents: number;
  minInstances: number;
  maxInstances: number;
  alwaysOnResources: number;
  excludedCronResources: number;
};

/**
 * Calculates the monthly full-utilization ceiling for configured always-on
 * capacity. Rates mirror the Stripe catalog in tools/pricing/catalog.go.
 *
 * CPU and memory billing use observed utilization, so this is deliberately a
 * ceiling rather than a promised invoice. Public egress and cron execution are
 * workload-dependent and excluded. Disk is allocated-capacity billing.
 */
export function estimateMonthlyCapacity(
  input: MonthlyCapacityEstimateInput,
): MonthlyCapacityEstimate {
  const minReplicasPerResource = input.regions.reduce(
    (sum, region) => sum + Math.max(0, region.replicasMin),
    0,
  );
  const maxReplicasPerResource = input.regions.reduce(
    (sum, region) => sum + Math.max(0, region.replicasMax),
    0,
  );
  const alwaysOnResources =
    input.outputs.length === 0
      ? 1
      : input.outputs.filter((output) => ALWAYS_ON_OUTPUT_KINDS.has(output.kind)).length;
  const excludedCronResources = input.outputs.filter((output) => output.kind === "cron").length;
  const minInstances = minReplicasPerResource * alwaysOnResources;
  const maxInstances = maxReplicasPerResource * alwaysOnResources;

  const seconds = BILLING_HOURS_PER_MONTH * SECONDS_PER_HOUR;
  const centsPerInstance =
    (Math.max(0, input.cpuMillicores) / 1_000) *
      seconds *
      DEPLOY_METER_RATES_CENTS.cpuSecond +
    (Math.max(0, input.memoryMib) / 1_024) *
      seconds *
      DEPLOY_METER_RATES_CENTS.memoryGiBSecond +
    (Math.max(0, input.storageMib) / 1_024) *
      seconds *
      DEPLOY_METER_RATES_CENTS.diskGiBSecond;

  return {
    minCents: centsPerInstance * minInstances,
    maxCents: centsPerInstance * maxInstances,
    minInstances,
    maxInstances,
    alwaysOnResources,
    excludedCronResources,
  };
}

export function formatCapacityEstimate(estimate: MonthlyCapacityEstimate): string {
  const format = (cents: number) =>
    new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: "USD",
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(cents / 100);

  if (Math.abs(estimate.maxCents - estimate.minCents) < 0.005) {
    return `${format(estimate.minCents)}/mo`;
  }
  return `${format(estimate.minCents)}–${format(estimate.maxCents)}/mo`;
}
