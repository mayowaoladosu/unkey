package github

import (
	"context"
	"fmt"
	"net/url"
	"time"

	restate "github.com/restatedev/sdk-go"
	hydrav1 "github.com/unkeyed/unkey/gen/proto/hydra/v1"
	"github.com/unkeyed/unkey/pkg/logger"
	"github.com/unkeyed/unkey/pkg/webhook"
)

type pullRequestPayload struct {
	Action       string            `json:"action"`
	Number       int64             `json:"number"`
	PullRequest  pullRequestDetail `json:"pull_request"`
	Installation pushInstallation  `json:"installation"`
	Sender       pushSender        `json:"sender"`
}

type pullRequestDetail struct {
	Title string         `json:"title"`
	User  pushSender     `json:"user"`
	Head  pullRequestRef `json:"head"`
	Base  pullRequestRef `json:"base"`
}

type pullRequestRef struct {
	Ref  string         `json:"ref"`
	SHA  string         `json:"sha"`
	Repo pushRepository `json:"repo"`
}

// pullRequest handles pull_request events from forks. Same-repo PR deployments
// are created by push events, but their closed event still comes through here
// so the matching branch preview can be reclaimed.
func (h *handler) pullRequest(
	ctx context.Context,
	event webhook.Event,
	payload pullRequestPayload,
) error {
	// New commits create previews; closed ends their lifecycle.
	if payload.Action != "opened" && payload.Action != "synchronize" && payload.Action != "reopened" && payload.Action != "closed" {
		return fmt.Errorf("%w: pull_request action %s adds no commits", webhook.ErrIgnore, payload.Action)
	}

	isFork := payload.PullRequest.Head.Repo.ID != payload.PullRequest.Base.Repo.ID
	// Same-repo PRs are already handled by the push event; skip open/sync to
	// avoid a double deploy. Closed is retained for branch-preview cleanup.
	if !isFork && payload.Action != "reopened" && payload.Action != "closed" {
		return fmt.Errorf("%w: same-repo pull request, push event handles this", webhook.ErrIgnore)
	}

	pr := payload.PullRequest
	baseRepo := pr.Base.Repo

	objectKey := fmt.Sprintf("%d:%d", payload.Installation.ID, baseRepo.ID)
	client := hydrav1.NewGitHubWebhookServiceIngressClient(h.restate, objectKey)

	deliveryID := event.ID
	var sendOpts []restate.IngressSendOption
	if deliveryID != "" {
		sendOpts = append(sendOpts, restate.WithIdempotencyKey(deliveryID))
	}

	authorHandle := payload.Sender.Login
	authorAvatar := payload.Sender.AvatarURL
	if authorAvatar == "" {
		authorAvatar = fmt.Sprintf("https://github.com/%s.png", url.PathEscape(authorHandle))
	}
	forkRepositoryFullName := ""
	if isFork {
		forkRepositoryFullName = pr.Head.Repo.FullName
	}

	_, err := client.HandlePush().Send(ctx, &hydrav1.HandlePushRequest{
		InstallationId:         payload.Installation.ID,
		RepositoryId:           baseRepo.ID,
		RepositoryFullName:     baseRepo.FullName,
		Branch:                 pr.Head.Ref,
		After:                  pr.Head.SHA,
		CommitMessage:          pr.Title,
		CommitAuthorHandle:     authorHandle,
		CommitAuthorAvatarUrl:  authorAvatar,
		CommitTimestamp:        time.Now().UnixMilli(),
		DeliveryId:             deliveryID,
		SenderLogin:            payload.Sender.Login,
		IsForkPr:               isFork,
		PrNumber:               payload.Number,
		ForkRepositoryFullName: forkRepositoryFullName,
		PullRequestClosed:      payload.Action == "closed",
	}, sendOpts...)
	if err != nil {
		return fmt.Errorf("enqueue pull request for %s: %w", baseRepo.FullName, err)
	}

	logger.Info("GitHub pull request webhook enqueued to Restate",
		"delivery_id", deliveryID,
		"repository", baseRepo.FullName,
		"branch", pr.Head.Ref,
		"commit_sha", pr.Head.SHA,
		"pr_action", payload.Action,
	)
	return nil
}
