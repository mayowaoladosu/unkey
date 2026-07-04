package slackstatus

import (
	"fmt"
	"strings"

	hydrav1 "github.com/unkeyed/unkey/gen/proto/hydra/v1"
	"github.com/unkeyed/unkey/svc/ctrl/internal/slack"
)

// mrkdwnEscaper neutralises Slack mrkdwn control sequences in untrusted text.
// Commit messages and sender logins on the approval path come from external
// fork contributors; without escaping, `<!channel>` pings or fake
// `<https://evil|label>` links could be injected into the trusted approval
// prompt that carries live Approve/Reject buttons. Per Slack's docs, only
// &, <, > need escaping in mrkdwn text.
var mrkdwnEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// escapeMrkdwn escapes untrusted text for safe interpolation into mrkdwn fields.
func escapeMrkdwn(s string) string {
	return mrkdwnEscaper.Replace(s)
}

// approvalBlockIDPrefix namespaces the actions block_id so the interactivity
// handler can recognise a deployment-approval interaction. The full block_id is
// "<prefix>:<deploymentID>:<workspaceID>". The handler treats these as lookup
// keys only and re-derives tenancy server-side.
const approvalBlockIDPrefix = "slack_deploy_approval"

// shouldNotifyEnvironment reports whether an outcome notification should fire
// for a deployment. Production always notifies; previews only when the project
// opted in (KTD8, AE1).
func shouldNotifyEnvironment(isProduction, includePreviews bool) bool {
	return isProduction || includePreviews
}

// outcomeHeader returns the header line for a given deployment state.
func outcomeHeader(state hydrav1.SlackDeploymentState) string {
	switch state {
	case hydrav1.SlackDeploymentState_SLACK_DEPLOYMENT_STATE_READY:
		return "✅ Deployment ready"
	case hydrav1.SlackDeploymentState_SLACK_DEPLOYMENT_STATE_FAILED:
		return "❌ Deployment failed"
	case hydrav1.SlackDeploymentState_SLACK_DEPLOYMENT_STATE_IN_PROGRESS,
		hydrav1.SlackDeploymentState_SLACK_DEPLOYMENT_STATE_UNSPECIFIED:
		return "🚀 Deploying…"
	default:
		return "🚀 Deploying…"
	}
}

// commitField renders "<sha> message", trimming a long commit message.
func commitField(sha, message string) string {
	short := sha
	if len(short) > 7 {
		short = short[:7]
	}
	if message == "" {
		return fmt.Sprintf("`%s`", short)
	}
	return fmt.Sprintf("`%s` %s", short, message)
}

// structuredFields renders the R8 structured fields shared by all messages so
// an AI-agent consumer can act on them. Free-text values that can carry
// user/contributor-controlled content (labels, commit message, actor) are
// mrkdwn-escaped; IDs and statuses are system-generated.
func structuredFields(deploymentID, projectLabel, envLabel, status, commitSha, commitMessage, trigger, triggeredBy string) slack.Block {
	return slack.NewSectionBlock(
		slack.NewMarkdownField(fmt.Sprintf("*Project:*\n%s", escapeMrkdwn(projectLabel))),
		slack.NewMarkdownField(fmt.Sprintf("*Environment:*\n%s", escapeMrkdwn(envLabel))),
		slack.NewMarkdownField(fmt.Sprintf("*Status:*\n%s", status)),
		slack.NewMarkdownField(fmt.Sprintf("*Deployment:*\n`%s`", deploymentID)),
		slack.NewMarkdownField(fmt.Sprintf("*Commit:*\n%s", commitField(commitSha, escapeMrkdwn(commitMessage)))),
		slack.NewMarkdownField(fmt.Sprintf("*Triggered by:*\n%s (%s)", escapeMrkdwn(triggeredBy), trigger)),
	)
}

// linkBlock renders a single markdown link line as a section.
func linkBlock(url, label string) slack.Block {
	return slack.NewSectionBlock(slack.NewMarkdownField(fmt.Sprintf("<%s|%s>", url, label)))
}

// outcomeBlocks builds the message for a deployment outcome (ready/failed/…).
// The link is state-appropriate: the live URL on ready, the logs URL on failed.
func outcomeBlocks(deploymentID string, cfg *hydrav1.SlackStatusInitRequest, state hydrav1.SlackDeploymentState) []slack.Block {
	blocks := []slack.Block{
		slack.NewHeaderBlock(outcomeHeader(state)),
		structuredFields(
			deploymentID,
			cfg.GetProjectId(),
			cfg.GetEnvironmentLabel(),
			outcomeStatusText(state),
			cfg.GetCommitSha(),
			cfg.GetCommitMessage(),
			cfg.GetTrigger(),
			cfg.GetTriggeredBy(),
		),
	}

	switch state {
	case hydrav1.SlackDeploymentState_SLACK_DEPLOYMENT_STATE_READY:
		if cfg.GetEnvironmentUrl() != "" {
			blocks = append(blocks, linkBlock(cfg.GetEnvironmentUrl(), "Open deployment"))
		}
	case hydrav1.SlackDeploymentState_SLACK_DEPLOYMENT_STATE_FAILED:
		if cfg.GetLogUrl() != "" {
			blocks = append(blocks, linkBlock(cfg.GetLogUrl(), "View logs"))
		}
	case hydrav1.SlackDeploymentState_SLACK_DEPLOYMENT_STATE_IN_PROGRESS,
		hydrav1.SlackDeploymentState_SLACK_DEPLOYMENT_STATE_UNSPECIFIED:
		if cfg.GetLogUrl() != "" {
			blocks = append(blocks, linkBlock(cfg.GetLogUrl(), "View deployment"))
		}
	}

	return blocks
}

// outcomeStatusText is the human-readable status shown in the fields.
func outcomeStatusText(state hydrav1.SlackDeploymentState) string {
	switch state {
	case hydrav1.SlackDeploymentState_SLACK_DEPLOYMENT_STATE_READY:
		return "ready"
	case hydrav1.SlackDeploymentState_SLACK_DEPLOYMENT_STATE_FAILED:
		return "failed"
	case hydrav1.SlackDeploymentState_SLACK_DEPLOYMENT_STATE_IN_PROGRESS,
		hydrav1.SlackDeploymentState_SLACK_DEPLOYMENT_STATE_UNSPECIFIED:
		return "in progress"
	default:
		return "in progress"
	}
}

// approvalBlocks builds the interactive approval prompt for a gated deployment.
// The actions block_id carries the deployment and workspace as lookup keys for
// the interactivity handler.
func approvalBlocks(deploymentID string, req *hydrav1.SlackPostApprovalRequest) []slack.Block {
	blockID := fmt.Sprintf("%s:%s:%s", approvalBlockIDPrefix, deploymentID, req.GetWorkspaceId())

	blocks := []slack.Block{
		slack.NewHeaderBlock("⏸ Deployment awaiting approval"),
		structuredFields(
			deploymentID,
			req.GetProjectId(),
			req.GetEnvironmentLabel(),
			"awaiting approval",
			req.GetCommitSha(),
			req.GetCommitMessage(),
			req.GetTrigger(),
			req.GetTriggeredBy(),
		),
	}
	if req.GetReviewUrl() != "" {
		blocks = append(blocks, linkBlock(req.GetReviewUrl(), "Review in dashboard"))
	}
	blocks = append(blocks, slack.NewActionsBlock(
		blockID,
		slack.NewButton("Approve", "approve", deploymentID, "primary"),
		slack.NewButton("Reject", "reject", deploymentID, "danger"),
	))

	return blocks
}
