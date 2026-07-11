"use client";

import { queryClient } from "@/lib/collections/client";
import { trpc } from "@/lib/trpc/client";
import { Button, toast, useStepWizard } from "@unkey/ui";

type DeployActionProps = {
  projectId: string;
  appId: string;
  disabled?: boolean;
  source?: { type: "github" } | { type: "image"; image: string };
  onDeploymentCreated: (deploymentId: string) => void;
};

export const DeployAction = ({
  projectId,
  appId,
  disabled,
  source = { type: "github" },
  onDeploymentCreated,
}: DeployActionProps) => {
  const { goTo } = useStepWizard();

  const deploy = trpc.deploy.deployment.create.useMutation({
    onSuccess: async (data) => {
      await queryClient.invalidateQueries({ queryKey: ["deployments", projectId] });
      toast.success("Deployment triggered", {
        description: "Your app is being built and deployed",
      });
      onDeploymentCreated(data.deploymentId);
      goTo("deploying");
    },
    onError: (error) => {
      toast.error("Deployment failed", { description: error.message });
    },
  });

  const deployInput =
    source.type === "image"
      ? ({ projectId, appId, environmentSlug: "production", source: "image", image: source.image } as const)
      : ({ projectId, appId, environmentSlug: "production", source: "default" } as const);

  return (
    <div className="flex flex-col gap-3">
      <Button
        type="button"
        variant="primary"
        size="xlg"
        className="rounded-lg"
        disabled={deploy.isLoading || disabled}
        loading={deploy.isLoading}
        onClick={() => deploy.mutate(deployInput)}
      >
        Deploy to production
      </Button>
      <span className="text-gray-9 text-[11px] leading-5 text-center">
        {source.type === "image"
          ? "The image will be pulled, provisioned, health checked, and routed."
          : "Source will be built, provisioned, health checked, and routed."}
      </span>
    </div>
  );
};
