"use client";

import { useWorkspaceNavigation } from "@/hooks/use-workspace-navigation";
import { buildRoute, routes } from "@/lib/navigation/routes";
import type { DeployPlanOption } from "@/lib/trpc/routers/stripe/getDeployPlans";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { ComputePlanConfirmDialog } from "../../settings/billing/components/compute-plan-picker";
import { DeployPlanGateDialog, DeployPlanGateDialogView } from "./deploy-plan-gate-dialog";

const MOCK_PLANS: DeployPlanOption[] = [
  {
    plan: "starter",
    name: "Starter",
    description: null,
    amount: 500,
    currency: "usd",
    interval: "month",
  },
  {
    plan: "pro",
    name: "Pro",
    description: null,
    amount: 2500,
    currency: "usd",
    interval: "month",
  },
  {
    plan: "business",
    name: "Business",
    description: null,
    amount: null,
    currency: "usd",
    interval: null,
  },
];

const SCENARIOS = [
  "live",
  "plans",
  "loading",
  "empty",
  "non-admin",
  "confirm",
  "submitting",
] as const;
type Scenario = (typeof SCENARIOS)[number];

/**
 * Dev-only previews of every deploy-gate state, so the flow can be reviewed
 * (and later wired up) without seeding entitlements or Stripe data. "live"
 * mounts the real dialog; "no-projects" toggles ?debug=no-projects so the page
 * renders its real empty branch; the rest drive the presentational pieces with
 * mock plans.
 */
export function DeployGateDebugBar() {
  if (process.env.NODE_ENV !== "development") {
    return null;
  }
  return <DebugBar />;
}

function DebugBar() {
  const workspace = useWorkspaceNavigation();
  const router = useRouter();
  const searchParams = useSearchParams();
  const [scenario, setScenario] = useState<Scenario | null>(null);

  const noProjects = searchParams.get("debug") === "no-projects";
  const toggleNoProjects = () => {
    router.replace(
      noProjects
        ? routes.projects.list({ workspaceSlug: workspace.slug })
        : buildRoute(
            "/[workspaceSlug]/projects",
            { workspaceSlug: workspace.slug },
            { debug: "no-projects" },
          ),
    );
  };

  const billingHref = routes.settings.billing({ workspaceSlug: workspace.slug });
  const close = (open: boolean) => {
    if (!open) {
      setScenario(null);
    }
  };
  const viewProps = {
    onOpenChange: close,
    plans: MOCK_PLANS,
    plansLoading: false,
    isAdmin: true,
    billingHref,
    onSelect: () => setScenario(null),
  };

  const buttonClass = (active: boolean) =>
    `rounded-md px-2 py-1 text-[11px] hover:bg-gray-4 hover:text-gray-12 ${
      active ? "bg-gray-5 text-gray-12" : "text-gray-11"
    }`;

  return (
    <>
      <div className="fixed right-4 bottom-4 z-50 flex items-center gap-1 rounded-lg border border-gray-6 bg-gray-2 p-1 shadow-lg">
        <span className="px-1.5 text-[11px] text-gray-9">deploy gate</span>
        {SCENARIOS.map((s) => (
          <button
            key={s}
            type="button"
            onClick={() => setScenario(scenario === s ? null : s)}
            className={buttonClass(scenario === s)}
          >
            {s}
          </button>
        ))}
        <button type="button" onClick={toggleNoProjects} className={buttonClass(noProjects)}>
          no-projects
        </button>
      </div>

      {scenario === "live" ? (
        <DeployPlanGateDialog isOpen onOpenChange={close} from="banner" />
      ) : null}
      {scenario === "plans" ? <DeployPlanGateDialogView isOpen {...viewProps} /> : null}
      {scenario === "loading" ? (
        <DeployPlanGateDialogView isOpen {...viewProps} plans={[]} plansLoading />
      ) : null}
      {scenario === "empty" ? <DeployPlanGateDialogView isOpen {...viewProps} plans={[]} /> : null}
      {scenario === "non-admin" ? (
        <DeployPlanGateDialogView isOpen {...viewProps} isAdmin={false} />
      ) : null}
      {scenario === "confirm" || scenario === "submitting" ? (
        <ComputePlanConfirmDialog
          plan={MOCK_PLANS[1]}
          onOpenChange={close}
          onConfirm={() => setScenario("submitting")}
          isLoading={scenario === "submitting"}
        />
      ) : null}
    </>
  );
}
