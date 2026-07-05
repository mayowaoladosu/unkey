package deployment

import (
	"context"

	restateingress "github.com/restatedev/sdk-go/ingress"
	"github.com/unkeyed/unkey/gen/proto/ctrl/v1/ctrlv1connect"
	hydrav1 "github.com/unkeyed/unkey/gen/proto/hydra/v1"
	"github.com/unkeyed/unkey/pkg/logger"
	restateadmin "github.com/unkeyed/unkey/pkg/restate/admin"
	"github.com/unkeyed/unkey/svc/ctrl/dedup"
	"github.com/unkeyed/unkey/svc/ctrl/internal/auditlogs"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
	githubclient "github.com/unkeyed/unkey/svc/ctrl/worker/github"
)

// Service implements the DeployService ConnectRPC API. It coordinates
// deployment operations by persisting state to the database and delegating
// workflow execution to Restate.
type Service struct {
	ctrlv1connect.UnimplementedDeployServiceHandler
	db                              db.Database
	restate                         *restateingress.Client
	restateAdmin                    *restateadmin.Client
	github                          githubclient.GitHubClient
	auditlogs                       auditlogs.AuditLogService
	allowUnauthenticatedDeployments bool
	bearer                          string
	dedup                           *dedup.Service
}

// deploymentClient creates a typed Restate ingress client for the DeployService
// keyed by deployment_id. Each deployment runs as its own isolated workflow,
// so multiple deployments per environment can build in parallel. The contended
// resource (apps.current_deployment_id) is serialized inside RoutingService
// via SwapLiveDeployment.
func (s *Service) deploymentClient(deploymentID string) hydrav1.DeployServiceIngressClient {
	return hydrav1.NewDeployServiceIngressClient(s.restate, deploymentID)
}

// resolveSlackApproval fire-and-forgets a SlackStatusService.ResolveApproval so
// a decision made outside Slack (dashboard, API) — or from a Slack click —
// retires the approval prompt's Approve/Reject buttons and renders the outcome.
// resolvedBy is the actor's display name for attribution (empty when unknown).
// Best-effort: errors are logged, never propagated, and the service no-ops when
// the deployment never had a prompt posted.
func (s *Service) resolveSlackApproval(ctx context.Context, deploymentID string, approved bool, resolvedBy string) {
	if s.restate == nil {
		return
	}
	if _, err := hydrav1.NewSlackStatusServiceIngressClient(s.restate, deploymentID).
		ResolveApproval().Send(ctx, &hydrav1.SlackResolveApprovalRequest{
		Approved:   approved,
		ResolvedBy: resolvedBy,
		Superseded: false,
	}); err != nil {
		logger.Warn("failed to retire slack approval prompt",
			"deployment_id", deploymentID,
			"error", err,
		)
	}
}

// Config holds the configuration for creating a new [Service].
type Config struct {
	// Database provides read/write access to deployment metadata.
	Database db.Database
	// Restate is the ingress client for triggering durable workflows.
	Restate *restateingress.Client
	// RestateAdmin is used to cancel in-flight invocations when a user
	// manually aborts a deployment. Optional — when nil, CancelDeployment
	// will fail closed.
	RestateAdmin *restateadmin.Client
	// GitHub is the client for GitHub API operations (fetching HEAD, etc.).
	GitHub githubclient.GitHubClient
	// Auditlogs records audit events for deployment operations (e.g.
	// operator-triggered rebuilds) so they show up in the customer's audit
	// feed. Required.
	Auditlogs auditlogs.AuditLogService
	// AllowUnauthenticatedDeployments toggles the public GitHub API path for
	// local development with public repositories. Production keeps this false.
	AllowUnauthenticatedDeployments bool
	// Bearer is the preshared token that callers must provide in the Authorization header.
	Bearer string
}

// New creates a new [Service] with the given configuration.
func New(cfg Config) *Service {
	return &Service{
		UnimplementedDeployServiceHandler: ctrlv1connect.UnimplementedDeployServiceHandler{},
		db:                                cfg.Database,
		restate:                           cfg.Restate,
		restateAdmin:                      cfg.RestateAdmin,
		github:                            cfg.GitHub,
		auditlogs:                         cfg.Auditlogs,
		allowUnauthenticatedDeployments:   cfg.AllowUnauthenticatedDeployments,
		bearer:                            cfg.Bearer,
		dedup:                             dedup.New(cfg.Database, cfg.RestateAdmin),
	}
}
