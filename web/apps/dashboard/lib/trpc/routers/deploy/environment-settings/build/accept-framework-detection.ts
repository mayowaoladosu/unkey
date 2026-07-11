import { persistFrameworkDetectionAcceptance } from "@/lib/trpc/routers/deploy/environment-settings/build/framework-detection-acceptance";
import { z } from "zod";
import { workspaceProcedure } from "../../../../trpc";

export const acceptFrameworkDetection = workspaceProcedure
  .input(
    z.object({
      projectId: z.string().min(1),
      appId: z.string().min(1),
      fingerprint: z.string().regex(/^[0-9a-f]{64}$/),
    }),
  )
  .mutation(async ({ ctx, input }) => {
    return await persistFrameworkDetectionAcceptance({
      workspaceId: ctx.workspace.id,
      projectId: input.projectId,
      appId: input.appId,
      fingerprint: input.fingerprint,
      mode: "output",
    });
  });
