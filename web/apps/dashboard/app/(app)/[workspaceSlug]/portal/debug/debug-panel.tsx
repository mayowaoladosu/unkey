"use client";

import { cn } from "@/lib/utils";
import { type ReactNode, createContext, useContext, useMemo, useState } from "react";

type DebugControl = { key: string; label: string; options: string[] };

// Register debug state switches here. Each renders as a segmented control in
// the fixed panel and is readable from any portal page via useDebug().
// Intentionally not dev-gated so states can be reviewed in any environment.
export const DEBUG_CONTROLS: DebugControl[] = [
  { key: "listView", label: "Portal list", options: ["filled", "empty"] },
];

type DebugContextValue = {
  values: Record<string, string>;
  set: (key: string, value: string) => void;
};

const DebugContext = createContext<DebugContextValue | null>(null);

export function DebugProvider({ children }: { children: ReactNode }) {
  const [values, setValues] = useState<Record<string, string>>(() =>
    Object.fromEntries(DEBUG_CONTROLS.map((control) => [control.key, control.options[0]])),
  );

  const value = useMemo<DebugContextValue>(
    () => ({
      values,
      set: (key, next) => setValues((prev) => ({ ...prev, [key]: next })),
    }),
    [values],
  );

  return <DebugContext.Provider value={value}>{children}</DebugContext.Provider>;
}

export function useDebug(): DebugContextValue {
  const ctx = useContext(DebugContext);
  if (!ctx) {
    throw new Error("useDebug must be used within a DebugProvider");
  }
  return ctx;
}

export function DebugPanel() {
  const { values, set } = useDebug();
  const [open, setOpen] = useState(true);

  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2 rounded-xl border border-dashed border-grayA-6 bg-gray-1 p-3 shadow-lg">
      <div className="flex items-center justify-between gap-4">
        <span className="text-[11px] uppercase tracking-wide text-gray-9">Debug</span>
        <button
          type="button"
          onClick={() => setOpen((prev) => !prev)}
          className="text-[11px] text-gray-10 hover:text-gray-12"
        >
          {open ? "hide" : "show"}
        </button>
      </div>
      {open &&
        DEBUG_CONTROLS.map((control) => (
          <div key={control.key} className="flex items-center gap-2">
            <span className="w-20 text-[11px] text-gray-10">{control.label}</span>
            <div className="flex items-center gap-1 rounded-lg border border-grayA-4 bg-grayA-2 p-0.5">
              {control.options.map((option) => (
                <button
                  key={option}
                  type="button"
                  onClick={() => set(control.key, option)}
                  className={cn(
                    "rounded-md px-2 py-0.5 text-[12px] capitalize transition-colors",
                    values[control.key] === option
                      ? "bg-white text-gray-12 shadow-sm dark:bg-black"
                      : "text-gray-10 hover:text-gray-12",
                  )}
                >
                  {option}
                </button>
              ))}
            </div>
          </div>
        ))}
    </div>
  );
}
