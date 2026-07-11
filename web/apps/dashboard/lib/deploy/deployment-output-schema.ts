import { z } from "zod";

export const deploymentOutputBindingSchema = z.object({
  name: z.string().regex(/^[A-Z][A-Z0-9_]{0,63}$/),
  resource: z.string().min(1),
  protocol: z.enum(["http", "tcp"]).optional(),
});

export const deploymentOutputSchema = z.object({
  kind: z.enum(["container", "static", "function", "worker", "cron"]),
  name: z.string().regex(/^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$/),
  port: z.number().int().min(1).max(65_535).optional(),
  upstreamProtocol: z.enum(["http1", "h2c"]).optional(),
  directory: z.string().min(1).optional(),
  spaFallback: z.boolean().optional(),
  runtime: z.string().min(1).optional(),
  handler: z.string().min(1).optional(),
  command: z.array(z.string().min(1)).optional(),
  schedule: z.string().min(1).optional(),
  public: z.boolean().optional(),
  bindings: z.array(deploymentOutputBindingSchema).optional(),
});

export const deploymentOutputsSchema = z.array(deploymentOutputSchema).max(32).superRefine((outputs, ctx) => {
  const byName = new Map(outputs.map((output) => [output.name, output]));
  if (byName.size !== outputs.length) {
    ctx.addIssue({ code: "custom", message: "Resource names must be unique" });
  }
  if (outputs.filter((output) => output.public).length > 1) {
    ctx.addIssue({ code: "custom", message: "Only one resource can be public" });
  }

  for (const [index, output] of outputs.entries()) {
    const path = (field: string) => [index, field];
    if (output.kind === "container" && output.port === undefined) {
      ctx.addIssue({ code: "custom", path: path("port"), message: "Services require a port" });
    }
    if (output.kind === "static" && !output.directory) {
      ctx.addIssue({ code: "custom", path: path("directory"), message: "Static outputs require a directory" });
    }
    if (output.kind === "function" && (!output.runtime || !output.handler)) {
      ctx.addIssue({ code: "custom", path: path("runtime"), message: "Functions require a runtime and handler" });
    }
    if (output.kind === "worker" && !output.command?.length) {
      ctx.addIssue({ code: "custom", path: path("command"), message: "Workers require a command" });
    }
    if (output.kind === "cron" && (!output.command?.length || !output.schedule)) {
      ctx.addIssue({ code: "custom", path: path("schedule"), message: "Cron jobs require a schedule and command" });
    }
    if ((output.kind === "worker" || output.kind === "cron") && output.public) {
      ctx.addIssue({ code: "custom", path: path("public"), message: "Workers and cron jobs cannot be public" });
    }

    const bindingNames = new Set<string>();
    for (const binding of output.bindings ?? []) {
      if (bindingNames.has(binding.name)) {
        ctx.addIssue({ code: "custom", path: path("bindings"), message: `Binding ${binding.name} is duplicated` });
      }
      bindingNames.add(binding.name);
      const target = byName.get(binding.resource);
      if (!target) {
        ctx.addIssue({ code: "custom", path: path("bindings"), message: `Binding ${binding.name} targets an unknown resource` });
      } else if (target.name === output.name) {
        ctx.addIssue({ code: "custom", path: path("bindings"), message: `Binding ${binding.name} cannot target itself` });
      } else if (target.kind !== "container" && target.kind !== "function") {
        ctx.addIssue({ code: "custom", path: path("bindings"), message: `Binding ${binding.name} must target a service or function` });
      }
    }
  }
});

export type DeploymentOutput = z.infer<typeof deploymentOutputSchema>;
