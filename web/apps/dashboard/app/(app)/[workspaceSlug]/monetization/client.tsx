"use client";
import { useWorkspaceNavigation } from "@/hooks/use-workspace-navigation";
import { routes } from "@/lib/navigation/routes";
import { trpc } from "@/lib/trpc/client";
import { Button, SettingCard, SettingCardGroup } from "@unkey/ui";
import Link from "next/link";
import { BillableResourcesCard } from "./components/billable-resources-card";
import { RateCardsCard } from "./components/rate-cards-card";
import { StripeConnectCard } from "./components/stripe-connect-card";

/**
 * Monetization: a customer billing their own end-users from Unkey usage.
 * Distinct from Settings > Billing (Unkey charging the customer). Configures
 * the connected Stripe account, which keyspaces/namespaces are billed, and
 * points to where per-user pricing attaches (identities).
 */
export const MonetizationClient: React.FC = () => {
  const workspace = useWorkspaceNavigation();

  // Mirrors the server-side requireWorkspaceAdmin gate purely for UX, so
  // non-admins see a clear affordance rather than a request that FORBIDs.
  const { data: currentUser } = trpc.user.getCurrentUser.useQuery();
  const isAdmin = currentUser?.role === "admin";

  return (
    <div className="flex w-full flex-col items-center py-6">
      <div className="flex w-full max-w-4xl flex-col gap-6 px-4">
        <SettingCardGroup>
          <StripeConnectCard isAdmin={isAdmin} />
        </SettingCardGroup>

        <RateCardsCard isAdmin={isAdmin} />

        <BillableResourcesCard isAdmin={isAdmin} />

        <SettingCardGroup>
          <SettingCard
            title="Attach billing to a user"
            description="Each end-user (identity) resolves to one rate card — its own selection, a customer assignment, or the workspace default — and maps to a customer on your connected Stripe account. Configure a specific user's rate card and provider customer on its identity."
            border="both"
          >
            <div className="flex w-full justify-end">
              <Button variant="outline" size="lg">
                <Link href={routes.identities.list({ workspaceSlug: workspace.slug })}>
                  Manage identities
                </Link>
              </Button>
            </div>
          </SettingCard>
        </SettingCardGroup>
      </div>
    </div>
  );
};
