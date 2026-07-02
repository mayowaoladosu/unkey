"use client";

import { Plus } from "@unkey/icons";
import { Button } from "@unkey/ui";
import type { ComponentProps } from "react";
import { useState } from "react";
import { CreatePortalDialog } from "./create-portal-dialog";

export function CreatePortalButton({
  size = "md",
}: {
  size?: ComponentProps<typeof Button>["size"];
}) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <Button variant="primary" size={size} onClick={() => setOpen(true)}>
        <Plus iconSize="sm-regular" />
        Create portal
      </Button>
      <CreatePortalDialog isOpen={open} onOpenChange={setOpen} />
    </>
  );
}
