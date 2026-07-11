"use client";

import {
  type DeploymentOutput,
  deploymentOutputsSchema,
} from "@/lib/deploy/deployment-output-schema";
import { Layers2, Plus, Trash } from "@unkey/icons";
import { Button } from "@unkey/ui";
import { useEffect, useMemo, useState } from "react";
import { useEnvironmentSettings } from "../../environment-provider";
import { useUpdateAllEnvironments } from "../../hooks/use-update-all-environments";
import { FormSettingCard, resolveSaveState } from "../shared/form-setting-card";

const inputClass =
  "h-9 w-full rounded-md border border-grayA-5 bg-gray-1 px-2.5 text-xs text-gray-12 outline-none focus:border-accent-8";

export function Outputs() {
  const { settings } = useEnvironmentSettings();
  const updateAllEnvironments = useUpdateAllEnvironments();
  const [draft, setDraft] = useState<DeploymentOutput[]>(settings.outputs);

  const serialized = JSON.stringify(settings.outputs);
  useEffect(() => {
    setDraft(settings.outputs);
  }, [serialized, settings.outputs]);

  const validation = useMemo(() => deploymentOutputsSchema.safeParse(draft), [draft]);
  const hasChanges = JSON.stringify(draft) !== serialized;
  const saveState = resolveSaveState([
    [!validation.success, { status: "disabled", reason: validation.error?.issues[0]?.message }],
    [!hasChanges, { status: "disabled", reason: "No changes to save" }],
  ]);

  const patch = (index: number, values: Partial<DeploymentOutput>) => {
    setDraft((current) =>
      current.map((output, outputIndex) =>
        outputIndex === index ? { ...output, ...values } : output,
      ),
    );
  };

  const add = (kind: DeploymentOutput["kind"]) => {
    const ordinal = draft.filter((output) => output.kind === kind).length + 1;
    const baseName = kind === "container" ? "service" : kind;
    const common = { kind, name: `${baseName}-${ordinal}`, public: false } as DeploymentOutput;
    const output: DeploymentOutput =
      kind === "container"
        ? { ...common, port: 8080, upstreamProtocol: "http1" }
        : kind === "static"
          ? { ...common, directory: "dist", spaFallback: true }
          : kind === "function"
            ? { ...common, runtime: "nodejs22", handler: "src/index.handler", port: 8080 }
            : kind === "cron"
              ? { ...common, schedule: "0 * * * *", command: ["npm", "run", "cron"] }
              : { ...common, command: ["npm", "run", "worker"] };
    setDraft((current) => [...current, output]);
  };

  return (
    <FormSettingCard
      icon={<Layers2 className="text-gray-12" iconSize="xl-medium" />}
      title="Services & resources"
      description="Define independently materialized services, functions, workers, cron jobs, static output, and private bindings. Changes apply on the next deploy."
      displayValue={
        settings.outputs.length > 0 ? `${settings.outputs.length} configured` : "Automatic single service"
      }
      onSubmit={() => {
        if (!validation.success) {
          return;
        }
        updateAllEnvironments((settingsDraft) => {
          settingsDraft.outputs = validation.data;
        });
      }}
      saveState={saveState}
      className="data-[form-wide]:max-w-none"
      stickyHeader={
        <div className="flex flex-wrap gap-1.5" data-form-wide>
          {(["container", "function", "worker", "cron", "static"] as const).map((kind) => (
            <Button key={kind} type="button" variant="outline" size="sm" onClick={() => add(kind)}>
              <Plus iconSize="sm-regular" />
              {kind === "container" ? "Service" : kind}
            </Button>
          ))}
        </div>
      }
    >
      <div className="grid gap-3" data-form-wide>
        {draft.length === 0 ? (
          <div className="rounded-md border border-dashed border-grayA-5 p-4 text-xs text-gray-9">
            No explicit resources. Framework detection and the image defaults create one public service.
          </div>
        ) : null}
        {draft.map((output, index) => (
          <div key={`${index}:${output.name}`} className="rounded-lg border border-grayA-5 bg-gray-1 p-3">
            <div className="grid gap-2 md:grid-cols-[130px_minmax(140px,1fr)_auto_auto]">
              <label className="grid gap-1 text-[10px] font-medium uppercase tracking-wide text-gray-9">
                Kind
                <select
                  className={inputClass}
                  value={output.kind}
                  onChange={(event) => patch(index, { kind: event.target.value as DeploymentOutput["kind"] })}
                >
                  <option value="container">Service</option>
                  <option value="function">Function</option>
                  <option value="worker">Worker</option>
                  <option value="cron">Cron</option>
                  <option value="static">Static</option>
                </select>
              </label>
              <Field label="Name">
                <input
                  className={inputClass}
                  value={output.name}
                  onChange={(event) => patch(index, { name: event.target.value })}
                  placeholder="api"
                />
              </Field>
              <label className="flex items-end gap-2 pb-2 text-xs text-gray-11">
                <input
                  type="checkbox"
                  checked={output.public ?? false}
                  disabled={output.kind === "worker" || output.kind === "cron"}
                  onChange={(event) => patch(index, { public: event.target.checked })}
                />
                Public
              </label>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="self-end text-error-11"
                onClick={() => setDraft((current) => current.filter((_, itemIndex) => itemIndex !== index))}
              >
                <Trash iconSize="sm-regular" />
              </Button>
            </div>

            <div className="mt-2 grid gap-2 md:grid-cols-2">
              {(output.kind === "container" || output.kind === "function") && (
                <Field label="Port">
                  <input
                    className={inputClass}
                    type="number"
                    min={1}
                    max={65_535}
                    value={output.port ?? 8080}
                    onChange={(event) => patch(index, { port: Number(event.target.value) })}
                  />
                </Field>
              )}
              {output.kind === "container" && (
                <Field label="Protocol">
                  <select
                    className={inputClass}
                    value={output.upstreamProtocol ?? "http1"}
                    onChange={(event) =>
                      patch(index, { upstreamProtocol: event.target.value as "http1" | "h2c" })
                    }
                  >
                    <option value="http1">HTTP/1</option>
                    <option value="h2c">h2c / gRPC</option>
                  </select>
                </Field>
              )}
              {output.kind === "function" && (
                <>
                  <Field label="Runtime">
                    <select
                      className={inputClass}
                      value={output.runtime ?? "nodejs22"}
                      onChange={(event) => patch(index, { runtime: event.target.value })}
                    >
                      <option value="nodejs22">Node.js 22</option>
                      <option value="python3.12">Python 3.12</option>
                    </select>
                  </Field>
                  <Field label="Handler">
                    <input
                      className={inputClass}
                      value={output.handler ?? ""}
                      onChange={(event) => patch(index, { handler: event.target.value })}
                      placeholder="src/index.handler"
                    />
                  </Field>
                </>
              )}
              {output.kind === "static" && (
                <>
                  <Field label="Output directory">
                    <input
                      className={inputClass}
                      value={output.directory ?? ""}
                      onChange={(event) => patch(index, { directory: event.target.value })}
                      placeholder="dist"
                    />
                  </Field>
                  <label className="flex items-end gap-2 pb-2 text-xs text-gray-11">
                    <input
                      type="checkbox"
                      checked={output.spaFallback ?? false}
                      onChange={(event) => patch(index, { spaFallback: event.target.checked })}
                    />
                    SPA fallback
                  </label>
                </>
              )}
              {(output.kind === "worker" || output.kind === "cron" || output.kind === "container") && (
                <Field label="Command">
                  <input
                    className={inputClass}
                    value={(output.command ?? []).join(" ")}
                    onChange={(event) =>
                      patch(index, { command: event.target.value.trim().split(/\s+/).filter(Boolean) })
                    }
                    placeholder="npm run worker"
                  />
                </Field>
              )}
              {output.kind === "cron" && (
                <Field label="Cron schedule (UTC)">
                  <input
                    className={inputClass}
                    value={output.schedule ?? ""}
                    onChange={(event) => patch(index, { schedule: event.target.value })}
                    placeholder="0 * * * *"
                  />
                </Field>
              )}
              {output.kind !== "static" && (
                <Field label="Private bindings (NAME=resource)">
                  <input
                    className={inputClass}
                    value={(output.bindings ?? [])
                      .map((binding) => `${binding.name}=${binding.resource}`)
                      .join(", ")}
                    onChange={(event) =>
                      patch(index, { bindings: parseBindings(event.target.value) })
                    }
                    placeholder="DATABASE=db, API=api"
                  />
                </Field>
              )}
            </div>
          </div>
        ))}
        {!validation.success ? (
          <p className="text-xs text-error-11">{validation.error.issues[0]?.message}</p>
        ) : null}
      </div>
    </FormSettingCard>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="grid gap-1 text-[10px] font-medium uppercase tracking-wide text-gray-9">
      {label}
      {children}
    </label>
  );
}

function parseBindings(value: string): DeploymentOutput["bindings"] {
  return value
    .split(",")
    .map((entry) => entry.trim())
    .filter(Boolean)
    .map((entry) => {
      const [name, resource] = entry.split("=", 2).map((part) => part.trim());
      return { name, resource, protocol: "http" as const };
    });
}
