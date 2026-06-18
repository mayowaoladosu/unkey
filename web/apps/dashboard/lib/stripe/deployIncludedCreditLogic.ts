import type Stripe from "stripe";
import type { DeployBillingConfig } from "./deployBilling";
import { EXPIRY_GRACE_SECONDS, netDeployFee } from "./deployCredits";

/**
 * The workspace's Deploy included credit for the period its paid invoices
 * currently cover: the sum of every paid invoice's net Deploy plan-fee for that
 * period. Absolute, not incremental, so applying the same invoice twice is a
 * no-op. a webhook retry or an optimistic write racing the webhook recomputes
 * the same total instead of double-counting. This is the invoice-side twin of
 * the credit grant's periodTotalCents (grants sum to the same amount), so every
 * write path lands on the same number.
 *
 * Only Deploy plan-fee lines count (netDeployFee filters by plan-fee price id),
 * so an Unkey API plan or its proration on a mixed invoice never leaks in. The
 * credit is exactly the Deploy plan fee paid this period.
 *
 * Only invoices with a positive net Deploy plan-fee count, matching the credit
 * grant (grantDeployCreditsForInvoice grants nothing for a non-positive net).
 * Subscribe and renewal contribute the full fee; a mid-cycle upgrade adds a
 * second invoice whose net (+new, -old) is the prorated top-up; a downgrade's
 * non-positive net contributes nothing, so the period keeps its credit.
 *
 * The current period is the one the most recent credit-granting invoice covers.
 * Returns null when no such invoice exists or its period has already closed,
 * meaning "leave the stored value alone".
 */
export function deployIncludedCreditCents(
  config: DeployBillingConfig,
  paidInvoices: Stripe.Invoice[],
): number | null {
  // Invoices come newest-first from Stripe; the current period is the one the
  // most recent credit-granting (positive net) Deploy invoice belongs to.
  let periodEnd: number | null = null;
  for (const invoice of paidInvoices) {
    const fee = netDeployFee(config, invoice.lines.data);
    if (fee && fee.amountCents > 0) {
      periodEnd = fee.periodEnd;
      break;
    }
  }
  if (periodEnd === null) {
    return null;
  }

  // Paid so long ago the period's usage invoice has finalized: the credit can
  // no longer be redeemed, so there is nothing live to mirror.
  if ((periodEnd + EXPIRY_GRACE_SECONDS) * 1000 <= Date.now()) {
    return null;
  }

  // Sum every positive-net Deploy plan-fee invoice for this period. Grouping by
  // period end matches how the credit grants group (they share expires_at), so
  // the mirror tracks exactly the total the grants apply.
  let totalCents = 0;
  for (const invoice of paidInvoices) {
    const fee = netDeployFee(config, invoice.lines.data);
    if (fee && fee.amountCents > 0 && fee.periodEnd === periodEnd) {
      totalCents += fee.amountCents;
    }
  }

  return totalCents;
}
