import { ActorType } from "@/gen/proto/ctrl/v1/actor_pb";
import type { Context } from "@/lib/trpc/context";
import { Code, ConnectError } from "@connectrpc/connect";
import { TRPCError } from "@trpc/server";
import { z } from "zod";

export const environmentSlugSchema = z
  .string()
  .trim()
  .min(1, "Environment name is required")
  .max(64)
  .regex(/^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$/, "Use lowercase letters, numbers, and hyphens");

export function actorFromContext(ctx: Context & { user: { id: string } }) {
  return {
    id: ctx.user.id,
    type: ActorType.USER,
    remoteIp: ctx.audit.location,
    userAgent: ctx.audit.userAgent ?? "",
  };
}

export function lifecycleError(error: unknown, fallback: string): TRPCError {
  if (error instanceof TRPCError) {
    return error;
  }
  if (error instanceof ConnectError) {
    const code =
      error.code === Code.AlreadyExists
        ? "CONFLICT"
        : error.code === Code.NotFound
          ? "NOT_FOUND"
          : error.code === Code.FailedPrecondition
            ? "PRECONDITION_FAILED"
            : "INTERNAL_SERVER_ERROR";
    return new TRPCError({ code, message: error.rawMessage || fallback, cause: error });
  }
  return new TRPCError({
    code: "INTERNAL_SERVER_ERROR",
    message: error instanceof Error ? error.message : fallback,
    cause: error,
  });
}
