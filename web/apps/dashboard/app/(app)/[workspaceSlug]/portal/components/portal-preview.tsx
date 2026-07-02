"use client";

import { cn } from "@/lib/utils";
import type { PortalBrandingValue } from "./portal-branding";

const HEX_RE = /^#[0-9a-fA-F]{6}$/;

function contrastText(hex: string) {
  const n = Number.parseInt(hex.slice(1), 16);
  const luminance = 0.2126 * ((n >> 16) & 255) + 0.7152 * ((n >> 8) & 255) + 0.0722 * (n & 255);
  return luminance > 160 ? "#18181B" : "#FFFFFF";
}

/**
 * Static, deliberately-lo-fi mock of the end-user portal page so operators can
 * see their logo + brand color in context before going live.
 */
export function PortalPreview({
  name,
  url,
  branding,
  className,
}: {
  name: string;
  url: string;
  branding: PortalBrandingValue;
  className?: string;
}) {
  const color = HEX_RE.test(branding.primaryColor) ? branding.primaryColor : "#18181B";
  const initial = (name.trim()[0] ?? "P").toUpperCase();

  return (
    <div
      className={cn(
        "w-full overflow-hidden rounded-xl border border-grayA-4 bg-gray-1 shadow-sm",
        className,
      )}
    >
      <div className="flex items-center gap-2 border-b border-grayA-3 bg-gray-2 px-3 py-2">
        <div className="flex gap-1.5">
          <span className="size-2 rounded-full bg-grayA-5" />
          <span className="size-2 rounded-full bg-grayA-5" />
          <span className="size-2 rounded-full bg-grayA-5" />
        </div>
        <div className="flex-1 truncate rounded-md border border-grayA-3 bg-gray-1 px-2 py-0.5 text-center text-[10px] text-gray-9">
          {url}
        </div>
      </div>

      <div className="flex items-center justify-between border-b border-grayA-3 px-4 py-3">
        <div className="flex min-w-0 items-center gap-2.5">
          {branding.logoUrl ? (
            <img
              src={branding.logoUrl}
              alt=""
              className="size-6 shrink-0 rounded-md object-contain"
            />
          ) : (
            <span
              className="flex size-6 shrink-0 items-center justify-center rounded-md text-[12px] font-semibold"
              style={{ backgroundColor: color, color: contrastText(color) }}
            >
              {initial}
            </span>
          )}
          <span className="truncate text-[13px] font-medium text-accent-12">{name}</span>
        </div>
        <span className="size-5 shrink-0 rounded-full bg-grayA-4" />
      </div>

      <div className="flex flex-col gap-3 px-4 py-4">
        <div className="h-3 w-24 rounded bg-grayA-5" />
        <div className="h-2 w-44 max-w-full rounded bg-grayA-3" />
        <div className="rounded-lg border border-grayA-3">
          {[0, 1].map((row) => (
            <div
              key={row}
              className={cn(
                "flex items-center justify-between px-3 py-3",
                row > 0 && "border-t border-grayA-3",
              )}
            >
              <div className="flex flex-col gap-1.5">
                <div className="h-2 w-20 rounded bg-grayA-5" />
                <div className="h-1.5 w-32 rounded bg-grayA-3" />
              </div>
              <div className="h-2 w-10 rounded bg-grayA-3" />
            </div>
          ))}
        </div>
        <div className="flex items-center justify-between pt-1">
          <span
            className="rounded-md px-3 py-1.5 text-[11px] font-medium"
            style={{ backgroundColor: color, color: contrastText(color) }}
          >
            Create key
          </span>
          <span className="h-2 w-16 rounded" style={{ backgroundColor: `${color}33` }} />
        </div>
      </div>

      <div className="border-t border-grayA-3 px-4 py-2 text-center text-[10px] text-gray-8">
        Powered by Unkey
      </div>
    </div>
  );
}
