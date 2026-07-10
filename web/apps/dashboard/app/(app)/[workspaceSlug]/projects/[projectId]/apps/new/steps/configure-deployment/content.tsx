"use client";

import { DeploymentSettings } from "@/app/(app)/[workspaceSlug]/projects/[projectId]/apps/[appId]/(overview)/settings/deployment-settings";
import { Button, useStepWizard } from "@unkey/ui";
import { useState } from "react";
import { FrameworkDetectionCard } from "./framework-detection-card";

export const ConfigureDeploymentContent = () => {
  const { next } = useStepWizard();
  const [settingsRevision, setSettingsRevision] = useState(0);

  return (
    <div className="w-225">
      <FrameworkDetectionCard onDefaultsApplied={() => setSettingsRevision((value) => value + 1)} />
      <DeploymentSettings key={settingsRevision} githubReadOnly sections={{ build: true }} />
      <div className="flex justify-end mt-6 mb-10 flex-col gap-4">
        <Button type="button" variant="primary" size="xlg" className="rounded-lg" onClick={next}>
          Next
        </Button>
        <span className="text-gray-10 text-[13px] text-center">
          Start configuring your environment variables
        </span>
      </div>
    </div>
  );
};
