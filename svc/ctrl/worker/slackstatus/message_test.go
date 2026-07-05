package slackstatus

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	hydrav1 "github.com/unkeyed/unkey/gen/proto/hydra/v1"
	"github.com/unkeyed/unkey/svc/ctrl/internal/slack"
)

// allText flattens every field and text string across a set of blocks, so tests
// can assert a link or value appears somewhere in the rendered message.
func allText(blocks []slack.Block) string {
	var b strings.Builder
	for _, blk := range blocks {
		if blk.Text != nil {
			b.WriteString(blk.Text.Text)
			b.WriteString("\n")
		}
		for _, f := range blk.Fields {
			b.WriteString(f.Text)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// TestApprovalBlocks_BlockIDContract locks the block_id + action_id format the
// dashboard interactivity handler (web .../api/webhooks/slack) parses. Changing
// it here without updating the handler breaks approve/reject routing.
func TestApprovalBlocks_BlockIDContract(t *testing.T) {
	req := &hydrav1.SlackPostApprovalRequest{
		WorkspaceId:      "ws_123",
		ProjectId:        "proj_456",
		EnvironmentLabel: "api - preview",
		ReviewUrl:        "https://app.unkey.com/acme/projects/proj_456/deployments/dep_789",
		IsProduction:     false,
		CommitSha:        "a1b2c3d4e5f6",
		CommitMessage:    "fix auth",
		Trigger:          "github",
		TriggeredBy:      "octocat",
	}

	blocks := approvalBlocks("dep_789", req)

	var actions *slack.Block
	for i := range blocks {
		if blocks[i].Type == "actions" {
			actions = &blocks[i]
		}
	}
	require.NotNil(t, actions, "approval message must contain an actions block")

	// block_id contract: "<prefix>:<deploymentID>:<workspaceID>".
	parts := strings.Split(actions.BlockID, ":")
	require.Len(t, parts, 3)
	require.Equal(t, approvalBlockIDPrefix, parts[0])
	require.Equal(t, "dep_789", parts[1])
	require.Equal(t, "ws_123", parts[2])

	// Two buttons: approve (primary) and reject (danger).
	require.Len(t, actions.Elements, 2)
	require.Equal(t, "approve", actions.Elements[0].ActionID)
	require.Equal(t, "primary", actions.Elements[0].Style)
	require.Equal(t, "reject", actions.Elements[1].ActionID)
	require.Equal(t, "danger", actions.Elements[1].Style)

	// The review link is present.
	require.Contains(t, allText(blocks), req.GetReviewUrl())
}

// TestApprovalBlocks_EscapesUntrustedMrkdwn proves a fork contributor cannot
// inject Slack control sequences (channel pings, fake links) into the approval
// prompt via the commit message or sender login.
func TestApprovalBlocks_EscapesUntrustedMrkdwn(t *testing.T) {
	req := &hydrav1.SlackPostApprovalRequest{
		WorkspaceId:      "ws_123",
		ProjectId:        "proj_456",
		EnvironmentLabel: "preview",
		ReviewUrl:        "https://app.unkey.com/review",
		IsProduction:     false,
		CommitSha:        "a1b2c3d4",
		CommitMessage:    "pwn <!channel> see <https://evil.example|Review in dashboard>",
		Trigger:          "github",
		TriggeredBy:      "<https://evil.example|attacker>",
	}

	rendered := allText(approvalBlocks("dep_789", req))
	require.NotContains(t, rendered, "<!channel>")
	require.NotContains(t, rendered, "<https://evil.example")
	require.Contains(t, rendered, "&lt;!channel&gt;")
	// The legitimate review link is still a real mrkdwn link.
	require.Contains(t, rendered, "<https://app.unkey.com/review|")
}

// TestResolvedApprovalBlocks_RemovesButtonsAndShowsOutcome verifies retiring an
// approval prompt strips the interactive controls and renders the decision.
func TestResolvedApprovalBlocks_RemovesButtonsAndShowsOutcome(t *testing.T) {
	req := &hydrav1.SlackPostApprovalRequest{
		WorkspaceId:      "ws_123",
		ProjectId:        "proj_456",
		EnvironmentLabel: "preview",
		ReviewUrl:        "https://app.unkey.com/review",
		IsProduction:     false,
		CommitSha:        "a1b2c3d4",
		CommitMessage:    "fix auth",
		Trigger:          "github",
		TriggeredBy:      "octocat",
	}

	approvedBlocks := resolvedApprovalBlocks("dep_789", req, true, false, "James <admin>")
	for _, blk := range approvedBlocks {
		require.NotEqual(t, "actions", blk.Type, "resolved prompt must not carry buttons")
	}
	rendered := allText(approvedBlocks)
	require.Contains(t, rendered, "approved")
	// resolvedBy is untrusted display text and must be escaped.
	require.Contains(t, rendered, "James &lt;admin&gt;")

	rejected := allText(resolvedApprovalBlocks("dep_789", req, false, false, ""))
	require.Contains(t, rejected, "rejected")
	require.NotContains(t, rejected, "Resolved by")

	// Superseded overrides approved/rejected and carries no resolver attribution,
	// even when a resolvedBy is passed.
	superseded := allText(resolvedApprovalBlocks("dep_789", req, true, true, "James"))
	require.Contains(t, superseded, "superseded")
	require.NotContains(t, superseded, "approved")
	require.NotContains(t, superseded, "Resolved by")
}

// TestOutcomeBlocks_LinkPerState verifies the ready message links to the live
// URL and the failed message links to the logs URL (AE2).
func TestOutcomeBlocks_LinkPerState(t *testing.T) {
	cfg := &hydrav1.SlackStatusInitRequest{
		WorkspaceId:      "ws_1",
		ProjectId:        "proj_1",
		EnvironmentLabel: "api - production",
		EnvironmentUrl:   "https://api-acme.unkey.app",
		LogUrl:           "https://app.unkey.com/acme/projects/proj_1/deployments/dep_1",
		IsProduction:     true,
		CommitSha:        "abcdef1",
		CommitMessage:    "ship it",
		Trigger:          "github",
		TriggeredBy:      "octocat",
	}

	ready := allText(outcomeBlocks("dep_1", cfg, hydrav1.SlackDeploymentState_SLACK_DEPLOYMENT_STATE_READY))
	require.Contains(t, ready, cfg.GetEnvironmentUrl())

	failed := allText(outcomeBlocks("dep_1", cfg, hydrav1.SlackDeploymentState_SLACK_DEPLOYMENT_STATE_FAILED))
	require.Contains(t, failed, cfg.GetLogUrl())
}
