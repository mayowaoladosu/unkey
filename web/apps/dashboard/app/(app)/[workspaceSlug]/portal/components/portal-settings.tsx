"use client";

import { Switch } from "@/components/ui/switch";
import { slugify } from "@/lib/slugify";
import { cn } from "@/lib/utils";
import { Button, CopyButton, SettingCard, SettingCardGroup } from "@unkey/ui";
import { useState } from "react";
import { IntegrateDialog } from "./integrate-dialog";
import { PortalBranding, type PortalBrandingValue } from "./portal-branding";
import { PortalPreview } from "./portal-preview";

/**
 * Portal config that lives inside a project or keyspace: a prominent
 * enable/disable header (auth-providers style) that gates the settings below,
 * plus branding controls with a live preview card.
 * Local state only — prototype; wire to portal_configurations/portal_branding later.
 */
export function PortalSettings({
  resourceName,
  portalId,
  defaultEnabled = false,
}: {
  resourceName: string;
  portalId: string;
  defaultEnabled?: boolean;
}) {
  const [enabled, setEnabled] = useState(defaultEnabled);
  const [integrateOpen, setIntegrateOpen] = useState(false);
  const [branding, setBranding] = useState<PortalBrandingValue>({
    logoUrl: "",
    primaryColor: "#18181B",
  });
  const url = `${slugify(resourceName)}.unkey.com/portal`;

  return (
    <div className="flex w-full flex-col gap-6">
      <div className="flex items-center gap-4 rounded-2xl border border-grayA-4 p-5">
        <div className="flex-1">
          <div className="text-sm font-medium text-accent-12">Customer portal</div>
          <p className="mt-1 text-[13px] leading-5 text-gray-11">
            Give your end users a branded page to manage their API keys. When enabled, you can
            create sessions and redirect users to it.
          </p>
        </div>
        <Switch checked={enabled} onCheckedChange={setEnabled} />
      </div>

      <div
        className={cn(
          "flex flex-col gap-6 transition-opacity duration-200",
          !enabled && "pointer-events-none select-none opacity-50",
        )}
        aria-hidden={!enabled}
      >
        <SettingCardGroup>
          <SettingCard
            title="Portal URL"
            description="Served on your Unkey subdomain. Custom domains come later."
            contentWidth="w-full lg:w-[420px]"
          >
            <div className="flex w-full items-center justify-end gap-2">
              <div className="flex-1 truncate rounded-lg border border-grayA-4 px-3 py-2 text-[13px] text-gray-11 lg:w-[300px] lg:flex-none">
                {url}
              </div>
              <CopyButton value={url} />
            </div>
          </SettingCard>
          <SettingCard
            title="Integration"
            description="Create a session for a signed-in user, then redirect them here."
            contentWidth="w-auto"
          >
            <Button onClick={() => setIntegrateOpen(true)}>How to integrate</Button>
          </SettingCard>
        </SettingCardGroup>

        <PortalBranding
          value={branding}
          onChange={setBranding}
          preview={<PortalPreview name={resourceName} url={url} branding={branding} />}
        />
      </div>

      <IntegrateDialog portalId={portalId} isOpen={integrateOpen} onOpenChange={setIntegrateOpen} />
    </div>
  );
}
