"use client";

import { type MenuItem, TableActionPopover } from "@/components/logs/table-action.popover";
import { Clone, ExternalLink, Trash } from "@unkey/icons";
import { toast } from "@unkey/ui";
import type { PropsWithChildren } from "react";
import { type Portal, deletePortal, portalUrl } from "../data/portals";

export function PortalActions({ portal, children }: PropsWithChildren<{ portal: Portal }>) {
  const url = portalUrl(portal.slug);

  const menuItems: MenuItem[] = [];

  if (portal.enabled) {
    menuItems.push({
      id: "open-portal",
      label: "Open portal",
      icon: <ExternalLink iconSize="md-medium" />,
      onClick: () => window.open(`https://${url}`, "_blank", "noopener,noreferrer"),
    });
  }

  menuItems.push({
    id: "copy-url",
    label: "Copy URL",
    icon: <Clone iconSize="md-medium" />,
    onClick: () => {
      navigator.clipboard
        .writeText(url)
        .then(() => toast.success("Portal URL copied to clipboard"))
        .catch(() => toast.error("Failed to copy to clipboard"));
    },
    divider: true,
  });

  menuItems.push({
    id: "delete-portal",
    label: "Delete portal",
    icon: <Trash iconSize="md-medium" />,
    className: "text-error-11",
    onClick: () => deletePortal(portal.id),
  });

  return <TableActionPopover items={menuItems}>{children}</TableActionPopover>;
}
