import { describe, expect, it } from "vitest";
import { routes } from "./index";

const ws = "acme";
const projectId = "proj_123";
const appId = "app_456";
const deploymentId = "d_789";

describe("project-scoped paths", () => {
  it("builds the list and project base paths", () => {
    expect(routes.projects.list({ workspaceSlug: ws })).toBe("/acme/projects");
    expect(routes.projects.detail({ workspaceSlug: ws, projectId })).toBe(
      "/acme/projects/proj_123",
    );
  });

  it("appends the new-project query when flagged", () => {
    expect(routes.projects.list({ workspaceSlug: ws, new: true })).toBe("/acme/projects?new=true");
  });

  it("builds project leaf paths", () => {
    const scope = { workspaceSlug: ws, projectId };
    expect(routes.projects.settings(scope)).toBe("/acme/projects/proj_123/settings");
  });
});

describe("routes.projects.logs", () => {
  it("omits the query when no app is scoped", () => {
    expect(routes.projects.logs({ workspaceSlug: ws, projectId })).toBe(
      "/acme/projects/proj_123/logs",
    );
  });

  it("scopes logs to an app", () => {
    expect(routes.projects.logs({ workspaceSlug: ws, projectId, appId })).toBe(
      "/acme/projects/proj_123/logs?appId=app_456",
    );
  });
});

describe("routes.projects.requests", () => {
  it("omits the query when nothing is scoped", () => {
    expect(routes.projects.requests({ workspaceSlug: ws, projectId })).toBe(
      "/acme/projects/proj_123/requests",
    );
  });

  it("builds the since + appId query in order", () => {
    expect(routes.projects.requests({ workspaceSlug: ws, projectId, since: "6h", appId })).toBe(
      "/acme/projects/proj_123/requests?since=6h&appId=app_456",
    );
  });

  it("prefixes a deployment id filter with is:", () => {
    expect(
      routes.projects.requests({ workspaceSlug: ws, projectId, since: "6h", deploymentId }),
    ).toBe("/acme/projects/proj_123/requests?since=6h&deploymentId=is:d_789");
  });
});

describe("routes.projects.apps.new", () => {
  it("builds the bare new-app path", () => {
    expect(routes.projects.apps.new({ workspaceSlug: ws, projectId })).toBe(
      "/acme/projects/proj_123/apps/new",
    );
  });

  it("carries the repo-select step and app id", () => {
    expect(
      routes.projects.apps.new({ workspaceSlug: ws, projectId, step: "select-repo", appId }),
    ).toBe("/acme/projects/proj_123/apps/new?step=select-repo&appId=app_456");
  });

  it("carries a prebuilt image into configuration", () => {
    expect(
      routes.projects.apps.new({
        workspaceSlug: ws,
        projectId,
        step: "configure-deployment",
        appId,
        source: "image",
        image: "ghcr.io/acme/api:latest",
      }),
    ).toBe(
      "/acme/projects/proj_123/apps/new?step=configure-deployment&appId=app_456&source=image&image=ghcr.io/acme/api:latest",
    );
  });
});

describe("app-scoped paths", () => {
  const scope = { workspaceSlug: ws, projectId, appId };

  it("builds app leaf paths", () => {
    expect(routes.projects.apps.settings(scope)).toBe(
      "/acme/projects/proj_123/apps/app_456/settings",
    );
    expect(routes.projects.apps.deployments(scope)).toBe(
      "/acme/projects/proj_123/apps/app_456/deployments",
    );
    expect(routes.projects.apps.environments(scope)).toBe(
      "/acme/projects/proj_123/apps/app_456/environments",
    );
  });

  it("scopes app settings to one environment", () => {
    expect(routes.projects.apps.settings({ ...scope, environmentId: "env_staging" })).toBe(
      "/acme/projects/proj_123/apps/app_456/settings?environmentId=env_staging",
    );
  });

  it("builds a deployment path", () => {
    expect(routes.projects.apps.deployment({ ...scope, deploymentId })).toBe(
      "/acme/projects/proj_123/apps/app_456/deployments/d_789",
    );
  });

  it("flags a build deployment", () => {
    expect(routes.projects.apps.deployment({ ...scope, deploymentId, build: true })).toBe(
      "/acme/projects/proj_123/apps/app_456/deployments/d_789?build=true",
    );
  });

  it("flags the first-deploy welcome handoff", () => {
    expect(routes.projects.apps.deployment({ ...scope, deploymentId, welcome: true })).toBe(
      "/acme/projects/proj_123/apps/app_456/deployments/d_789?welcome=true",
    );
  });

  it("builds every deployment workspace destination", () => {
    const deploymentScope = { ...scope, deploymentId };
    expect(routes.projects.apps.deploymentLogs(deploymentScope)).toBe(
      "/acme/projects/proj_123/apps/app_456/deployments/d_789/logs",
    );
    expect(routes.projects.apps.deploymentResources(deploymentScope)).toBe(
      "/acme/projects/proj_123/apps/app_456/deployments/d_789/resources",
    );
    expect(routes.projects.apps.deploymentSource(deploymentScope)).toBe(
      "/acme/projects/proj_123/apps/app_456/deployments/d_789/source",
    );
    expect(routes.projects.apps.deploymentNetwork(deploymentScope)).toBe(
      "/acme/projects/proj_123/apps/app_456/deployments/d_789/network",
    );
  });

  it("builds an openapi diff path", () => {
    expect(routes.projects.apps.openapiDiff({ ...scope, from: "dep_old", to: "dep_new" })).toBe(
      "/acme/projects/proj_123/apps/app_456/openapi-diff?from=dep_old&to=dep_new",
    );
  });
});
