import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import {
  DEPLOY_METER_RATES_CENTS,
  estimateMonthlyCapacity,
  formatCapacityEstimate,
} from "./cost-estimate";

describe("estimateMonthlyCapacity", () => {
  it("calculates the default one-instance monthly capacity ceiling", () => {
    const estimate = estimateMonthlyCapacity({
      cpuMillicores: 250,
      memoryMib: 256,
      storageMib: 0,
      regions: [{ replicasMin: 1, replicasMax: 1 }],
      outputs: [],
    });

    expect(estimate).toMatchObject({
      minInstances: 1,
      maxInstances: 1,
      alwaysOnResources: 1,
      excludedCronResources: 0,
    });
    expect(estimate.minCents).toBeCloseTo(684.3312, 4);
    expect(estimate.maxCents).toBeCloseTo(684.3312, 4);
    expect(formatCapacityEstimate(estimate)).toBe("$6.84/mo");
  });

  it("multiplies resource capacity across regions and autoscaling bounds", () => {
    const estimate = estimateMonthlyCapacity({
      cpuMillicores: 1_000,
      memoryMib: 1_024,
      storageMib: 1_024,
      regions: [
        { replicasMin: 1, replicasMax: 2 },
        { replicasMin: 2, replicasMax: 4 },
      ],
      outputs: [{ kind: "container" }, { kind: "worker" }],
    });

    expect(estimate.minInstances).toBe(6);
    expect(estimate.maxInstances).toBe(12);
    expect(estimate.maxCents).toBeCloseTo(estimate.minCents * 2, 8);
    expect(formatCapacityEstimate(estimate)).toMatch(/^\$\d+\.\d{2}–\$\d+\.\d{2}\/mo$/);
  });

  it("excludes static output and workload-dependent cron execution", () => {
    const estimate = estimateMonthlyCapacity({
      cpuMillicores: 250,
      memoryMib: 256,
      storageMib: 0,
      regions: [{ replicasMin: 1, replicasMax: 4 }],
      outputs: [{ kind: "static" }, { kind: "cron" }],
    });

    expect(estimate).toMatchObject({
      minCents: 0,
      maxCents: 0,
      minInstances: 0,
      maxInstances: 0,
      alwaysOnResources: 0,
      excludedCronResources: 1,
    });
  });

  it("stays pinned to the authoritative Go Stripe catalog rates", () => {
    const catalog = readFileSync(
      resolve(process.cwd(), "../../../tools/pricing/catalog.go"),
      "utf8",
    );

    expect(catalog).toContain(
      `Key: "cpu_seconds", DisplayName: "CPU seconds", EventName: "cpu_seconds", Aggregation: AggregationLast, CentsPerUnit: ${DEPLOY_METER_RATES_CENTS.cpuSecond}`,
    );
    expect(catalog).toContain(
      `Key: "memory_gib_seconds", DisplayName: "Memory GiB-seconds", EventName: "memory_gib_seconds", Aggregation: AggregationLast, CentsPerUnit: ${DEPLOY_METER_RATES_CENTS.memoryGiBSecond}`,
    );
    expect(catalog).toContain(
      `Key: "disk_gib_seconds", DisplayName: "Disk GiB-seconds", EventName: "disk_gib_seconds", Aggregation: AggregationLast, CentsPerUnit: ${DEPLOY_METER_RATES_CENTS.diskGiBSecond}`,
    );
  });
});
