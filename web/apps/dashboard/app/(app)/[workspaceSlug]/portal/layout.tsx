import type { ReactNode } from "react";
import { DebugPanel, DebugProvider } from "./debug/debug-panel";

export default function PortalLayout({ children }: { children: ReactNode }) {
  return (
    <DebugProvider>
      {children}
      <DebugPanel />
    </DebugProvider>
  );
}
