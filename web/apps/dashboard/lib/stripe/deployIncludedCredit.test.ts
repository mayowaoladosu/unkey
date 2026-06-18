import type Stripe from "stripe";
import { describe, expect, it } from "vitest";
import type { DeployBillingConfig } from "./deployBilling";
import { deployIncludedCreditCents } from "./deployIncludedCreditLogic";

const config: DeployBillingConfig = {
  planFeePriceIds: {
    starter: "price_fee_starter",
    pro: "price_fee_pro",
    business: "price_fee_business",
  },
  meteredPriceIds: ["price_cpu"],
  allDeployPriceIds: new Set([
    "price_fee_starter",
    "price_fee_pro",
    "price_fee_business",
    "price_cpu",
  ]),
};

function line(priceId: string, amount: number, periodEnd: number): Stripe.InvoiceLineItem {
  return {
    amount,
    discount_amounts: [],
    period: { end: periodEnd, start: periodEnd - 3600 },
    pricing: { type: "price_details", price_details: { price: priceId } },
  } as unknown as Stripe.InvoiceLineItem;
}

function invoice(lines: Stripe.InvoiceLineItem[]): Stripe.Invoice {
  return { lines: { data: lines, has_more: false } } as unknown as Stripe.Invoice;
}

const period = Math.floor(Date.now() / 1000) + 7 * 24 * 60 * 60;

describe("deployIncludedCreditCents", () => {
  it("is the plan fee on subscribe or renewal", () => {
    expect(
      deployIncludedCreditCents(config, [invoice([line("price_fee_business", 5000, period)])]),
    ).toBe(5000);
  });

  it("sums the period across a mid-cycle upgrade (subscribe + proration top-up)", () => {
    // Newest-first: the upgrade proration invoice (net +3000), then the
    // original subscribe invoice (+2000). Period total is their sum, 5000.
    const total = deployIncludedCreditCents(config, [
      invoice([line("price_fee_starter", -2000, period), line("price_fee_business", 5000, period)]),
      invoice([line("price_fee_starter", 2000, period)]),
    ]);
    expect(total).toBe(5000);
  });

  it("is idempotent: recomputing the same invoices never double-counts", () => {
    const invoices = [
      invoice([line("price_fee_starter", -2000, period), line("price_fee_business", 5000, period)]),
      invoice([line("price_fee_starter", 2000, period)]),
    ];
    expect(deployIncludedCreditCents(config, invoices)).toBe(
      deployIncludedCreditCents(config, invoices),
    );
  });

  it("returns the full period total when the latest invoice is a proration (backfill)", () => {
    // Backfill with no prior stored value must still reconstruct the whole
    // period, not just the latest proration delta.
    const total = deployIncludedCreditCents(config, [
      invoice([line("price_fee_starter", -2000, period), line("price_fee_business", 5000, period)]),
      invoice([line("price_fee_starter", 2000, period)]),
    ]);
    expect(total).toBe(5000);
  });

  it("ignores non-positive nets so a downgrade keeps the period credit", () => {
    // A negative-net invoice does not grant, so it neither defines nor lowers
    // the total; the subscribe fee stands.
    const total = deployIncludedCreditCents(config, [
      invoice([line("price_fee_business", -2800, period), line("price_fee_starter", 300, period)]),
      invoice([line("price_fee_business", 5000, period)]),
    ]);
    expect(total).toBe(5000);
  });

  it("returns null with no Deploy plan-fee invoice", () => {
    expect(
      deployIncludedCreditCents(config, [invoice([line("price_cpu", 1000, period)])]),
    ).toBeNull();
    expect(deployIncludedCreditCents(config, [])).toBeNull();
  });

  it("returns null once the period has closed", () => {
    const closed = Math.floor(Date.now() / 1000) - 30 * 24 * 60 * 60;
    expect(
      deployIncludedCreditCents(config, [invoice([line("price_fee_business", 5000, closed)])]),
    ).toBeNull();
  });
});
