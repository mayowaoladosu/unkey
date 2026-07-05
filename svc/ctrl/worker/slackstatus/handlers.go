package slackstatus

import (
	"context"
	"fmt"
	"time"

	restate "github.com/restatedev/sdk-go"
	hydrav1 "github.com/unkeyed/unkey/gen/proto/hydra/v1"
	vaultv1 "github.com/unkeyed/unkey/gen/proto/vault/v1"
	"github.com/unkeyed/unkey/pkg/logger"
	"github.com/unkeyed/unkey/svc/ctrl/internal/db"
	"github.com/unkeyed/unkey/svc/ctrl/internal/slack"
)

// resolvedTarget is one channel a project fans notifications out to, with its
// per-channel environment scope. EncryptedBotToken is ciphertext, safe to
// journal in Restate; it is decrypted only inside the message-send step so
// plaintext is never persisted.
type resolvedTarget struct {
	ChannelID         string `json:"channel_id"`
	EncryptedBotToken string `json:"encrypted_bot_token"`
	NotifyProduction  bool   `json:"notify_production"`
	NotifyPreviews    bool   `json:"notify_previews"`
}

// resolveResult is the outcome of a connection lookup.
type resolveResult struct {
	Connected bool             `json:"connected"`
	Targets   []resolvedTarget `json:"targets"`
}

// postedMessage identifies one message this object posted so a later state
// change can edit it in place. The ciphertext token rides along so edits still
// work if the channel is disconnected between post and edit.
type postedMessage struct {
	Channel           string `json:"channel"`
	TS                string `json:"ts"`
	EncryptedBotToken string `json:"encrypted_bot_token"`
}

// resolve looks up the project's Slack connections and their installations.
// A project with no connections (or with rows whose installation is gone)
// returns a not-connected zero value rather than an error, so Restate does not
// retry forever on a permanently-absent connection.
func (s *Service) resolve(ctx context.Context, projectID string) (resolveResult, error) {
	conns, err := s.db.ListSlackProjectConnectionsByProjectId(ctx, projectID)
	if err != nil {
		if db.IsNotFound(err) {
			return resolveResult{}, nil //nolint:exhaustruct
		}
		return resolveResult{}, err //nolint:exhaustruct
	}

	// Installations are per (workspace, team); all of a project's rows usually
	// share one, so cache lookups by id.
	tokens := map[string]string{}
	targets := make([]resolvedTarget, 0, len(conns))
	for _, conn := range conns {
		token, ok := tokens[conn.InstallationID]
		if !ok {
			inst, instErr := s.db.FindSlackInstallationById(ctx, conn.InstallationID)
			if instErr != nil {
				if db.IsNotFound(instErr) {
					// Dangling row (installation revoked); skip this channel.
					continue
				}
				return resolveResult{}, instErr //nolint:exhaustruct
			}
			token = inst.BotToken
			tokens[conn.InstallationID] = token
		}
		targets = append(targets, resolvedTarget{
			ChannelID:         conn.ChannelID,
			EncryptedBotToken: token,
			NotifyProduction:  conn.NotifyProduction,
			NotifyPreviews:    conn.NotifyPreviews,
		})
	}

	if len(targets) == 0 {
		return resolveResult{}, nil //nolint:exhaustruct
	}
	return resolveResult{Connected: true, Targets: targets}, nil
}

// decrypt turns the vault-encrypted bot token into plaintext, keyed by
// workspace ID. Called only inside a Restate step so plaintext is not journaled.
func (s *Service) decrypt(ctx context.Context, workspaceID, encrypted string) (string, error) {
	// The worker can run without vault configured (cfg.Vault.URL empty leaves a
	// nil client, see run.go). A nil-interface call would panic and churn through
	// Restate's retry budget; fail with a TerminalError so the enclosing
	// restate.Run gives up immediately (a plain error would retry for the full
	// 30s WithMaxRetryDuration per channel) and the handler hits its
	// log-and-continue path.
	if s.vault == nil {
		return "", restate.TerminalError(fmt.Errorf("vault is not configured; cannot decrypt slack bot token"), 500)
	}
	resp, err := s.vault.Decrypt(ctx, &vaultv1.DecryptRequest{
		Keyring:   workspaceID,
		Encrypted: encrypted,
	})
	if err != nil {
		return "", err
	}
	return resp.GetPlaintext(), nil
}

