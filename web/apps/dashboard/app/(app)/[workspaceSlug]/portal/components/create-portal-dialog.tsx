"use client";

import { FormCombobox } from "@/components/ui/form-combobox";
import { useWorkspaceNavigation } from "@/hooks/use-workspace-navigation";
import { cn } from "@/lib/utils";
import { Button, DialogContainer } from "@unkey/ui";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import { MOCK_APPS, MOCK_KEYSPACES, type PortalResourceType, createPortal } from "../data/portals";

type Props = {
  isOpen: boolean;
  onOpenChange: (open: boolean) => void;
};

export function CreatePortalDialog({ isOpen, onOpenChange }: Props) {
  const workspace = useWorkspaceNavigation();
  const router = useRouter();

  const [resourceType, setResourceType] = useState<PortalResourceType>("app");
  const [resource, setResource] = useState("");

  const noun = resourceType === "app" ? "app" : "keyspace";

  const options = useMemo(() => {
    const list = resourceType === "app" ? MOCK_APPS : MOCK_KEYSPACES;
    return list.map((r) => ({
      value: r.name,
      label: r.name,
      searchValue: r.name,
      selectedLabel: r.name,
    }));
  }, [resourceType]);

  const reset = () => {
    setResourceType("app");
    setResource("");
  };

  const selectType = (type: PortalResourceType) => {
    setResourceType(type);
    setResource("");
  };

  const onCreate = () => {
    if (!resource) {
      return;
    }
    const id = createPortal({ resourceType, resourceName: resource });
    onOpenChange(false);
    reset();
    router.push(`/${workspace.slug}/portal/${id}`);
  };

  return (
    <DialogContainer
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          reset();
        }
        onOpenChange(open);
      }}
      title="Create portal"
      footer={
        <div className="w-full flex justify-end gap-2">
          <Button variant="outline" size="lg" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button variant="primary" size="lg" disabled={!resource} onClick={onCreate}>
            Create portal
          </Button>
        </div>
      }
    >
      <div className="flex flex-col gap-4">
        <p className="text-gray-11 text-[13px]">
          Connect this portal to one resource. This can't be changed later.
        </p>
        <div className="flex flex-col gap-2">
          <ResourceOption
            selected={resourceType === "app"}
            onSelect={() => selectType("app")}
            title="Deploy app"
            description="Multi-keyspace portal for an app deployed on Unkey"
          />
          <ResourceOption
            selected={resourceType === "keyspace"}
            onSelect={() => selectType("keyspace")}
            title="API keyspace"
            description="Self-serve key management for a single keyspace"
          />
        </div>
        <FormCombobox
          label={resourceType === "app" ? "Select app" : "Select keyspace"}
          requirement="required"
          options={options}
          value={resource}
          onSelect={setResource}
          placeholder={<span className="text-gray-9">{`Search ${noun}s…`}</span>}
          searchPlaceholder={`Search ${noun}s...`}
          emptyMessage={`No ${noun}s found`}
        />
      </div>
    </DialogContainer>
  );
}

function ResourceOption({
  selected,
  onSelect,
  title,
  description,
}: {
  selected: boolean;
  onSelect: () => void;
  title: string;
  description: string;
}) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={cn(
        "text-left rounded-lg border p-3 transition-colors",
        selected ? "border-accent-9 bg-accent-2" : "border-grayA-4 hover:border-grayA-6",
      )}
    >
      <div className="flex items-center gap-2">
        <span
          className={cn(
            "size-4 rounded-full border flex items-center justify-center",
            selected ? "border-accent-9" : "border-grayA-6",
          )}
        >
          {selected && <span className="size-2 rounded-full bg-accent-9" />}
        </span>
        <span className="text-gray-12 text-[13px] font-medium">{title}</span>
      </div>
      <p className="text-gray-11 text-[13px] mt-1 ml-6">{description}</p>
    </button>
  );
}
