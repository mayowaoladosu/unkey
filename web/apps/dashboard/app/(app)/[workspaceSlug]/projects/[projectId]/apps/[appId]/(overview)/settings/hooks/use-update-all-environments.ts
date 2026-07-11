"use client";

import { collection } from "@/lib/collections";
import type { EnvironmentSettings } from "@/lib/collections/deploy/environment-settings";
import { useCallback } from "react";
import { useProjectData } from "../../data-provider";
import { useEnvironmentSettings } from "../environment-provider";

/**
 * Returns a function that applies a settings mutation to every environment.
 *
 * Use this for settings that don't have per-environment UI (e.g. dockerfile,
 * root directory, port, command, healthcheck) so they stay consistent across
 * all environments.
 */
export function useUpdateAllEnvironments() {
  const { environments } = useProjectData();
  const { settings, variant } = useEnvironmentSettings();

  return useCallback(
    (updater: (draft: EnvironmentSettings) => void) => {
      const targets =
        variant === "environment"
          ? environments.filter((environment) => environment.id === settings.environmentId)
          : environments;
      for (const env of targets) {
        collection.environmentSettings.update(env.id, updater);
      }
    },
    [environments, settings.environmentId, variant],
  );
}
