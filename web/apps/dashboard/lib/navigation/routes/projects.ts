/**
 * Route builders for the /projects area, exposed as one nested object so call
 * sites read like the url hierarchy: `routes.projects.apps.settings(scope)`.
 * Every navigable result goes through buildRoute, which checks the bracket
 * pattern against Next's generated route table (typedRoutes) and types the
 * params from the generated ParamMap.
 */
import type { Route } from "next";
import { type WorkspaceScope, buildRoute } from "./shared";

type ProjectScope = WorkspaceScope & { projectId: string };
type AppScope = ProjectScope & { appId: string };

export const projectRoutes = {
  list({ workspaceSlug, new: isNew }: WorkspaceScope & { new?: boolean }): Route {
    return buildRoute("/[workspaceSlug]/projects", { workspaceSlug }, { new: isNew || undefined });
  },

  detail(scope: ProjectScope): Route {
    return buildRoute("/[workspaceSlug]/projects/[projectId]", projectParams(scope));
  },

  settings(scope: ProjectScope): Route {
    return buildRoute("/[workspaceSlug]/projects/[projectId]/settings", projectParams(scope));
  },

  logs({
    appId,
    deploymentId,
    ...scope
  }: ProjectScope & { appId?: string; deploymentId?: string }): Route {
    return buildRoute("/[workspaceSlug]/projects/[projectId]/logs", projectParams(scope), {
      appId,
      deploymentId: deploymentId ? isFilter(deploymentId) : undefined,
    });
  },

  requests({
    since,
    appId,
    deploymentId,
    ...scope
  }: ProjectScope & { since?: string; appId?: string; deploymentId?: string }): Route {
    return buildRoute("/[workspaceSlug]/projects/[projectId]/requests", projectParams(scope), {
      since,
      appId,
      deploymentId: deploymentId ? isFilter(deploymentId) : undefined,
    });
  },

  apps: {
    new({
      step,
      appId,
      source,
      image,
      ...scope
    }: ProjectScope & {
      step?: string;
      appId?: string;
      source?: "github" | "image";
      image?: string;
    }): Route {
      return buildRoute("/[workspaceSlug]/projects/[projectId]/apps/new", projectParams(scope), {
        step,
        appId,
        source,
        image,
      });
    },

    overview(scope: AppScope): Route {
      return buildRoute(
        "/[workspaceSlug]/projects/[projectId]/apps/[appId]/overview",
        appParams(scope),
      );
    },

    settings(scope: AppScope): Route {
      return buildRoute(
        "/[workspaceSlug]/projects/[projectId]/apps/[appId]/settings",
        appParams(scope),
      );
    },

    envVars(scope: AppScope): Route {
      return buildRoute(
        "/[workspaceSlug]/projects/[projectId]/apps/[appId]/env-vars",
        appParams(scope),
      );
    },

    environments(scope: AppScope): Route {
      return buildRoute(
        "/[workspaceSlug]/projects/[projectId]/apps/[appId]/environments",
        appParams(scope),
      );
    },

    sentinelPolicies(scope: AppScope): Route {
      return buildRoute(
        "/[workspaceSlug]/projects/[projectId]/apps/[appId]/sentinel-policies",
        appParams(scope),
      );
    },

    deployments(scope: AppScope): Route {
      return buildRoute(
        "/[workspaceSlug]/projects/[projectId]/apps/[appId]/deployments",
        appParams(scope),
      );
    },

    deployment({
      deploymentId,
      build,
      welcome,
      ...scope
    }: AppScope & { deploymentId: string; build?: boolean; welcome?: boolean }): Route {
      return buildRoute(
        "/[workspaceSlug]/projects/[projectId]/apps/[appId]/deployments/[deploymentId]",
        { ...appParams(scope), deploymentId },
        { build: build || undefined, welcome: welcome || undefined },
      );
    },

    deploymentLogs(scope: AppScope & { deploymentId: string }): Route {
      return buildRoute(
        "/[workspaceSlug]/projects/[projectId]/apps/[appId]/deployments/[deploymentId]/logs",
        { ...appParams(scope), deploymentId: scope.deploymentId },
      );
    },

    deploymentResources(scope: AppScope & { deploymentId: string }): Route {
      return buildRoute(
        "/[workspaceSlug]/projects/[projectId]/apps/[appId]/deployments/[deploymentId]/resources",
        { ...appParams(scope), deploymentId: scope.deploymentId },
      );
    },

    deploymentSource(scope: AppScope & { deploymentId: string }): Route {
      return buildRoute(
        "/[workspaceSlug]/projects/[projectId]/apps/[appId]/deployments/[deploymentId]/source",
        { ...appParams(scope), deploymentId: scope.deploymentId },
      );
    },

    deploymentNetwork(scope: AppScope & { deploymentId: string }): Route {
      return buildRoute(
        "/[workspaceSlug]/projects/[projectId]/apps/[appId]/deployments/[deploymentId]/network",
        { ...appParams(scope), deploymentId: scope.deploymentId },
      );
    },

    openapiDiff({ from, to, ...scope }: AppScope & { from?: string; to?: string }): Route {
      return buildRoute(
        "/[workspaceSlug]/projects/[projectId]/apps/[appId]/openapi-diff",
        appParams(scope),
        { from, to },
      );
    },
  },
};

function projectParams({ workspaceSlug, projectId }: ProjectScope) {
  return { workspaceSlug, projectId };
}

function appParams({ appId, ...scope }: AppScope) {
  return { ...projectParams(scope), appId };
}

/**
 * `is:` is the logs/requests table filter syntax for a deployment id. Both
 * backends match the id exactly, and the deployment filter UI emits `is`,
 * so links use the same operator to keep filter chips consistent.
 */
function isFilter(deploymentId: string): `is:${string}` {
  return `is:${deploymentId}`;
}
