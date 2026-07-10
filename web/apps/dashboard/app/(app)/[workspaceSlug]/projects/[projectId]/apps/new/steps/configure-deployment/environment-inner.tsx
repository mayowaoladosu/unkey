"use client";

import { EnvironmentContext } from "@/app/(app)/[workspaceSlug]/projects/[projectId]/apps/[appId]/(overview)/settings/environment-provider";
import { collection } from "@/lib/collections";
import {
  ENVIRONMENT_SETTINGS_DEFAULTS,
  type EnvironmentSettings,
  buildSettingsMutations,
  useSettingsIsSaving,
} from "@/lib/collections/deploy/environment-settings";
import { trpc } from "@/lib/trpc/client";
import { eq, useLiveQuery } from "@tanstack/react-db";
import { Button, toast } from "@unkey/ui";
import { type PropsWithChildren, useCallback, useEffect, useMemo, useRef, useState } from "react";

export const OnboardingEnvironmentSettingsInner = ({
  children,
  prodEnvId,
  environments,
  onSettingsStatusChange,
}: PropsWithChildren<{
  prodEnvId: string;
  environments: { id: string; slug: string }[];
  onSettingsStatusChange: (status: "loading" | "ready" | "error") => void;
}>) => {
  const otherEnvIds = useMemo(
    () => environments.filter((e) => e.id !== prodEnvId).map((e) => e.id),
    [environments, prodEnvId],
  );

  const { data } = useLiveQuery(
    (q) =>
      q
        .from({ s: collection.environmentSettings })
        .where(({ s }) => eq(s.environmentId, prodEnvId)),
    [prodEnvId],
  );

  const settings = data.at(0);

  const isSaving = useSettingsIsSaving();

  const { data: availableRegions } = trpc.deploy.environmentSettings.getAvailableRegions.useQuery(
    undefined,
    { enabled: Boolean(prodEnvId) },
  );

  const { settingsInitialized, initializationError, retryInitialization } = useInitializeSettings(
    environments,
    availableRegions,
  );
  useEffect(() => {
    if (settings && settingsInitialized) {
      onSettingsStatusChange("ready");
    } else if (initializationError) {
      onSettingsStatusChange("error");
    } else {
      onSettingsStatusChange("loading");
    }
  }, [settings, settingsInitialized, initializationError, onSettingsStatusChange]);

  if (initializationError) {
    return (
      <div className="w-225">
        <div className="rounded-xl border border-errorA-5 bg-errorA-2 p-4 shadow-sm">
          <p className="text-[13px] font-semibold text-gray-12">
            Deployment settings could not be initialized
          </p>
          <p className="mt-1 text-xs text-gray-10">{initializationError}</p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="mt-3"
            onClick={retryInitialization}
          >
            Try again
          </Button>
        </div>
      </div>
    );
  }

  if (!settings || !settingsInitialized) {
    return null;
  }

  return (
    <EnvironmentContext.Provider value={{ settings, variant: "onboarding", isSaving }}>
      {otherEnvIds.map((id) => (
        <EnvironmentSettingsPreloader key={id} envId={id} />
      ))}
      {children}
    </EnvironmentContext.Provider>
  );
};

const EnvironmentSettingsPreloader = ({ envId }: { envId: string }) => {
  useLiveQuery(
    (q) =>
      q.from({ s: collection.environmentSettings }).where(({ s }) => eq(s.environmentId, envId)),
    [envId],
  );
  return null;
};

// Settings are empty initially so we persist defaults for every environment.
// Uses buildSettingsMutations directly to bypass the collection's onUpdate
// handler (which would show toasts and whose silent metadata flag is broken).
function useInitializeSettings(
  environments: { id: string; slug: string }[],
  availableRegions: { id: string; name: string }[] | undefined,
): {
  settingsInitialized: boolean;
  initializationError: string | null;
  retryInitialization: () => void;
} {
  const startedAttemptRef = useRef<number | null>(null);
  const [settingsInitialized, setSettingsInitialized] = useState(false);
  const [initializationError, setInitializationError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  const retryInitialization = useCallback(() => {
    setSettingsInitialized(false);
    setInitializationError(null);
    setAttempt((value) => value + 1);
  }, []);

  useEffect(() => {
    if (!availableRegions || environments.length === 0) {
      return;
    }
    if (startedAttemptRef.current === attempt) {
      return;
    }
    startedAttemptRef.current = attempt;
    setSettingsInitialized(false);
    setInitializationError(null);

    const d = ENVIRONMENT_SETTINGS_DEFAULTS;
    const defaults = {
      autoDeploy: d.autoDeploy,
      dockerfile: d.dockerfile,
      dockerContext: d.dockerContext,
      buildCommand: d.buildCommand,
      watchPaths: [] as string[],
      port: d.port,
      cpuMillicores: d.cpuMillicores,
      memoryMib: d.memoryMib,
      storageMib: d.storageMib,
      command: [] as string[],
      healthcheck: null,
      regions: availableRegions.map((r) => ({
        id: r.id,
        name: r.name,
        replicasMin: 1,
        replicasMax: 1,
      })),
      shutdownSignal: d.shutdownSignal,
      upstreamProtocol: d.upstreamProtocol,
      openapiSpecPath: null,
    };

    const empty: EnvironmentSettings = {
      environmentId: "",
      autoDeploy: true,
      dockerfile: "",
      dockerContext: "",
      buildCommand: "",
      watchPaths: [],
      port: 0,
      cpuMillicores: 0,
      memoryMib: 0,
      storageMib: 0,
      command: [],
      healthcheck: null,
      regions: [],
      shutdownSignal: "",
      upstreamProtocol: "http1",
      openapiSpecPath: null,
    };

    const mutations = environments.flatMap((env) =>
      buildSettingsMutations(env.id, empty, { ...defaults, environmentId: env.id }),
    );

    if (mutations.length > 0) {
      Promise.all(mutations)
        .then(() => setSettingsInitialized(true))
        .catch((err) => {
          const message = err instanceof Error ? err.message : "An unexpected error occurred";
          setInitializationError(message);
          toast.error("Failed to initialize settings", {
            description: message,
          });
        });
    } else {
      setSettingsInitialized(true);
    }
  }, [environments, availableRegions, attempt]);

  return { settingsInitialized, initializationError, retryInitialization };
}
