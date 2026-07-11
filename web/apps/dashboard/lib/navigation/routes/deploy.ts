import type { Route } from "next";
import { type WorkspaceScope, buildRoute } from "./shared";

export const deployRoutes = {
  root({ workspaceSlug }: WorkspaceScope): Route {
    return buildRoute("/[workspaceSlug]/deploy", { workspaceSlug });
  },
};