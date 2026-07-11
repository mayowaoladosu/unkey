type LifecycleEventIdentity = {
  time: number;
  eventFingerprint: string;
  podUid: string;
  containerName: string;
  restartCount: number;
  eventKind: string;
};

// eventFingerprint groups equivalent incidents across workloads. It is not a
// row identifier: resources started in the same millisecond can intentionally
// share one. Include the Kubernetes container-life identity so React can keep
// simultaneous resource events distinct and stable across refetches.
export function lifecycleEventRowKey(event: LifecycleEventIdentity): string {
  return [
    event.time,
    event.eventFingerprint,
    event.podUid,
    event.containerName,
    event.restartCount,
    event.eventKind,
  ].join(":");
}