"use client";

import { shortenId } from "@/lib/shorten-id";
import { trpc } from "@/lib/trpc/client";
import { PageBody } from "@unkey/ui";
import { useDeployment } from "../layout-provider";

export default function DeploymentSourcePage() {
  const { deployment } = useDeployment();
  const resources = trpc.deploy.deployment.resources.useQuery({
    deploymentId: deployment.id,
    projectId: deployment.projectId,
  });
  const build = trpc.deploy.deployment.buildSteps.useQuery({
    deploymentId: deployment.id,
  });

  return (
    <PageBody>
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.2fr)_minmax(300px,.8fr)]">
        <section className="overflow-hidden rounded-lg border border-grayA-4 bg-gray-1">
          <header className="border-b border-grayA-4 px-4 py-3">
            <h2 className="text-xs font-medium text-gray-12">Immutable deployment manifest</h2>
            <p className="mt-1 text-[11px] text-gray-9">
              Fingerprint {resources.data?.manifest?.fingerprint ?? "not recorded"}
            </p>
          </header>
          <pre className="max-h-[70dvh] overflow-auto p-4 font-mono text-[11px] leading-5 text-gray-11">
            {resources.isLoading
              ? "Loading manifest…"
              : JSON.stringify(resources.data?.manifest?.manifest ?? {}, null, 2)}
          </pre>
        </section>
        <section className="overflow-hidden rounded-lg border border-grayA-4 bg-gray-1">
          <header className="border-b border-grayA-4 px-4 py-3">
            <h2 className="text-xs font-medium text-gray-12">Build graph</h2>
            <p className="mt-1 text-[11px] text-gray-9">Deployment {shortenId(deployment.id)}</p>
          </header>
          {build.isLoading ? (
            <div className="m-4 h-20 animate-pulse rounded bg-grayA-2" />
          ) : (build.data?.steps.length ?? 0) === 0 ? (
            <p className="p-4 text-xs text-gray-9">No structured build steps were recorded.</p>
          ) : (
            <div className="divide-y divide-grayA-3">
              {build.data?.steps.map((step) => (
                <div key={step.step_id} className="flex items-start justify-between gap-3 px-4 py-3">
                  <div className="min-w-0">
                    <p className="truncate text-xs text-gray-12">{step.name}</p>
                    {step.error ? <p className="mt-1 text-[10px] text-error-11">{step.error}</p> : null}
                  </div>
                  <span className="shrink-0 rounded-full bg-grayA-3 px-2 py-0.5 text-[10px] text-gray-10">
                    {step.cached ? "cached" : `${Math.max(step.completed_at - step.started_at, 0)}ms`}
                  </span>
                </div>
              ))}
            </div>
          )}
        </section>
      </div>
    </PageBody>
  );
}
