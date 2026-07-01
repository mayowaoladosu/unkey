import { db } from "@/lib/db";
import { ratelimit, withRatelimit, workspaceProcedure } from "@/lib/trpc/trpc";

/**
 * Reports whether the workspace has a Stripe connected account linked for
 * end-user billing. The account reference itself stays encrypted at rest and
 * is not returned.
 */
export const getStripeConnect = workspaceProcedure
  .use(withRatelimit(ratelimit.read))
  .query(async ({ ctx }) => {
    const settings = await db.query.workspaceBillingSettings.findFirst({
      where: (table, { eq }) => eq(table.workspaceId, ctx.workspace.id),
      columns: { stripeConnectEncrypted: true },
    });

    return { linked: Boolean(settings?.stripeConnectEncrypted) };
  });
