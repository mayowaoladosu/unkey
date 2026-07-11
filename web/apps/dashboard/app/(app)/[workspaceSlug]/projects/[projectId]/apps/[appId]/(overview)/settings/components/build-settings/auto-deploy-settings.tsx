"use client";

import { Switch } from "@/components/ui/switch";
import { collection } from "@/lib/collections";
import type { EnvironmentSettings } from "@/lib/collections/deploy/environment-settings";
import { HalfDottedCirclePlay } from "@unkey/icons";
import { SettingCard } from "@unkey/ui";
import { useEffect } from "react";
import { useForm, useWatch } from "react-hook-form";
import { useProjectData } from "../../../data-provider";
import { useEnvironmentSettings } from "../../environment-provider";
import { useMultiEnvironmentSettings } from "../../hooks/use-multi-environment-settings";
import { SettingDescription } from "../shared/form-blocks";
import { FormSettingCard, resolveSaveState } from "../shared/form-setting-card";
import { SelectedConfig } from "../shared/selected-config";

type AutoDeployTarget = {
  label: string;
  description: string;
  settings: EnvironmentSettings;
};

export const AutoDeploy = () => {
  const { settings, variant } = useEnvironmentSettings();
  const { environments } = useProjectData();
  const selectedEnvironment = environments.find(
    (environment) => environment.id === settings.environmentId,
  );

  if (variant === "environment") {
    if (
      selectedEnvironment &&
      selectedEnvironment.slug !== "production" &&
      selectedEnvironment.slug !== "preview"
    ) {
      return <ManualAutoDeploy environmentSlug={selectedEnvironment.slug} />;
    }

    const slug = selectedEnvironment?.slug ?? "environment";
    return (
      <AutoDeployEditor
        targets={[
          {
            label: titleCase(slug),
            description:
              slug === "production"
                ? "pushes to the default branch"
                : "pushes to non-default branches",
            settings,
          },
        ]}
      />
    );
  }

  return <MultiEnvironmentAutoDeploy />;
};

const MultiEnvironmentAutoDeploy = () => {
  const multiSettings = useMultiEnvironmentSettings();
  if (!multiSettings) {
    return null;
  }
  return (
    <AutoDeployEditor
      targets={[
        {
          label: "Production",
          description: "pushes to the default branch",
          settings: multiSettings.production,
        },
        {
          label: "Preview",
          description: "pushes to non-default branches",
          settings: multiSettings.preview,
        },
      ]}
    />
  );
};

const AutoDeployEditor = ({ targets }: { targets: AutoDeployTarget[] }) => {
  const defaultSignature = targets
    .map((target) => `${target.settings.environmentId}:${target.settings.autoDeploy}`)
    .join("|");
  const getDefaultValues = () =>
    Object.fromEntries(
      targets.map((target) => [target.settings.environmentId, target.settings.autoDeploy]),
    );

  const {
    handleSubmit,
    setValue,
    formState: { isSubmitting },
    control,
    reset,
  } = useForm<Record<string, boolean>>({
    mode: "onChange",
    defaultValues: getDefaultValues(),
  });

  useEffect(() => {
    reset(getDefaultValues());
    // The signature changes only when the target environments or persisted
    // auto-deploy values change; the targets array itself is created by the parent.
    // biome-ignore lint/correctness/useExhaustiveDependencies: see comment above
  }, [defaultSignature, reset]);

  const currentValues = useWatch({ control });

  const onSubmit = async (values: Record<string, boolean>) => {
    for (const target of targets) {
      const environmentId = target.settings.environmentId;
      if (values[environmentId] !== target.settings.autoDeploy) {
        collection.environmentSettings.update(environmentId, (draft) => {
          draft.autoDeploy = values[environmentId];
        });
      }
    }
  };

  const hasChanges = targets.some(
    (target) =>
      (currentValues[target.settings.environmentId] ?? target.settings.autoDeploy) !==
      target.settings.autoDeploy,
  );

  const saveState = resolveSaveState([
    [isSubmitting, { status: "saving" }],
    [!hasChanges, { status: "disabled", reason: "No changes to save" }],
  ]);

  return (
    <FormSettingCard
      icon={<HalfDottedCirclePlay className="text-gray-12" iconSize="xl-medium" />}
      title="Auto deploy"
      description="Automatically trigger deployments when code is pushed to GitHub."
      displayValue={
        <div className="flex items-center gap-3">
          {targets.map((target, index) => (
            <div key={target.settings.environmentId} className="contents">
              {index > 0 ? <span className="text-gray-8">|</span> : null}
              <span className="space-x-1">
                <span className="text-gray-11 text-xs font-normal">{target.label}</span>
                <span className="font-medium text-gray-12">
                  {target.settings.autoDeploy ? "On" : "Off"}
                </span>
              </span>
            </div>
          ))}
        </div>
      }
      onSubmit={handleSubmit(onSubmit)}
      saveState={saveState}
      footerLeft={
        <SettingDescription>
          When disabled, you can still deploy manually from the dashboard.
        </SettingDescription>
      }
    >
      <div className="flex flex-col gap-1" data-form-wide>
        {targets.map((target) => (
          <EnvRow
            key={target.settings.environmentId}
            label={target.label}
            description={target.description}
            checked={
              currentValues[target.settings.environmentId] ?? target.settings.autoDeploy
            }
            onChange={(value) =>
              setValue(target.settings.environmentId, value, { shouldDirty: true })
            }
          />
        ))}
      </div>
    </FormSettingCard>
  );
};

const ManualAutoDeploy = ({ environmentSlug }: { environmentSlug: string }) => (
  <SettingCard
    className="px-4 py-[18px]"
    icon={<HalfDottedCirclePlay className="text-gray-12" iconSize="xl-medium" />}
    title="Auto deploy"
    description={`${titleCase(environmentSlug)} is an independent environment without a Git branch rule. Create deployments manually from Deploy.`}
    contentWidth="w-full lg:w-[320px] justify-end"
  >
    <SelectedConfig label={<span className="text-gray-11 font-normal">Manual</span>} />
  </SettingCard>
);

function titleCase(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

const EnvRow = ({
  label,
  description,
  checked,
  onChange,
}: {
  label: string;
  description: string;
  checked: boolean;
  onChange: (value: boolean) => void;
}) => (
  <div className="flex items-center gap-3 py-1.5 cursor-pointer">
    <Switch checked={checked} onCheckedChange={onChange} size="sm" />
    <span className="text-sm text-gray-12">
      <span className="font-medium">{label}</span>
      <span className="text-gray-9"> — {description}</span>
    </span>
  </div>
);
