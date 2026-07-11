"use client";

import { useWorkspaceNavigation } from "@/hooks/use-workspace-navigation";
import type { Deployment } from "@/lib/collections";
import { queryClient } from "@/lib/collections/client";
import { routes } from "@/lib/navigation/routes";
import { trpc } from "@/lib/trpc/client";
import { Check, CloudUp } from "@unkey/icons";
import { Button, DialogContainer, toast } from "@unkey/ui";
import { useRouter } from "next/navigation";
import { useProjectData } from "../../../../../data-provider";
import { DeploymentSection } from "./components/deployment-section";

type PromoteToProductionDialogProps = {
  isOpen: boolean;
  onClose: () => void;
  deployment: Deployment;
};

export function PromoteToProductionDialog({
  isOpen,
  onClose,
  deployment,
}: PromoteToProductionDialogProps) {
  const router = useRouter();
  const workspace = useWorkspaceNavigation();
  const { projectId } = useProjectData();
  const promote = trpc.deploy.deployment.promoteToProduction.useMutation({
    onSuccess: async ({ deploymentId }) => {
      await queryClient.invalidateQueries({ queryKey: ["deployments", projectId] });
      toast.success("Production deployment started", {
        description: "Traffic switches only after the production health checks pass.",
      });
      onClose();
      router.push(
        routes.projects.apps.deployment({
          workspaceSlug: workspace.slug,
          projectId: deployment.projectId,
          appId: deployment.appId,
          deploymentId,
          welcome: true,
        }),
      );
    },
    onError: (error) => {
      toast.error("Promotion failed", { description: error.message });
    },
  });

  return (
    <DialogContainer
      isOpen={isOpen}
      onOpenChange={onClose}
      title="Promote to production"
      subTitle="Create a production release from this tested preview. Production settings and secrets remain authoritative."
      footer={
        <Button
          type="button"
          variant="primary"
          size="xlg"
          className="w-full rounded-lg"
          disabled={promote.isLoading}
          loading={promote.isLoading}
          onClick={() => promote.mutate({ deploymentId: deployment.id })}
        >
          <CloudUp iconSize="sm-medium" />
          Promote to production
        </Button>
      }
    >
      <div className="flex flex-col gap-6">
        <DeploymentSection title="Preview deployment" deployment={deployment} isCurrent={false} />
        <div className="rounded-lg border border-grayA-5 bg-grayA-2 p-4">
          <p className="text-xs font-medium text-gray-12">Release plan</p>
          <div className="mt-3 space-y-2 text-xs text-gray-10">
            <PlanItem>
              {deployment.image
                ? "Reuse the immutable built image without rebuilding source"
                : "Build the pinned commit because this preview has no reusable image"}
            </PlanItem>
            <PlanItem>Apply production environment variables, resources, and regions</PlanItem>
            <PlanItem>Run production health checks before assigning live domains</PlanItem>
            <PlanItem>Retain the previous production deployment for instant rollback</PlanItem>
          </div>
        </div>
      </div>
    </DialogContainer>
  );
}

function PlanItem({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex items-start gap-2">
      <span className="mt-0.5 grid size-4 shrink-0 place-items-center rounded-full bg-successA-3 text-success-11">
        <Check className="size-2.5" />
      </span>
      <span>{children}</span>
    </div>
  );
}
