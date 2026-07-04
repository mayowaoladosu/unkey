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

// resolveResult is the outcome of a connection lookup. EncryptedBotToken is
// ciphertext, safe to journal in Restate; it is decrypted only inside the
// message-send step so plaintext is never persisted.
type resolveResult struct {
	Connected         bool   `json:"connected"`
	ChannelID         string `json:"channel_id"`
	EncryptedBotToken string `json:"encrypted_bot_token"`
	IncludePreviews   bool   `json:"include_previews"`
}

// resolve looks up the project's Slack connection and installation. A missing
// connection or installation returns a not-connected zero value rather than an
// error, so Restate does not retry forever on a permanently-absent connection.
func (s *Service) resolve(ctx context.Context, projectID string) (resolveResult, error) {
	conn, err := s.db.FindSlackProjectConnectionByProjectId(ctx, projectID)
	if err != nil {
		if db.IsNotFound(err) {
			return resolveResult{}, nil //nolint:exhaustruct
		}
		return resolveResult{}, err //nolint:exhaustruct
	}

	inst, err := s.db.FindSlackInstallationById(ctx, conn.InstallationID)
	if err != nil {
		if db.IsNotFound(err) {
			return resolveResult{}, nil //nolint:exhaustruct
		}
		return resolveResult{}, err //nolint:exhaustruct
	}

	return resolveResult{
		Connected:         true,
		ChannelID:         conn.ChannelID,
		EncryptedBotToken: inst.BotToken,
		IncludePreviews:   conn.IncludePreviews,
	}, nil
}

// decrypt turns the vault-encrypted bot token into plaintext, keyed by
// workspace ID. Called only inside a Restate step so plaintext is not journaled.
func (s *Service) decrypt(ctx context.Context, workspaceID, encrypted string) (string, error) {
	// The worker can run without vault configured (cfg.Vault.URL empty leaves a
	// nil client, see run.go). A nil-interface call would panic and churn through
	// Restate's retry budget; fail with a terminal error so the handlers hit
	// their existing log-and-return paths instead.
	if s.vault == nil {
		return "", fmt.Errorf("vault is not configured; cannot decrypt slack bot token")
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

// Init posts the initial outcome message and stores the channel/ts so a later
// ReportStatus can edit it. No-ops when the project has no Slack connection or
// the environment is out of scope.
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
	// Environment scoping: skip previews unless the project opted in.
	if !shouldNotifyEnvironment(req.GetIsProduction(), resolved.IncludePreviews) {
		return &hydrav1.SlackStatusInitResponse{}, nil
	}

	restate.Set(ctx, stateConfig, req)

	msg := slack.Message{
		Channel: resolved.ChannelID,
		Text:    outcomeHeader(hydrav1.SlackDeploymentState_SLACK_DEPLOYMENT_STATE_IN_PROGRESS),
		Blocks:  outcomeBlocks(deploymentID, req, hydrav1.SlackDeploymentState_SLACK_DEPLOYMENT_STATE_IN_PROGRESS),
		TS:      "",
	}

	post, err := restate.Run(ctx, func(rc restate.RunContext) (slack.PostResult, error) {
		token, decErr := s.decrypt(rc, req.GetWorkspaceId(), resolved.EncryptedBotToken)
		if decErr != nil {
			return slack.PostResult{}, decErr //nolint:exhaustruct
		}
		return s.slack.PostMessage(rc, token, msg)
	}, restate.WithName("post slack message"), restate.WithMaxRetryDuration(30*time.Second))
	if err != nil {
		logger.Error("failed to post slack message", "deployment_id", deploymentID, "error", err)
		return &hydrav1.SlackStatusInitResponse{}, nil
	}

	restate.Set(ctx, stateChannel, post.Channel)
	restate.Set(ctx, stateTS, post.TS)

	return &hydrav1.SlackStatusInitResponse{}, nil
}

// ReportStatus edits the message posted by Init to reflect a new state.
// Fire-and-forget — errors are logged, never propagated.
func (s *Service) ReportStatus(ctx restate.ObjectContext, req *hydrav1.SlackStatusReportRequest) (*hydrav1.SlackStatusReportResponse, error) {
	deploymentID := restate.Key(ctx)

	config, err := restate.Get[*hydrav1.SlackStatusInitRequest](ctx, stateConfig)
	if err != nil || config == nil {
		return &hydrav1.SlackStatusReportResponse{}, nil
	}
	channel, _ := restate.Get[string](ctx, stateChannel)
	ts, _ := restate.Get[string](ctx, stateTS)
	if channel == "" || ts == "" {
		return &hydrav1.SlackStatusReportResponse{}, nil
	}

	resolved, err := restate.Run(ctx, func(rc restate.RunContext) (resolveResult, error) {
		return s.resolve(rc, config.GetProjectId())
	}, restate.WithName("resolve slack connection"), restate.WithMaxRetryDuration(30*time.Second))
	if err != nil || !resolved.Connected {
		return &hydrav1.SlackStatusReportResponse{}, nil
	}

	msg := slack.Message{
		Channel: channel,
		Text:    outcomeHeader(req.GetState()),
		Blocks:  outcomeBlocks(deploymentID, config, req.GetState()),
		TS:      ts,
	}

	if updateErr := restate.RunVoid(ctx, func(rc restate.RunContext) error {
		token, decErr := s.decrypt(rc, config.GetWorkspaceId(), resolved.EncryptedBotToken)
		if decErr != nil {
			return decErr
		}
		return s.slack.UpdateMessage(rc, token, msg)
	}, restate.WithName("update slack message"), restate.WithMaxRetryDuration(30*time.Second)); updateErr != nil {
		logger.Error("failed to update slack message", "deployment_id", deploymentID, "error", updateErr)
	}

	return &hydrav1.SlackStatusReportResponse{}, nil
}

// PostApproval posts the interactive approval prompt for a gated deployment.
// Approval prompts are not subject to environment scoping: gated deployments are
// external-contributor previews, so scoping them out would make the default
// config never post an actionable prompt.
func (s *Service) PostApproval(ctx restate.ObjectContext, req *hydrav1.SlackPostApprovalRequest) (*hydrav1.SlackPostApprovalResponse, error) {
	deploymentID := restate.Key(ctx)

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

	msg := slack.Message{
		Channel: resolved.ChannelID,
		Text:    "Deployment awaiting approval",
		Blocks:  approvalBlocks(deploymentID, req),
		TS:      "",
	}

	post, err := restate.Run(ctx, func(rc restate.RunContext) (slack.PostResult, error) {
		token, decErr := s.decrypt(rc, req.GetWorkspaceId(), resolved.EncryptedBotToken)
		if decErr != nil {
			return slack.PostResult{}, decErr //nolint:exhaustruct
		}
		return s.slack.PostMessage(rc, token, msg)
	}, restate.WithName("post slack approval prompt"), restate.WithMaxRetryDuration(30*time.Second))
	if err != nil {
		logger.Error("failed to post slack approval prompt", "deployment_id", deploymentID, "error", err)
		return &hydrav1.SlackPostApprovalResponse{}, nil
	}

	restate.Set(ctx, stateChannel, post.Channel)
	restate.Set(ctx, stateTS, post.TS)

	return &hydrav1.SlackPostApprovalResponse{}, nil
}
