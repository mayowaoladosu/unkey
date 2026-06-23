import { getStripeClient } from "@/lib/stripe";
import { handleStripeError } from "@/lib/trpc/routers/utils/stripe";
import {
  ratelimit,
  requireWorkspaceAdmin,
  withRatelimit,
  workspaceProcedure,
} from "@/lib/trpc/trpc";
import { TRPCError } from "@trpc/server";
import Stripe from "stripe";
import { z } from "zod";

const updateCustomerInputSchema = z.object({
  paymentMethod: z.string(),
  // Optional sessionId path: needed for the post-checkout flow where the
  // workspace doesn't yet have a stripeCustomerId. The session must belong to
  // this workspace (`client_reference_id`). Outside that flow the customer is
  // resolved from the workspace itself.
  sessionId: z.string().optional(),
});

const customerSchema = z.object({
  id: z.string(),
});

export const updateCustomer = workspaceProcedure
  .use(requireWorkspaceAdmin)
  .use(withRatelimit(ratelimit.update))
  .input(updateCustomerInputSchema)
  .output(customerSchema)
  .mutation(async ({ ctx, input }) => {
    const stripe = getStripeClient();

    // Never trust a client-supplied customer id. Resolve it server-side so a
    // member can only ever update their own workspace's Stripe customer, not
    // another workspace's (IDOR). Either the session belongs to this workspace
    // or we fall back to the workspace's already-bound customer.
    let resolvedCustomerId: string | null = null;

    if (input.sessionId) {
      const session = await stripe.checkout.sessions.retrieve(input.sessionId);
      if (!session || session.client_reference_id !== ctx.workspace.id) {
        throw new TRPCError({
          code: "NOT_FOUND",
          message: "Customer not found",
        });
      }
      resolvedCustomerId =
        typeof session.customer === "string" ? session.customer : (session.customer?.id ?? null);
    } else if (ctx.workspace.stripeCustomerId) {
      resolvedCustomerId = ctx.workspace.stripeCustomerId;
    }

    if (!resolvedCustomerId) {
      throw new TRPCError({
        code: "NOT_FOUND",
        message: "Customer not found",
      });
    }

    try {
      const customer = await stripe.customers.update(resolvedCustomerId, {
        invoice_settings: {
          default_payment_method: input.paymentMethod,
        },
      });

      if (!customer) {
        throw new TRPCError({
          code: "NOT_FOUND",
          message: "Customer not found or has been deleted",
        });
      }

      return {
        id: customer.id,
      };
    } catch (error) {
      // If error is already a TRPCError, rethrow unchanged
      if (error instanceof TRPCError) {
        throw error;
      }

      // Handle Stripe errors
      if (error instanceof Stripe.errors.StripeError) {
        handleStripeError(error);
      }

      // Handle unknown errors
      throw new TRPCError({
        code: "INTERNAL_SERVER_ERROR",
        message: "Failed to update customer",
      });
    }
  });
