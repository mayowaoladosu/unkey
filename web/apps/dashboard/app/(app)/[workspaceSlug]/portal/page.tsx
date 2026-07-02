"use client";

import { useWorkspaceNavigation } from "@/hooks/use-workspace-navigation";
import { BookBookmark } from "@unkey/icons";
import {
  Button,
  Empty,
  PageBody,
  PageContainer,
  PageHeader,
  PageHeaderActions,
  PageHeaderContent,
  PageHeaderTitle,
} from "@unkey/ui";
import { CreatePortalButton } from "./components/create-portal-button";
import { PortalCard } from "./components/portal-card";
import { usePortals } from "./data/portals";
import { useDebug } from "./debug/debug-panel";

export default function PortalPage() {
  const workspace = useWorkspaceNavigation();
  const portals = usePortals();
  const { values } = useDebug();
  const showEmpty = values.listView === "empty";

  return (
    <PageContainer>
      <PageHeader>
        <PageHeaderContent>
          <PageHeaderTitle>Customer portal</PageHeaderTitle>
        </PageHeaderContent>
        {!showEmpty && (
          <PageHeaderActions>
            <CreatePortalButton />
          </PageHeaderActions>
        )}
      </PageHeader>
      <PageBody>
        {showEmpty ? (
          <div className="h-full min-h-[300px] flex items-center justify-center">
            <Empty>
              <Empty.Icon />
              <Empty.Title>Enable Customer Portal for your end users</Empty.Title>
              <Empty.Description>
                Give your customers a branded page to manage their API keys, view usage, and read
                your docs. No code on their side.
              </Empty.Description>
              <Empty.Actions className="mt-4">
                <CreatePortalButton size="md" />
                <a href="https://www.unkey.com/docs" target="_blank" rel="noopener noreferrer">
                  <Button size="md">
                    <BookBookmark />
                    Documentation
                  </Button>
                </a>
              </Empty.Actions>
            </Empty>
          </div>
        ) : (
          <div className="grid gap-4 grid-cols-1 md:grid-cols-2 xl:grid-cols-3">
            {portals.map((portal) => (
              <PortalCard key={portal.id} portal={portal} workspaceSlug={workspace.slug} />
            ))}
          </div>
        )}
      </PageBody>
    </PageContainer>
  );
}
