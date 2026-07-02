"use client";

import { PortalSettings } from "@/app/(app)/[workspaceSlug]/portal/components/portal-settings";
import { useProjectData } from "@/app/(app)/[workspaceSlug]/projects/[projectId]/apps/[appId]/(overview)/data-provider";
import { PageBody, PageContainer, PageHeader, PageHeaderContent, PageHeaderTitle } from "@unkey/ui";

export default function ProjectPortalPage() {
  const { projectId, project } = useProjectData();
  const name = project?.name ?? "your-project";

  return (
    <PageContainer>
      <PageHeader>
        <PageHeaderContent>
          <PageHeaderTitle>Customer portal</PageHeaderTitle>
        </PageHeaderContent>
      </PageHeader>
      <PageBody>
        <PortalSettings resourceName={name} portalId={`portal_${projectId}`} />
      </PageBody>
    </PageContainer>
  );
}
