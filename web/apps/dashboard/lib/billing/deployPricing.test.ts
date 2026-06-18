import { describe, expect, it } from "vitest";
import {
  deployOverageCents,
  microCentsToCents,
  priceDeployUsageMicroCents,
} from "./deployPricing";

describe("priceDeployUsageMicroCents", () => {
  it("prices zero usage as zero", () => {
    expect(
      priceDeployUsageMicroCents({
        cpuSeconds: 0,
        memoryGiBHours: 0,
        diskGiBHours: 0,
        egressGiB: 0,
        activeKeys: 0,
      }),
    ).toBe(0);
  });

  it("matches the Go catalog rates per meter unit", () => {
    expect(priceDeployUsageMicroCents({ cpuSeconds: 1, memoryGiBHours: 0, diskGiBHours: 0, egressGiB: 0, activeKeys: 0 })).toBe(694);
    expect(
      priceDeployUsageMicroCents({ cpuSeconds: 0, memoryGiBHours: 1 / 3600, diskGiBHours: 0, egressGiB: 0, activeKeys: 0 }),
    ).toBe(347);
    expect(priceDeployUsageMicroCents({ cpuSeconds: 0, memoryGiBHours: 0, diskGiBHours: 0, egressGiB: 1, activeKeys: 0 })).toBe(
      5_000_000,
    );
    expect(
      priceDeployUsageMicroCents({ cpuSeconds: 0, memoryGiBHours: 0, diskGiBHours: 1 / 3600, egressGiB: 0, activeKeys: 0 }),
    ).toBe(6);
    expect(priceDeployUsageMicroCents({ cpuSeconds: 0, memoryGiBHours: 0, diskGiBHours: 0, egressGiB: 0, activeKeys: 1 })).toBe(
      200_000,
    );
  });

  it("sums meters like the spend-cap worker", () => {
    const micro = priceDeployUsageMicroCents({ cpuSeconds: 0, memoryGiBHours: 0, diskGiBHours: 0, egressGiB: 10, activeKeys: 100 });
    expect(micro).toBe(70 * 1_000_000);
  });
});

describe("deployOverageCents", () => {
  it("returns null when included credit is unknown", () => {
    expect(deployOverageCents(5_000_000_000, null)).toBeNull();
  });

  it("returns zero while gross is within credits", () => {
    // $47.11 gross, $50 included -> $0 overage
    const grossMicro = 4_711 * 1_000_000;
    expect(deployOverageCents(grossMicro, 5_000)).toBe(0);
  });

  it("returns billable amount beyond credits", () => {
    // $75 gross, $50 included -> $25 overage
    expect(deployOverageCents(7_500 * 1_000_000, 5_000)).toBe(2_500);
  });
});

describe("microCentsToCents", () => {
  it("truncates sub-cent fractions for display", () => {
    expect(microCentsToCents(4_711_499_999)).toBe(4_711);
  });
});
