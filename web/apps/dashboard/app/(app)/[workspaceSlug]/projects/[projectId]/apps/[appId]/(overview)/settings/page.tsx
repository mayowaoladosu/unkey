"use client";

import { usePreventLeave } from "@/hooks/use-prevent-leave";
import { routes } from "@/lib/navigation/routes";
import {
  PageBody,
  PageContainer,
  PageHeader,
  PageHeaderContent,
  PageHeaderTitle,
  SettingsDangerZone,
} from "@unkey/ui";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { useAppId, useProjectData } from "../data-provider";
import { DeleteApp } from "./components/delete-app";
import { DisconnectGitHub } from "./components/disconnect-github";
import { DeploymentSettings } from "./deployment-settings";
import { EnvironmentSettingsProvider } from "./environment-provider";
import { useScrollToHash } from "./hooks/use-scroll-to-hash";

export default function SettingsPage() {
  const { bypass } = usePreventLeave();
  const router = useRouter();
  const params = useParams<{ workspaceSlug: string }>();
  const searchParams = useSearchParams();
  const { projectId, environments } = useProjectData();
  const appId = useAppId();
  const selectedEnvironmentId = searchParams.get("environmentId") ?? "";
  useScrollToHash();

  return (
    <EnvironmentSettingsProvider>
      <PageContainer>
        <PageHeader>
          <PageHeaderContent>
            <PageHeaderTitle>App Settings</PageHeaderTitle>
          </PageHeaderContent>
        </PageHeader>
        <PageBody>
          <div className="mb-5 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-grayA-5 bg-gray-1 p-4 shadow-sm">
            <div>
              <p className="text-sm font-medium text-gray-12">Configuration scope</p>
              <p className="mt-1 text-xs text-gray-9">
                Edit production and preview together, or target one environment independently.
              </p>
            </div>
            <select
              className="h-9 min-w-56 rounded-md border border-grayA-5 bg-gray-1 px-3 text-xs capitalize text-gray-12"
              value={selectedEnvironmentId}
              onChange={(event) =>
                router.replace(
                  routes.projects.apps.settings({
                    workspaceSlug: params.workspaceSlug,
                    projectId,
                    appId,
                    environmentId: event.target.value || undefined,
                  }),
                )
              }
            >
              <option value="">Production + preview</option>
              {environments.map((environment) => (
                <option key={environment.id} value={environment.id}>
                  {environment.slug}
                </option>
              ))}
            </select>
          </div>
          <DeploymentSettings onBeforeNavigate={bypass} />
          <SettingsDangerZone>
            <DisconnectGitHub />
            <DeleteApp />
          </SettingsDangerZone>
        </PageBody>
      </PageContainer>
    </EnvironmentSettingsProvider>
  );
}
