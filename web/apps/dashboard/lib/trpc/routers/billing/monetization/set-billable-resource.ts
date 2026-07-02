import { insertAuditLogs } from "@/lib/audit";
import { and, db, eq, schema } from "@/lib/db";
import { newId } from "@unkey/id";
import { z } from "zod";
import { requireWorkspaceAdmin, workspaceProcedure } from "../../../trpc";

export const setBillableResourceInputSchema = z.object({
  resourceType: z.enum(["keyspace", "namespace"]),
  resourceId: z.string().min(1),
  enabled: z.boolean(),
});

/**
 * Enables or disables a keyspace/namespace for end-user billing. Enabling
 * inserts a presence row (idempotent via the unique
 * workspace+type+resource index); disabling removes it. Usage on a resource is
 * billed only while its row exists.
 */
export const setBillableResource = workspaceProcedure
  .use(requireWorkspaceAdmin)
  .input(setBillableResourceInputSchema)
  .mutation(async ({ input, ctx }) => {
    await db.transaction(async (tx) => {
      if (input.enabled) {
        await tx
          .insert(schema.billingBillableResources)
          .values({
            // Opaque unique id; the billing area reuses the rateCard prefix
            // (see set-default.ts) rather than minting a new id namespace.
            id: newId("rateCard"),
            workspaceId: ctx.workspace.id,
            resourceType: input.resourceType,
            resourceId: input.resourceId,
            createdAt: Date.now(),
            updatedAt: null,
          })
          .onDuplicateKeyUpdate({ set: { updatedAt: Date.now() } });
      } else {
        await tx
          .delete(schema.billingBillableResources)
          .where(
            and(
              eq(schema.billingBillableResources.workspaceId, ctx.workspace.id),
              eq(schema.billingBillableResources.resourceType, input.resourceType),
              eq(schema.billingBillableResources.resourceId, input.resourceId),
            ),
          );
      }

      await insertAuditLogs(tx, {
        workspaceId: ctx.workspace.id,
        actor: { type: "user", id: ctx.user.id },
        event: "workspace.update",
        description: `${input.enabled ? "Enabled" : "Disabled"} billing for ${input.resourceType} ${input.resourceId}`,
        resources: [
          {
            type: "workspace",
            id: ctx.workspace.id,
            name: ctx.workspace.name || "Unknown workspace",
            meta: {
              resourceType: input.resourceType,
              resourceId: input.resourceId,
              enabled: input.enabled,
            },
          },
        ],
        context: {
          location: ctx.audit.location,
          userAgent: ctx.audit.userAgent,
        },
      });
    });

    return {
      resourceType: input.resourceType,
      resourceId: input.resourceId,
      enabled: input.enabled,
    };
  });
