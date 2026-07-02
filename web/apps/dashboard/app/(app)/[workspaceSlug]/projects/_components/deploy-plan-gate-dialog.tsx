"use client";

import { useWorkspaceNavigation } from "@/hooks/use-workspace-navigation";
import { routes } from "@/lib/navigation/routes";
import type { DeployPlan } from "@/lib/stripe/deployPlan";
import { trpc } from "@/lib/trpc/client";
import type { DeployPlanOption } from "@/lib/trpc/routers/stripe/getDeployPlans";
import { toast } from "@unkey/ui";
import type { Route } from "next";
import Link from "next/link";
import {
  ComputePlanDialog,
  ComputePlanFeatures,
  ComputePlanRows,
  ComputePlansMoreInfo,
} from "../../settings/billing/components/compute-plan-picker";

type ViewProps = {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
  plans: DeployPlanOption[];
  plansLoading: boolean;
  isAdmin: boolean;
  billingHref: Route;
  onSelect: (plan: DeployPlan) => void;
};

/**
 * Presentational layer of the paywall, also driven with mock states by the
 * dev-only DeployGateDebugBar.
 */
export function DeployPlanGateDialogView({
  isOpen,
  onOpenChange,
  plans,
  plansLoading,
  isAdmin,
  billingHref,
  onSelect,
}: ViewProps) {
  function renderPlanSection() {
    if (plansLoading) {
      return (
        <div className="flex flex-col gap-2.5" aria-hidden="true">
          <div className="h-[62px] animate-pulse rounded-[11px] border border-gray-4 bg-grayA-2" />
          <div className="h-[62px] animate-pulse rounded-[11px] border border-gray-4 bg-grayA-2" />
          <div className="h-[62px] animate-pulse rounded-[11px] border border-gray-4 bg-grayA-2" />
        </div>
      );
    }
    if (plans.length === 0) {
      return (
        <div className="rounded-[11px] border border-gray-4 bg-gray-1 px-4 py-6 text-center">
          <p className="text-[13px] text-gray-11">Compute plans aren't available right now.</p>
          <Link
            href={billingHref}
            onClick={() => onOpenChange(false)}
            className="mt-2 inline-block font-medium text-[13px] text-info-11 hover:underline"
          >
            Go to billing
          </Link>
        </div>
      );
    }
    return (
      <ComputePlanRows
        plans={plans}
        onSelect={onSelect}
        disabledReason={isAdmin ? undefined : "Only workspace admins can manage billing."}
      />
    );
  }

  return (
    <ComputePlanDialog
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      title="Choose a Compute plan"
      subTitle="Deploying on Unkey requires a Compute plan. Select one to continue."
    >
      <div className="flex flex-col gap-2.5">
        <ComputePlanFeatures />
        <ComputePlansMoreInfo />
      </div>
      <div className="mt-0">{renderPlanSection()}</div>
      {isAdmin ? null : (
        <p className="text-center text-[12px] text-gray-11">
          Only workspace admins can manage billing.
        </p>
      )}
    </ComputePlanDialog>
  );
}

type Props = {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
};

/**
 * The Compute paywall on the projects page. Confirming a plan is not wired up
 * yet: subscribe (card on file) / Stripe checkout (no card) land here later.
 */
export function DeployPlanGateDialog({ isOpen, onOpenChange }: Props) {
  const workspace = useWorkspaceNavigation();

  const { data: plansData, isLoading: plansLoading } = trpc.stripe.getDeployPlans.useQuery(
    undefined,
    { staleTime: 60_000 },
  );
  const { data: currentUser } = trpc.user.getCurrentUser.useQuery();
  const isAdmin = currentUser?.role === "admin";
  const plans = plansData?.plans ?? [];

  return (
    <DeployPlanGateDialogView
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      plans={plans}
      plansLoading={plansLoading}
      isAdmin={isAdmin}
      billingHref={routes.settings.billing({ workspaceSlug: workspace.slug })}
      onSelect={() => {
        // TODO(deploy-billing): route the chosen plan to Stripe checkout (no
        // card) or subscribe (card on file), then unlock project creation.
        onOpenChange(false);
        toast.info("Subscribing isn't wired up yet.");
      }}
    />
  );
}
