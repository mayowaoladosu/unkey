import {
  deployOverageCents,
  microCentsToCents,
  priceDeployUsageMicroCents,
} from "@/lib/billing/deployPricing";
import { clickhouse } from "@/lib/clickhouse";
import { db, eq, schema } from "@/lib/db";
import { getStripeClient } from "@/lib/stripe";
import { deployIncludedCreditForSubscription } from "@/lib/stripe/deployIncludedCredit";
import { ratelimit, withRatelimit, workspaceProcedure } from "@/lib/trpc/trpc";
import { TRPCError } from "@trpc/server";
import { z } from "zod";

export const queryDeployUsageResponse = z.object({
  cpuSeconds: z.number(),
  memoryGiBHours: z.number(),
  diskGiBHours: z.number(),
  egressGiB: z.number(),
  activeKeys: z.number(),
  /** Month-to-date gross usage priced locally (cents), same math as the spend-cap worker. */
  grossCents: z.number(),
  /**
   * Included usage credit for the period (cents), mirrored from the paid plan
   * fee. Null until subscribe or invoice.payment_succeeded syncs it.
   */
  includedCreditCents: z.number().nullable(),
  /** Billable spend beyond included credits (cents). Null when credit is unknown. */
  overageCents: z.number().nullable(),
});

export type DeployUsageResponse = z.infer<typeof queryDeployUsageResponse>;

/**
 * Month-to-date billable Deploy usage for the workspace, read from the same
 * ClickHouse checkpoint aggregation the hourly billing push uses, so the
 * dashboard shows the quantities that are actually billed.
 */
export const queryDeployUsage = workspaceProcedure
  .use(withRatelimit(ratelimit.read))
  .output(queryDeployUsageResponse)
  .query(async ({ ctx }) => {
    const now = new Date();
    const monthStart = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1);
    let includedCreditCents = ctx.workspace.deployIncludedCreditCents ?? null;

    // Backfill workspaces that subscribed before this column existed: read the
    // latest paid plan-fee invoice once on billing page load instead of waiting
    // for the next webhook or hourly Stripe preview refresh.
    if (includedCreditCents == null && ctx.workspace.stripeSubscriptionId) {
      const resolved = await deployIncludedCreditForSubscription(
        getStripeClient(),
        ctx.workspace.stripeSubscriptionId,
      );
      if (resolved != null) {
        includedCreditCents = resolved;
        await db
          .update(schema.workspaces)
          .set({ deployIncludedCreditCents: resolved })
          .where(eq(schema.workspaces.id, ctx.workspace.id));
      }
    }

    try {
      const [meters, keys] = await Promise.all([
        clickhouse.billing.deployMeterUsage({
          workspaceId: ctx.workspace.id,
          start: monthStart,
          end: now.getTime(),
        }),
        clickhouse.billing.activeKeysUsage({
          workspaceId: ctx.workspace.id,
          year: now.getUTCFullYear(),
          // getUTCMonth is 0-based; the query takes a calendar month.
          month: now.getUTCMonth() + 1,
        }),
      ]);

      const grossMicroCents = priceDeployUsageMicroCents({
        cpuSeconds: meters.cpuSeconds,
        memoryGiBHours: meters.memoryGiBHours,
        diskGiBHours: meters.diskGiBHours,
        egressGiB: meters.egressGiB,
        activeKeys: keys.activeKeys,
      });

      return {
        ...meters,
        activeKeys: keys.activeKeys,
        grossCents: microCentsToCents(grossMicroCents),
        includedCreditCents,
        overageCents: deployOverageCents(grossMicroCents, includedCreditCents),
      };
    } catch (err) {
      console.error("Failed to query deploy usage", err);
      throw new TRPCError({
        code: "INTERNAL_SERVER_ERROR",
        message: "Failed to fetch Deploy usage data. Please try again later.",
      });
    }
  });
