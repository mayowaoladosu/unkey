"use client";

import { slugify } from "@/lib/slugify";
import { useSyncExternalStore } from "react";

export type PortalResourceType = "app" | "keyspace";

export type Portal = {
  id: string;
  slug: string;
  resourceType: PortalResourceType;
  resourceName: string;
  enabled: boolean;
};

// Resources a portal can connect to. Prototype fixtures — swap for
// trpc queries (deploy apps / keyspaces) when the data layer lands.
export const MOCK_APPS = [
  { id: "app_billing", name: "billing-api" },
  { id: "app_payments", name: "payments-api" },
  { id: "app_search", name: "search-api" },
];

export const MOCK_KEYSPACES = [
  { id: "ks_docs", name: "docs-keys" },
  { id: "ks_partner", name: "partner-api" },
];

// Module-level store rather than React context: the top-nav breadcrumb renders
// above the portal layout, so it needs to read portals without a provider in
// scope. Swap this whole file for trpc hooks when the backend lands.
let portals: Portal[] = [
  {
    id: "portal_billing",
    slug: "billing-api",
    resourceType: "app",
    resourceName: "billing-api",
    enabled: true,
  },
  {
    id: "portal_docs",
    slug: "docs-keys",
    resourceType: "keyspace",
    resourceName: "docs-keys",
    enabled: false,
  },
];

const listeners = new Set<() => void>();

function emit() {
  for (const listener of listeners) {
    listener();
  }
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function getSnapshot() {
  return portals;
}

export function portalUrl(slug: string): string {
  return `${slug}.unkey.com/portal`;
}

// Monotonic id source for created portals — avoids Math.random and stays
// stable across renders in the prototype.
let createdCount = 0;

export function createPortal(input: {
  resourceType: PortalResourceType;
  resourceName: string;
}): string {
  createdCount += 1;
  const id = `portal_new_${createdCount}`;
  portals = [
    {
      id,
      slug: slugify(input.resourceName),
      resourceType: input.resourceType,
      resourceName: input.resourceName,
      enabled: true,
    },
    ...portals,
  ];
  emit();
  return id;
}

export function setPortalEnabled(id: string, enabled: boolean) {
  portals = portals.map((p) => (p.id === id ? { ...p, enabled } : p));
  emit();
}

export function deletePortal(id: string) {
  portals = portals.filter((p) => p.id !== id);
  emit();
}

export function usePortals(): Portal[] {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

export function usePortal(id: string): Portal | null {
  return usePortals().find((p) => p.id === id) ?? null;
}
