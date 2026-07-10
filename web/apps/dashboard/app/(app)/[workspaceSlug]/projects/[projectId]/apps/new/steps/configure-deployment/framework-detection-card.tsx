"use client";

import {
  useAppId,
  useProjectData,
} from "@/app/(app)/[workspaceSlug]/projects/[projectId]/apps/[appId]/(overview)/data-provider";
import { collection } from "@/lib/collections";
import { hasFrameworkDefaults } from "@/lib/deploy/framework-defaults";
import { trpc } from "@/lib/trpc/client";
import { Button, toast } from "@unkey/ui";
import { type ReactNode, useEffect, useRef } from "react";

export const FrameworkDetectionCard = ({
  onDefaultsApplied,
}: {
  onDefaultsApplied: () => void;
}) => {
  const { projectId } = useProjectData();
  const appId = useAppId();
  const detectionMutation = trpc.github.detectFramework.useMutation();
  const detectionStarted = useRef(false);

  useEffect(() => {
    if (detectionStarted.current) {
      return;
    }
    detectionStarted.current = true;
    detectionMutation.mutate({ projectId, appId });
  }, [appId, projectId, detectionMutation]);

  const applyDefaults = trpc.deploy.environmentSettings.build.applyFrameworkDefaults.useMutation({
    onSuccess: async () => {
      try {
        await collection.environmentSettings.utils.refetch();
      } catch {
        // Remounting below creates a fresh settings subscription even if this
        // eager cache refresh fails.
      }
      // The server mutation already committed. Always remount the settings
      // panel so a cache refresh failure cannot leave stale form state.
      onDefaultsApplied();

      try {
        await detectionMutation.mutateAsync({ projectId, appId });
      } catch {
        // Defaults are already persisted and the settings panel has been
        // refreshed. A failed advisory re-detection must not hide success.
      }

      toast.success("Detected settings applied", {
        description: "The settings remain editable and will be used for the next deployment.",
      });
    },
    onError: (error) => {
      toast.error("Unable to apply detected settings", { description: error.message });
    },
  });

  if (detectionMutation.isError) {
    return (
      <DetectionShell>
        <p className="text-[13px] font-medium text-gray-12">Automatic detection unavailable</p>
        <p className="text-xs text-gray-10">
          Configure the build settings below manually. {detectionMutation.error.message}
        </p>
      </DetectionShell>
    );
  }

  if (!detectionMutation.data || detectionMutation.isLoading) {
    return (
      <DetectionShell>
        <p className="text-[13px] font-medium text-gray-12">Analyzing repository...</p>
        <p className="text-xs text-gray-10">Reading framework and build signals from GitHub.</p>
      </DetectionShell>
    );
  }

  const result = detectionMutation.data;
  if (!result || result.status === "unavailable") {
    return (
      <DetectionShell>
        <p className="text-[13px] font-medium text-gray-12">Automatic detection unavailable</p>
        <p className="text-xs text-gray-10">
          {result?.reason ?? "Configure the build settings below manually."}
        </p>
      </DetectionShell>
    );
  }

  const { detection, defaults } = result;
  const canApply = hasFrameworkDefaults(defaults);
  const detectedName = detection.preset?.name ?? "No single framework selected";
  const defaultsList = [
    defaults.rootDirectory && defaults.rootDirectory !== "."
      ? `Root: ${defaults.rootDirectory}`
      : null,
    defaults.dockerfile ? `Dockerfile: ${defaults.dockerfile}` : null,
    defaults.buildCommand ? `Build: ${defaults.buildCommand}` : null,
  ].filter((value): value is string => value !== null);

  return (
    <DetectionShell>
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <p className="text-[13px] font-semibold text-gray-12">{detectedName}</p>
            <DetectionPill>{detection.confidence} confidence</DetectionPill>
            <DetectionPill>{detection.buildStrategy}</DetectionPill>
            {detection.packageManager ? (
              <DetectionPill>{detection.packageManager}</DetectionPill>
            ) : null}
          </div>
          <p className="mt-1 text-xs text-gray-10">
            Detected from {result.source.repositoryFullName} on {result.source.branch}. The build
            worker verifies the actual commit before deployment.
          </p>
        </div>
        {canApply ? (
          <Button
            type="button"
            variant={result.defaultsApplied ? "outline" : "primary"}
            size="sm"
            className="shrink-0"
            disabled={result.defaultsApplied || applyDefaults.isLoading}
            loading={applyDefaults.isLoading}
            onClick={() =>
              applyDefaults.mutate({ projectId, appId, fingerprint: result.fingerprint })
            }
          >
            {result.defaultsApplied ? "Defaults applied" : "Use detected settings"}
          </Button>
        ) : null}
      </div>

      {defaultsList.length > 0 ? (
        <div className="mt-3 flex flex-wrap gap-2">
          {defaultsList.map((value) => (
            <span key={value} className="rounded-md bg-grayA-3 px-2 py-1 text-xs text-gray-11">
              {value}
            </span>
          ))}
        </div>
      ) : null}

      {detection.warnings.length > 0 || detection.unresolvedDecisions.length > 0 ? (
        <div className="mt-3 rounded-md border border-warningA-5 bg-warningA-2 px-3 py-2">
          {detection.warnings.map((warning) => (
            <p key={warning.code} className="text-xs text-warning-11">
              {warning.message}
            </p>
          ))}
          {detection.unresolvedDecisions.map((decision) => (
            <p key={decision.code} className="text-xs text-warning-11">
              {decision.message} Options: {decision.options.join(", ")}.
            </p>
          ))}
        </div>
      ) : null}
    </DetectionShell>
  );
};

const DetectionShell = ({ children }: { children: ReactNode }) => (
  <div className="mb-4 rounded-xl border border-grayA-5 bg-grayA-2 p-4 shadow-sm">{children}</div>
);

const DetectionPill = ({ children }: { children: ReactNode }) => (
  <span className="rounded-full border border-grayA-5 bg-grayA-2 px-2 py-0.5 text-[11px] font-medium text-gray-11">
    {children}
  </span>
);
