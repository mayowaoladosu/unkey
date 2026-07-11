import { describe, expect, it } from "vitest";
import { lifecycleEventRowKey } from "../lifecycle-event-row-key";

const sharedIncident = {
  time: 1_783_745_531_000,
  eventFingerprint: "a78aff4d",
  containerName: "deployment",
  restartCount: 0,
  eventKind: "running",
};

describe("lifecycleEventRowKey", () => {
  it("distinguishes simultaneous events with the same incident fingerprint", () => {
    const web = lifecycleEventRowKey({ ...sharedIncident, podUid: "pod-web" });
    const fn = lifecycleEventRowKey({ ...sharedIncident, podUid: "pod-function" });

    expect(web).not.toBe(fn);
  });

  it("is stable for the same container life", () => {
    const event = { ...sharedIncident, podUid: "pod-web" };

    expect(lifecycleEventRowKey(event)).toBe(lifecycleEventRowKey(event));
  });
});