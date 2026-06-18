import type Stripe from "stripe";
import { deployBillingConfig } from "./deployBilling";
import { deployIncludedCreditCents } from "./deployIncludedCreditLogic";

/**
 * The Deploy included credit implied by the subscription's current period,
 * recomputed from its paid invoices. Absolute and idempotent: calling it twice
 * yields the same number, so an optimistic write and the invoice webhook cannot
 * disagree or double-count. Used on the subscribe/change paths (before the
 * grant exists) and to backfill workspaces whose column predates this feature.
 * Returns null when Deploy billing is not configured or no live Deploy plan-fee
 * invoice exists, meaning "leave the stored value alone".
 */
export async function deployIncludedCreditForSubscription(
  stripe: Stripe,
  subscriptionId: string,
): Promise<number | null> {
  const config = await deployBillingConfig();
  if (!config) {
    return null;
  }
  // A period holds at most a subscribe/renewal invoice plus a handful of
  // upgrade prorations, all newest-first, so one page covers the current one.
  const invoices = await stripe.invoices.list({
    subscription: subscriptionId,
    status: "paid",
    limit: 100,
  });
  return deployIncludedCreditCents(config, invoices.data);
}
