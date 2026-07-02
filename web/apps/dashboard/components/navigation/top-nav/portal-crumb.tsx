"use client";

import { usePortal, usePortals } from "@/app/(app)/[workspaceSlug]/portal/data/portals";
import { useWorkspaceNavigation } from "@/hooks/use-workspace-navigation";
import { Plus, WindowLayout } from "@unkey/icons";
import { Crumb } from "./crumb";
import type { CrumbPopoverItem } from "./crumb-popover";

export function PortalCrumb({ portalId }: { portalId: string }) {
  const workspace = useWorkspaceNavigation();
  const portals = usePortals();
  const portal = usePortal(portalId);
  const label = portal?.resourceName ?? portalId;

  const items: CrumbPopoverItem[] = portals.map((p) => ({
    id: p.id,
    label: p.resourceName,
    href: `/${workspace.slug}/portal/${p.id}`,
  }));

  return (
    <Crumb
      icon={<WindowLayout className="size-3.5 text-accent-11" iconSize="sm-regular" />}
      label={label}
      href={`/${workspace.slug}/portal/${portalId}`}
      items={items}
      currentId={portalId}
      searchPlaceholder="Find portal..."
      emptyText="No portals found"
      footer={{
        icon: Plus,
        label: "All portals",
        href: `/${workspace.slug}/portal`,
      }}
    />
  );
}
