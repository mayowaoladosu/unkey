package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Payload represents a Slack webhook message payload.
type Payload struct {
	Text   string  `json:"text"`
	Blocks []Block `json:"blocks"`
}

// Block represents a Slack block element.
type Block struct {
	Type     string    `json:"type"`
	Text     *Text     `json:"text,omitempty"`
	Fields   []Field   `json:"fields,omitempty"`
	BlockID  string    `json:"block_id,omitempty"`
	Elements []Element `json:"elements,omitempty"`
}

// Element represents an interactive block element, such as a button inside an
// actions block.
type Element struct {
	Type     string `json:"type"`
	Text     *Text  `json:"text,omitempty"`
	ActionID string `json:"action_id,omitempty"`
	Value    string `json:"value,omitempty"`
	// Style is one of "primary", "danger", or "" (default).
	Style string `json:"style,omitempty"`
}

// Text represents a Slack text element.
type Text struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Emoji bool   `json:"emoji,omitempty"`
}

// Field represents a Slack section field.
type Field struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// NewHeaderBlock creates a header block with plain text.
func NewHeaderBlock(text string) Block {
	return Block{
		Type: "header",
		Text: &Text{
			Type:  "plain_text",
			Text:  text,
			Emoji: true,
		},
		Fields:   nil,
		BlockID:  "",
		Elements: nil,
	}
}

// NewSectionBlock creates a section block with markdown fields.
func NewSectionBlock(fields ...Field) Block {
	return Block{
		Type:     "section",
		Text:     nil,
		Fields:   fields,
		BlockID:  "",
		Elements: nil,
	}
}

// NewMarkdownField creates a markdown field for use in section blocks.
func NewMarkdownField(text string) Field {
	return Field{
		Type: "mrkdwn",
		Text: text,
	}
}

// NewActionsBlock creates an actions block holding interactive elements (e.g.
// buttons). blockID is echoed back in the interaction payload, so callers use
// it to route a click (e.g. to a specific deployment).
func NewActionsBlock(blockID string, elements ...Element) Block {
	return Block{
		Type:     "actions",
		Text:     nil,
		Fields:   nil,
		BlockID:  blockID,
		Elements: elements,
	}
}

// NewButton creates a button element. style is one of "primary", "danger", or
// "" for the default. actionID identifies which control was clicked.
func NewButton(text, actionID, value, style string) Element {
	return Element{
		Type: "button",
		Text: &Text{
			Type:  "plain_text",
			Text:  text,
			Emoji: true,
		},
		ActionID: actionID,
		Value:    value,
		Style:    style,
	}
}

// Client sends messages to Slack webhooks.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new Slack webhook client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Send posts a payload to the given webhook URL.
func (c *Client) Send(ctx context.Context, webhookURL string, payload Payload) (err error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close response body: %w", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned status %d", resp.StatusCode)
	}

	return nil
}
