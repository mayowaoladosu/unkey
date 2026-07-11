"use client";

import { Plus } from "@unkey/icons";
import { Button, InfoTooltip } from "@unkey/ui";
import { routes } from "@/lib/navigation/routes";
import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { useDeployGate } from "./hooks/use-deploy-gate";

type Props = {
  defaultOpen?: boolean;
  workspaceSlug: string;
};

export function CreateProjectButton({ defaultOpen, workspaceSlug }: Props) {
  const router = useRouter();

  // UX-only mirror of the authoritative ctrl-api gate, so a gated user gets a
  // disabled button into the paywall instead of a request that fails.
  const { gated, isLoading } = useDeployGate();

  useEffect(() => {
    if (defaultOpen && !gated && !isLoading) {
      router.replace(routes.deploy.root({ workspaceSlug }));
    }
  }, [defaultOpen, gated, isLoading, router, workspaceSlug]);

  return (
    <>
      <InfoTooltip
        content="A Compute plan is required to create projects."
        disabled={!gated}
        asChild
      >
        <span>
          <Button
            size="md"
            variant="primary"
            loading={isLoading}
            disabled={gated}
            onClick={() => router.push(routes.deploy.root({ workspaceSlug }))}
          >
            <Plus iconSize="sm-regular" />
            Deploy project
          </Button>
        </span>
      </InfoTooltip>
    </>
  );
}
