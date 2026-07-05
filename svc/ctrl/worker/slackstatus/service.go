// Package slackstatus implements the SlackStatusService Restate virtual object,
// keyed by deployment ID. It owns all outbound Slack messaging for a deployment
// (outcome notifications and the approval prompt): the per-project connection
// lookup, the vault bot-token decrypt, and the posted message's channel+ts so a
// later state change edits the same message. It no-ops when the project has no
// Slack connection.
package slackstatus

import (
	hydrav1 "github.com/unkeyed/unkey/gen/proto/hydra/v1"
	"github.com/unkeyed/unkey/gen/rpc/vault"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
	"github.com/unkeyed/unkey/svc/ctrl/internal/slack"
)

// Restate K/V state keys.
const (
	// stateMessages holds the []postedMessage identities of the outcome
	// messages posted by Init (one per in-scope channel) so ReportStatus can
	// edit them in place.
	stateMessages = "messages"
	// stateConfig holds the Init request so ReportStatus can re-render.
	stateConfig = "config"

	// The approval prompt keeps its own message identities and config, separate
	// from the outcome messages: Init and PostApproval share the same virtual
	// object key (the deployment ID), and after an approval the deploy workflow's
	// Init would otherwise overwrite the prompt state before ResolveApproval
	// edits it.
	stateApprovalMessages = "approval_messages"
	stateApprovalConfig   = "approval_config"
)

// Service is the SlackStatusService virtual object.
type Service struct {
	hydrav1.UnimplementedSlackStatusServiceServer
	slack *slack.WebClient
	vault vault.VaultServiceClient
	db    db.Database
}

var _ hydrav1.SlackStatusServiceServer = (*Service)(nil)

// Config holds the dependencies required to create a Service.
type Config struct {
	Slack *slack.WebClient
	Vault vault.VaultServiceClient
	DB    db.Database
}

// New creates a new SlackStatusService virtual object.
func New(cfg Config) *Service {
	return &Service{
		UnimplementedSlackStatusServiceServer: hydrav1.UnimplementedSlackStatusServiceServer{},
		slack:                                 cfg.Slack,
		vault:                                 cfg.Vault,
		db:                                    cfg.DB,
	}
}
