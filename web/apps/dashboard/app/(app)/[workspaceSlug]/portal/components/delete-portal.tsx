"use client";

import { useWorkspaceNavigation } from "@/hooks/use-workspace-navigation";
import { Button, DialogContainer, Input, SettingsZoneRow } from "@unkey/ui";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { type Portal, deletePortal } from "../data/portals";

export function DeletePortal({ portal }: { portal: Portal }) {
  const workspace = useWorkspaceNavigation();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [confirm, setConfirm] = useState("");

  const isValid = confirm === portal.resourceName;

  const onDelete = () => {
    if (!isValid) {
      return;
    }
    deletePortal(portal.id);
    setOpen(false);
    router.push(`/${workspace.slug}/portal`);
  };

  return (
    <>
      <SettingsZoneRow
        title="Delete portal"
        description="Permanently removes this portal. Existing sessions stop working. This action cannot be undone."
        action={{ label: "Delete portal", onClick: () => setOpen(true) }}
      />
      <DialogContainer
        isOpen={open}
        onOpenChange={(o) => {
          if (!o) {
            setConfirm("");
          }
          setOpen(o);
        }}
        title="Delete portal"
        footer={
          <div className="w-full flex flex-col gap-2 items-center justify-center">
            <Button
              type="button"
              variant="primary"
              color="danger"
              size="xlg"
              className="w-full"
              disabled={!isValid}
              onClick={onDelete}
            >
              Delete portal
            </Button>
            <div className="text-gray-9 text-xs">
              This action cannot be undone – proceed with caution
            </div>
          </div>
        }
      >
        <div className="flex flex-col gap-1">
          <p className="text-gray-11 text-[13px]">
            Type <span className="text-gray-12 font-medium">{portal.resourceName}</span> to confirm
          </p>
          <Input
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
            placeholder={`Enter "${portal.resourceName}" to confirm`}
          />
        </div>
      </DialogContainer>
    </>
  );
}
