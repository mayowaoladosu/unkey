"use client";

import { Switch } from "@/components/ui/switch";
import { useWorkspaceNavigation } from "@/hooks/use-workspace-navigation";
import {
  Button,
  CopyButton,
  PageBody,
  PageContainer,
  PageHeader,
  PageHeaderActions,
  PageHeaderContent,
  PageHeaderTitle,
  SettingCard,
  SettingCardGroup,
  SettingsDangerZone,
} from "@unkey/ui";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useState } from "react";
import { DeletePortal } from "../components/delete-portal";
import { IntegrateDialog } from "../components/integrate-dialog";
import { portalUrl, setPortalEnabled, usePortal } from "../data/portals";

export default function PortalConfigPage() {
  const workspace = useWorkspaceNavigation();
  const params = useParams<{ portalId: string }>();
  const portal = usePortal(params.portalId);
  const [integrateOpen, setIntegrateOpen] = useState(false);

  if (!portal) {
    return (
      <PageContainer>
        <PageHeader>
          <PageHeaderContent>
            <PageHeaderTitle>Portal not found</PageHeaderTitle>
          </PageHeaderContent>
        </PageHeader>
        <PageBody>
          <div className="text-gray-11 text-sm">
            This portal doesn't exist.{" "}
            <Link className="text-accent-11 underline" href={`/${workspace.slug}/portal`}>
              Back to portals
            </Link>
          </div>
        </PageBody>
      </PageContainer>
    );
  }

  return (
    <PageContainer>
      <PageHeader>
        <PageHeaderContent>
          <PageHeaderTitle>{portal.resourceName}</PageHeaderTitle>
        </PageHeaderContent>
        <PageHeaderActions>
          <Button variant="primary" onClick={() => setIntegrateOpen(true)}>
            How to integrate
          </Button>
        </PageHeaderActions>
      </PageHeader>
      <PageBody>
        <div className="flex w-full flex-col gap-6">
          <SettingCardGroup>
            <SettingCard
              title="Portal access"
              description="When enabled, sessions can be created and your customers can sign in."
              contentWidth="w-auto"
            >
              <div className="flex w-full justify-end">
                <Switch
                  checked={portal.enabled}
                  onCheckedChange={(checked) => setPortalEnabled(portal.id, checked)}
                />
              </div>
            </SettingCard>
            <SettingCard
              title="Portal URL"
              description="Served on your Unkey subdomain. Custom domains come later."
              contentWidth="w-full lg:w-[420px]"
            >
              <div className="flex w-full items-center justify-end gap-2">
                <div className="flex-1 lg:flex-none lg:w-[300px] truncate rounded-lg border border-grayA-4 px-3 py-2 text-[13px] text-gray-11">
                  {portalUrl(portal.slug)}
                </div>
                <CopyButton value={portalUrl(portal.slug)} />
              </div>
            </SettingCard>
            <SettingCard
              title="Connected resource"
              description="Set when the portal was created. Changing it means deleting and recreating."
              contentWidth="w-auto"
            >
              <span className="text-[11px] rounded-full border border-grayA-4 px-2 py-0.5 text-gray-11 whitespace-nowrap">
                {portal.resourceType === "app" ? "Deploy app" : "API keyspace"} ·{" "}
                {portal.resourceName}
              </span>
            </SettingCard>
          </SettingCardGroup>

          <SettingsDangerZone>
            <DeletePortal portal={portal} />
          </SettingsDangerZone>
        </div>
      </PageBody>
      <IntegrateDialog
        portalId={portal.id}
        isOpen={integrateOpen}
        onOpenChange={setIntegrateOpen}
      />
    </PageContainer>
  );
}