// deploymentAwaitingApproval reports whether the deployment is still in the
// awaiting_approval state. PostApproval uses it to avoid posting a live prompt
// for a deployment that was already resolved (e.g. via the dashboard) before
// the fire-and-forget PostApproval send arrived. A missing deployment counts as
// not-awaiting so no prompt is posted.
func (s *Service) deploymentAwaitingApproval(ctx context.Context, deploymentID string) (bool, error) {
	deployment, err := s.db.FindDeploymentById(ctx, deploymentID)
	if err != nil {
		if db.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return deployment.Status == db.DeploymentsStatusAwaitingApproval, nil
}

// postToTargets posts the same message to every target, one journaled step per
// channel so a failure on one channel never blocks the others. Returns the
// successfully posted messages.
func (s *Service) postToTargets(ctx restate.ObjectContext, workspaceID, text string, blocks []slack.Block, targets []resolvedTarget, stepPrefix, deploymentID string) []postedMessage {
	posted := make([]postedMessage, 0, len(targets))
	for _, target := range targets {
		msg := slack.Message{
			Channel: target.ChannelID,
			Text:    text,
			Blocks:  blocks,
			TS:      "",
		}
		token := target.EncryptedBotToken
		post, err := restate.Run(ctx, func(rc restate.RunContext) (slack.PostResult, error) {
			plaintext, decErr := s.decrypt(rc, workspaceID, token)
			if decErr != nil {
				return slack.PostResult{}, decErr //nolint:exhaustruct
			}
			return s.slack.PostMessage(rc, plaintext, msg)
		}, restate.WithName(fmt.Sprintf("%s %s", stepPrefix, target.ChannelID)), restate.WithMaxRetryDuration(30*time.Second))
		if err != nil {
			logger.Error("failed to post slack message", "deployment_id", deploymentID, "channel", target.ChannelID, "error", err)
			continue
		}
		posted = append(posted, postedMessage{
			Channel:           post.Channel,
			TS:                post.TS,
			EncryptedBotToken: token,
		})
	}
	return posted
}

// updateMessages edits every previously posted message, one journaled step per
// channel; errors are logged, never propagated.
func (s *Service) updateMessages(ctx restate.ObjectContext, workspaceID, text string, blocks []slack.Block, messages []postedMessage, stepPrefix, deploymentID string) {
	for _, m := range messages {
		msg := slack.Message{
			Channel: m.Channel,
			Text:    text,
			Blocks:  blocks,
			TS:      m.TS,
		}
		token := m.EncryptedBotToken
		if err := restate.RunVoid(ctx, func(rc restate.RunContext) error {
			plaintext, decErr := s.decrypt(rc, workspaceID, token)
			if decErr != nil {
				return decErr
			}
			return s.slack.UpdateMessage(rc, plaintext, msg)
		}, restate.WithName(fmt.Sprintf("%s %s", stepPrefix, m.Channel)), restate.WithMaxRetryDuration(30*time.Second)); err != nil {
			logger.Error("failed to update slack message", "deployment_id", deploymentID, "channel", m.Channel, "error", err)
		}
	}
}

// Init posts the initial outcome message to every in-scope channel and stores
// their identities so a later ReportStatus can edit them. No-ops when the
// project has no Slack connection or no channel covers the environment.
func (s *Service) Init(ctx restate.ObjectContext, req *hydrav1.SlackStatusInitRequest) (*hydrav1.SlackStatusInitResponse, error) {
	deploymentID := restate.Key(ctx)

	resolved, err := restate.Run(ctx, func(rc restate.RunContext) (resolveResult, error) {
		return s.resolve(rc, req.GetProjectId())
	}, restate.WithName("resolve slack connection"), restate.WithMaxRetryDuration(30*time.Second))
	if err != nil {
		logger.Warn("failed to resolve slack connection, skipping notification", "deployment_id", deploymentID, "error", err)
		return &hydrav1.SlackStatusInitResponse{}, nil
	}
	if !resolved.Connected {
		return &hydrav1.SlackStatusInitResponse{}, nil
	}

	// Per-channel environment scoping.
	inScope := make([]resolvedTarget, 0, len(resolved.Targets))
	for _, target := range resolved.Targets {
		if shouldNotifyEnvironment(req.GetIsProduction(), target.NotifyProduction, target.NotifyPreviews) {
			inScope = append(inScope, target)
		}
	}
	if len(inScope) == 0 {
		return &hydrav1.SlackStatusInitResponse{}, nil
	}

	restate.Set(ctx, stateConfig, req)

	state := hydrav1.SlackDeploymentState_SLACK_DEPLOYMENT_STATE_IN_PROGRESS
	posted := s.postToTargets(ctx, req.GetWorkspaceId(), outcomeHeader(state), outcomeBlocks(deploymentID, req, state), inScope, "post slack message", deploymentID)
	if len(posted) > 0 {
		restate.Set(ctx, stateMessages, posted)
	}

	return &hydrav1.SlackStatusInitResponse{}, nil
}

// ReportStatus edits the messages posted by Init to reflect a new state.
// Fire-and-forget — errors are logged, never propagated.
func (s *Service) ReportStatus(ctx restate.ObjectContext, req *hydrav1.SlackStatusReportRequest) (*hydrav1.SlackStatusReportResponse, error) {
	deploymentID := restate.Key(ctx)

	config, err := restate.Get[*hydrav1.SlackStatusInitRequest](ctx, stateConfig)
	if err != nil || config == nil {
		return &hydrav1.SlackStatusReportResponse{}, nil
	}
	messages, _ := restate.Get[[]postedMessage](ctx, stateMessages)
	if len(messages) == 0 {
		return &hydrav1.SlackStatusReportResponse{}, nil
	}

	s.updateMessages(ctx, config.GetWorkspaceId(), outcomeHeader(req.GetState()), outcomeBlocks(deploymentID, config, req.GetState()), messages, "update slack message", deploymentID)

	return &hydrav1.SlackStatusReportResponse{}, nil
}

// PostApproval posts the interactive approval prompt to every connected channel
// for a gated deployment. Approval prompts are not subject to environment
// scoping: gated deployments are external-contributor previews, so scoping them
// out would make the default config never post an actionable prompt.
func (s *Service) PostApproval(ctx restate.ObjectContext, req *hydrav1.SlackPostApprovalRequest) (*hydrav1.SlackPostApprovalResponse, error) {
	deploymentID := restate.Key(ctx)

	// The send is fire-and-forget behind retried steps, so it can arrive after
	// the deployment was already authorized or rejected (e.g. via the
	// dashboard). Re-check status so a late prompt with live buttons is never
	// posted for an already-resolved deployment.
	stillPending, err := restate.Run(ctx, func(rc restate.RunContext) (bool, error) {
		return s.deploymentAwaitingApproval(rc, deploymentID)
	}, restate.WithName("check deployment still awaiting approval"), restate.WithMaxRetryDuration(30*time.Second))
	if err != nil {
		logger.Warn("failed to check deployment status, skipping approval prompt", "deployment_id", deploymentID, "error", err)
		return &hydrav1.SlackPostApprovalResponse{}, nil
	}
	if !stillPending {
		return &hydrav1.SlackPostApprovalResponse{}, nil
	}

	resolved, err := restate.Run(ctx, func(rc restate.RunContext) (resolveResult, error) {
		return s.resolve(rc, req.GetProjectId())
	}, restate.WithName("resolve slack connection"), restate.WithMaxRetryDuration(30*time.Second))
	if err != nil {
		logger.Warn("failed to resolve slack connection, skipping approval prompt", "deployment_id", deploymentID, "error", err)
		return &hydrav1.SlackPostApprovalResponse{}, nil
	}
	if !resolved.Connected {
		return &hydrav1.SlackPostApprovalResponse{}, nil
	}

	posted := s.postToTargets(ctx, req.GetWorkspaceId(), "Deployment awaiting approval", approvalBlocks(deploymentID, req), resolved.Targets, "post slack approval prompt", deploymentID)
	if len(posted) > 0 {
		// Dedicated keys: the outcome messages (Init/ReportStatus) share this
		// virtual object and must not have their identities clobbered.
		restate.Set(ctx, stateApprovalMessages, posted)
		restate.Set(ctx, stateApprovalConfig, req)
	}

	return &hydrav1.SlackPostApprovalResponse{}, nil
}

// ResolveApproval edits the approval prompts to their resolved state, removing
// the Approve/Reject buttons. Fired fire-and-forget by AuthorizeDeployment and
// RejectDeployment so decisions made outside Slack retire the prompts. No-ops
// when no prompt was posted (unconnected project, or the prompt send failed).
func (s *Service) ResolveApproval(ctx restate.ObjectContext, req *hydrav1.SlackResolveApprovalRequest) (*hydrav1.SlackResolveApprovalResponse, error) {
	deploymentID := restate.Key(ctx)

	config, err := restate.Get[*hydrav1.SlackPostApprovalRequest](ctx, stateApprovalConfig)
	if err != nil || config == nil {
		return &hydrav1.SlackResolveApprovalResponse{}, nil
	}
	messages, _ := restate.Get[[]postedMessage](ctx, stateApprovalMessages)
	if len(messages) == 0 {
		return &hydrav1.SlackResolveApprovalResponse{}, nil
	}

	s.updateMessages(
		ctx,
		config.GetWorkspaceId(),
		resolvedApprovalText(req.GetApproved(), req.GetSuperseded()),
		resolvedApprovalBlocks(deploymentID, config, req.GetApproved(), req.GetSuperseded(), req.GetResolvedBy()),
		messages,
		"retire slack approval prompt",
		deploymentID,
	)

	return &hydrav1.SlackResolveApprovalResponse{}, nil
}
