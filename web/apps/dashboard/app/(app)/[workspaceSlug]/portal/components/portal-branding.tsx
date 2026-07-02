"use client";

import { cn } from "@/lib/utils";
import { Button, Input, SettingCard, SettingCardGroup } from "@unkey/ui";
import { type ReactNode, useRef, useState } from "react";

// Mirrors the portal_branding schema columns so this can be wired to trpc later.
// logoUrl holds a data URL in the prototype; real impl uploads and stores a URL.
export type PortalBrandingValue = {
  logoUrl: string;
  primaryColor: string;
};

const SWATCHES = ["#18181B", "#7C3AED", "#0D9488", "#D97706", "#DC2626"];
const MAX_LOGO_BYTES = 1024 * 1024;

export function PortalBranding({
  value,
  onChange,
  preview,
}: {
  value: PortalBrandingValue;
  onChange: (next: PortalBrandingValue) => void;
  preview?: ReactNode;
}) {
  const fileRef = useRef<HTMLInputElement>(null);
  const [dragging, setDragging] = useState(false);
  const [logoError, setLogoError] = useState<string | null>(null);

  const readLogo = (file: File | undefined) => {
    if (!file) {
      return;
    }
    if (!file.type.startsWith("image/")) {
      setLogoError("That file isn't an image.");
      return;
    }
    if (file.size > MAX_LOGO_BYTES) {
      setLogoError("Logo must be 1 MB or less.");
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      setLogoError(null);
      onChange({ ...value, logoUrl: String(reader.result) });
    };
    reader.readAsDataURL(file);
  };

  return (
    <SettingCardGroup>
      <SettingCard
        title="Logo"
        description="Square image, PNG or SVG, up to 1 MB. Shown in the portal header."
        contentWidth="w-auto"
      >
        <div className="flex items-center justify-end gap-3">
          {logoError && <span className="text-[12px] text-error-11">{logoError}</span>}
          <button
            type="button"
            aria-label="Upload logo"
            onClick={() => fileRef.current?.click()}
            onDragOver={(e) => {
              e.preventDefault();
              setDragging(true);
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={(e) => {
              e.preventDefault();
              setDragging(false);
              readLogo(e.dataTransfer.files[0]);
            }}
            className={cn(
              "flex size-12 shrink-0 items-center justify-center overflow-hidden rounded-lg border border-dashed border-grayA-6 bg-gray-2 transition-colors hover:border-grayA-8",
              dragging && "border-accent-12 bg-grayA-3",
              value.logoUrl && "border-solid",
            )}
          >
            {value.logoUrl ? (
              <img src={value.logoUrl} alt="Portal logo" className="size-full object-contain" />
            ) : (
              <span className="text-[18px] leading-none text-gray-9">+</span>
            )}
          </button>
          <div className="flex flex-col items-start gap-1">
            <Button size="sm" onClick={() => fileRef.current?.click()}>
              {value.logoUrl ? "Replace" : "Upload"}
            </Button>
            {value.logoUrl && (
              <Button
                size="sm"
                variant="ghost"
                onClick={() => {
                  setLogoError(null);
                  onChange({ ...value, logoUrl: "" });
                }}
              >
                Remove
              </Button>
            )}
          </div>
          <input
            ref={fileRef}
            type="file"
            accept="image/*"
            className="hidden"
            onChange={(e) => {
              readLogo(e.target.files?.[0]);
              e.target.value = "";
            }}
          />
        </div>
      </SettingCard>
      <SettingCard
        title="Brand color"
        description="Used for buttons and links in the portal."
        contentWidth="w-auto"
      >
        <div className="flex items-center justify-end gap-2">
          <div className="flex items-center gap-1.5 pr-1">
            {SWATCHES.map((hex) => (
              <button
                key={hex}
                type="button"
                aria-label={`Use ${hex}`}
                onClick={() => onChange({ ...value, primaryColor: hex })}
                className={cn(
                  "size-5 rounded-full border border-grayA-6 transition-shadow",
                  value.primaryColor.toUpperCase() === hex &&
                    "ring-2 ring-accent-12 ring-offset-2 ring-offset-gray-1",
                )}
                style={{ backgroundColor: hex }}
              />
            ))}
          </div>
          <label
            className="relative size-9 shrink-0 cursor-pointer overflow-hidden rounded-lg border border-gray-5"
            style={{ backgroundColor: value.primaryColor }}
          >
            <span className="sr-only">Pick brand color</span>
            <input
              type="color"
              value={value.primaryColor}
              onChange={(e) => onChange({ ...value, primaryColor: e.target.value.toUpperCase() })}
              className="absolute inset-0 cursor-pointer opacity-0"
            />
          </label>
          <Input
            className="w-[96px] font-mono uppercase"
            value={value.primaryColor}
            maxLength={7}
            onChange={(e) => {
              const raw = e.target.value.trim();
              onChange({ ...value, primaryColor: raw.startsWith("#") ? raw : `#${raw}` });
            }}
          />
        </div>
      </SettingCard>
      {preview && (
        <SettingCard
          title="Preview"
          description="How the portal looks to your users with the branding above."
          contentWidth="w-full lg:w-[420px]"
        >
          <div className="flex w-full justify-end">{preview}</div>
        </SettingCard>
      )}
    </SettingCardGroup>
  );
}
