"use client";

import { useWorkspaceNavigation } from "@/hooks/use-workspace-navigation";
import { queryClient } from "@/lib/collections/client";
import {
  dismissSettingsBanner,
  useSettingsBannerEnvironmentIds,
} from "@/lib/collections/deploy/environment-settings";
import { routes } from "@/lib/navigation/routes";
import { trpc } from "@/lib/trpc/client";
import { cn } from "@/lib/utils";
import { Hammer2, XMark } from "@unkey/icons";
import { Button, toast } from "@unkey/ui";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { useProjectData } from "../(overview)/data-provider";
import { GlowIcon } from "../components/glow-icon";
import { resolvePendingRedeployTarget } from "./pending-redeploy-target";

export function PendingRedeployBanner() {
  const { deployments, environments, projectId } = useProjectData();
  const router = useRouter();
  const workspace = useWorkspaceNavigation();
  const changedEnvironmentIds = useSettingsBannerEnvironmentIds();
  const target = resolvePendingRedeployTarget(
    changedEnvironmentIds,
    environments,
    deployments,
  );
  const targetEnvironment = target?.environment;
  const targetDeployment = target?.deployment;
  const visible = changedEnvironmentIds.length > 0;
  const show = visible && !!targetEnvironment && !!targetDeployment;

  const redeploy = trpc.deploy.deployment.redeploy.useMutation({
    onSuccess: async (data) => {
      if (!targetDeployment || !targetEnvironment) {
        return;
      }
      await queryClient.invalidateQueries({ queryKey: ["deployments", projectId] });
      router.push(
        routes.projects.apps.deployment({
          workspaceSlug: workspace.slug,
          projectId: targetDeployment.projectId,
          appId: targetDeployment.appId,
          deploymentId: data.deploymentId,
        }),
      );
      dismissSettingsBanner(targetEnvironment.id);
    },
    onError: (error) => {
      toast.error("Redeploy failed", { description: error.message });
    },
  });

  useEffect(
    function getDismissedAutomatically() {
      if (!visible) {
        return;
      }
      const timer = setTimeout(() => dismissSettingsBanner(), 10_000);
      return () => {
        clearTimeout(timer);
      };
    },
    [visible],
  );

  return (
    <div
      aria-hidden={!show}
      inert={!show || undefined}
      className={cn(
        "fixed top-6 right-6 z-50 transition-[transform,opacity] duration-300 ease-out",
        show ? "translate-x-0 opacity-100" : "translate-x-[calc(100%+24px)] opacity-0",
      )}
    >
      <div className="relative flex items-start gap-4 rounded-xl border border-gray-4 bg-gray-1 p-4 shadow-lg w-100">
        <button
          type="button"
          onClick={() => dismissSettingsBanner()}
          className="absolute top-3 right-3 text-gray-9 hover:text-gray-11 transition-colors cursor-pointer"
          aria-label="Dismiss"
        >
          <XMark className="size-4" />
        </button>

        <GlowIcon
          icon={<Hammer2 iconSize="sm-medium" className="size-4.5" />}
          className="w-9 h-9 shrink-0"
        />

        <div className="flex flex-col gap-3 flex-1 min-w-0">
          <div className="flex flex-col gap-1 pr-5">
            <span className="text-sm font-semibold text-gray-12 leading-5">Changes detected</span>
            <span className="text-xs text-gray-11 leading-4">
              Redeploy to apply your latest changes to {targetEnvironment?.slug ?? "this environment"}.
            </span>
          </div>
          <Button
            variant="primary"
            size="md"
            className="w-full"
            disabled={redeploy.isLoading}
            loading={redeploy.isLoading}
            onClick={() => {
              if (targetDeployment) {
                redeploy.mutate({ deploymentId: targetDeployment.id });
              }
            }}
          >
            Redeploy
          </Button>
        </div>
      </div>
    </div>
  );
}
