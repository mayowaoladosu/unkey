"use client";

import { cn } from "@/lib/utils";
import { Dots } from "@unkey/icons";
import { Button, InfoTooltip } from "@unkey/ui";
import type { Route } from "next";
import Link from "next/link";
import { type Portal, portalUrl } from "../data/portals";
import { PortalActions } from "./portal-actions";

export function PortalCard({ portal, workspaceSlug }: { portal: Portal; workspaceSlug: string }) {
  const url = portalUrl(portal.slug);
  const href = `/${workspaceSlug}/portal/${portal.id}` as Route;

  return (
    <div className="relative p-5 flex flex-col border border-grayA-4 hover:border-grayA-7 rounded-2xl w-full h-full gap-5 group transition-all duration-300 [&_a]:z-10 [&_button]:z-10">
      {/* Invisible base clickable layer — covers the whole card, opens config. */}
      <Link
        href={href}
        className="absolute inset-0 z-0"
        aria-label={`Configure ${portal.resourceName}`}
      />

      <div className="flex gap-4 items-center min-h-11">
        <div className="flex flex-col w-full gap-2 py-[5px] min-w-0">
          <div className="flex items-center gap-2 min-w-0">
            <InfoTooltip
              content={portal.resourceName}
              asChild
              position={{ align: "start", side: "top" }}
            >
              <Link
                href={href}
                className={cn(
                  "font-medium text-sm leading-[14px] truncate hover:underline",
                  portal.enabled ? "text-accent-12" : "text-gray-11",
                )}
              >
                {portal.resourceName}
              </Link>
            </InfoTooltip>
            {!portal.enabled && (
              <span className="shrink-0 rounded-full bg-grayA-3 px-2 py-0.5 text-[10px] font-medium text-gray-11">
                Disabled
              </span>
            )}
          </div>

          {portal.enabled ? (
            <InfoTooltip content={url} asChild position={{ align: "start", side: "top" }}>
              <a
                href={`https://${url}`}
                target="_blank"
                rel="noopener noreferrer"
                className="relative font-medium text-xs leading-[12px] text-gray-11 truncate max-w-[180px] hover:text-accent-12 transition-colors hover:underline"
              >
                {url}
              </a>
            </InfoTooltip>
          ) : (
            <span className="font-medium text-xs leading-[12px] text-gray-10 truncate max-w-[180px]">
              {url}
            </span>
          )}
        </div>

        <div className="relative">
          <PortalActions portal={portal}>
            <Button variant="ghost" size="icon" className="mb-auto shrink-0" title="Portal actions">
              <Dots iconSize="sm-regular" />
            </Button>
          </PortalActions>
        </div>
      </div>
    </div>
  );
}
