package slack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// newTestClient returns a WebClient pointed at the given test server.
func newTestClient(t *testing.T, srv *httptest.Server) *WebClient {
	t.Helper()
	c := NewWebClient()
	c.baseURL = srv.URL
	return c
}

func TestPostMessage_SendsBearerAndReturnsChannelTS(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody Message

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(raw, &gotBody))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1700000000.000100"}`))
	}))
	t.Cleanup(srv.Close)

	res, err := newTestClient(t, srv).PostMessage(context.Background(), "xoxb-test-token", Message{
		Channel: "C123",
		Text:    "hello",
	})
	require.NoError(t, err)
	require.Equal(t, "Bearer xoxb-test-token", gotAuth)
	require.Equal(t, "/chat.postMessage", gotPath)
	require.Equal(t, "C123", gotBody.Channel)
	require.Equal(t, "C123", res.Channel)
	require.Equal(t, "1700000000.000100", res.TS)
}

func TestPostMessage_NonOKResponseIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":false,"error":"channel_not_found"}`))
	}))
	t.Cleanup(srv.Close)

	_, err := newTestClient(t, srv).PostMessage(context.Background(), "xoxb", Message{Channel: "C404"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "channel_not_found")
}

func TestUpdateMessage_TargetsChannelAndTS(t *testing.T) {
	var gotBody Message
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(raw, &gotBody))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"channel":"C123","ts":"1700000000.000100"}`))
	}))
	t.Cleanup(srv.Close)

	err := newTestClient(t, srv).UpdateMessage(context.Background(), "xoxb", Message{
		Channel: "C123",
		TS:      "1700000000.000100",
		Text:    "resolved",
	})
	require.NoError(t, err)
	require.Equal(t, "/chat.update", gotPath)
	require.Equal(t, "C123", gotBody.Channel)
	require.Equal(t, "1700000000.000100", gotBody.TS)
}

func TestUpdateMessage_RequiresTS(t *testing.T) {
	err := NewWebClient().UpdateMessage(context.Background(), "xoxb", Message{Channel: "C123"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "ts is required")
}

func TestActionsBlock_SerializesTwoButtons(t *testing.T) {
	block := NewActionsBlock(
		"deploy:dep_123:ws_456",
		NewButton("Approve", "approve", "dep_123", "primary"),
		NewButton("Reject", "reject", "dep_123", "danger"),
	)

	raw, err := json.Marshal(block)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Equal(t, "actions", decoded["type"])
	require.Equal(t, "deploy:dep_123:ws_456", decoded["block_id"])

	elements, ok := decoded["elements"].([]any)
	require.True(t, ok)
	require.Len(t, elements, 2)

	first := elements[0].(map[string]any)
	require.Equal(t, "button", first["type"])
	require.Equal(t, "approve", first["action_id"])
	require.Equal(t, "primary", first["style"])

	second := elements[1].(map[string]any)
	require.Equal(t, "reject", second["action_id"])
	require.Equal(t, "danger", second["style"])
}
