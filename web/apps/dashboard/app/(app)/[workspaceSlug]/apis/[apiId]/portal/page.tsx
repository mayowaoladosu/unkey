"use client";

import { PortalSettings } from "@/app/(app)/[workspaceSlug]/portal/components/portal-settings";
import { useWorkspaceNavigation } from "@/hooks/use-workspace-navigation";
import { trpc } from "@/lib/trpc/client";
import { SettingsShell } from "@unkey/ui";
import { use } from "react";
import { ApisNavbar } from "../api-id-navbar";

type Props = {
  params: Promise<{ apiId: string }>;
};

export default function ApiPortalPage(props: Props) {
  const { apiId } = use(props.params);
  const workspace = useWorkspaceNavigation();
  const { data } = trpc.api.queryApiKeyDetails.useQuery({ apiId }, { enabled: Boolean(apiId) });
  const name = data?.currentApi?.name ?? apiId;

  return (
    <div>
      <ApisNavbar
        apiId={apiId}
        activePage={{ href: `/${workspace.slug}/apis/${apiId}/portal`, text: "Customer portal" }}
      />
      <SettingsShell>
        <PortalSettings resourceName={name} portalId={`portal_${apiId}`} />
      </SettingsShell>
    </div>
  );
}
