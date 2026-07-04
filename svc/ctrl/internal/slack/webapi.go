package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// webAPIBaseURL is the Slack Web API root. chat.postMessage and chat.update are
// appended to it.
const webAPIBaseURL = "https://slack.com/api"

// WebClient calls the Slack Web API using a bot token. Unlike the incoming
// webhook Client, it can post interactive messages (buttons) and update them in
// place, and it returns the posted message's channel and timestamp so a later
// update can target the same message.
//
// The caller supplies the decrypted bot token per call; this client never
// touches vault. It is safe for concurrent use.
type WebClient struct {
	httpClient *http.Client
	// baseURL is the Slack Web API root; overridable in tests.
	baseURL string
}

// NewWebClient creates a Slack Web API client.
func NewWebClient() *WebClient {
	return &WebClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    webAPIBaseURL,
	}
}

// Message is the body posted to chat.postMessage / chat.update.
type Message struct {
	Channel string  `json:"channel"`
	Text    string  `json:"text,omitempty"`
	Blocks  []Block `json:"blocks,omitempty"`
	// TS is set only for chat.update, identifying the message to edit.
	TS string `json:"ts,omitempty"`
}

// PostResult identifies a posted message so it can be updated later.
type PostResult struct {
	Channel string
	TS      string
}

// slackAPIResponse is the common envelope returned by the Web API.
type slackAPIResponse struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error"`
	Channel string `json:"channel"`
	TS      string `json:"ts"`
}

// PostMessage posts a message to a channel and returns its channel and ts.
func (c *WebClient) PostMessage(ctx context.Context, botToken string, msg Message) (PostResult, error) {
	resp, err := c.call(ctx, botToken, "chat.postMessage", msg)
	if err != nil {
		return PostResult{}, err
	}
	return PostResult{Channel: resp.Channel, TS: resp.TS}, nil
}

// UpdateMessage edits an existing message identified by msg.Channel and msg.TS.
func (c *WebClient) UpdateMessage(ctx context.Context, botToken string, msg Message) error {
	if msg.TS == "" {
		return fmt.Errorf("update message: ts is required")
	}
	_, err := c.call(ctx, botToken, "chat.update", msg)
	return err
}

func (c *WebClient) call(ctx context.Context, botToken, method string, msg Message) (slackAPIResponse, error) {
	body, err := json.Marshal(msg)
	if err != nil {
		return slackAPIResponse{}, fmt.Errorf("marshal %s: %w", method, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+method, bytes.NewBuffer(body))
	if err != nil {
		return slackAPIResponse{}, fmt.Errorf("create %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+botToken)

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return slackAPIResponse{}, fmt.Errorf("send %s request: %w", method, err)
	}
	defer func() {
		_ = httpResp.Body.Close()
	}()

	if httpResp.StatusCode != http.StatusOK {
		return slackAPIResponse{}, fmt.Errorf("slack %s returned status %d", method, httpResp.StatusCode)
	}

	var parsed slackAPIResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&parsed); err != nil {
		return slackAPIResponse{}, fmt.Errorf("decode %s response: %w", method, err)
	}
	// The Web API always returns HTTP 200; application errors are carried in the
	// `ok`/`error` fields.
	if !parsed.OK {
		return slackAPIResponse{}, fmt.Errorf("slack %s failed: %s", method, parsed.Error)
	}

	return parsed, nil
}
