// Package urls builds dashboard-facing URLs that are shared across ctrl workers,
// so the deployment link format stays identical between the deploy workflow and
// the github webhook approval path instead of being duplicated (and drifting).
package urls

import "fmt"

// DeploymentLogURL returns the dashboard URL for a deployment's detail page.
//
// It MUST stay in sync with the Next.js route at
// web/apps/dashboard/app/(app)/[workspaceSlug]/projects/[projectId]/apps/[appId]/(overview)/deployments/[deploymentId].
// The (overview) segment is a route group and does not appear in the path; the
// apps/<appId> segment does, and omitting it 404s.
func DeploymentLogURL(dashboardURL, workspaceSlug, projectID, appID, deploymentID string) string {
	return fmt.Sprintf(
		"%s/%s/projects/%s/apps/%s/deployments/%s",
		dashboardURL, workspaceSlug, projectID, appID, deploymentID,
	)
}
