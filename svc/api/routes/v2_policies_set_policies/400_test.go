package handler_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/ptr"
	"github.com/unkeyed/unkey/svc/api/internal/testutil"
	"github.com/unkeyed/unkey/svc/api/internal/testutil/seed"
	"github.com/unkeyed/unkey/svc/api/openapi"
	handler "github.com/unkeyed/unkey/svc/api/routes/v2_policies_set_policies"
)

func TestSetPoliciesBadRequest(t *testing.T) {
	h := testutil.NewHarness(t)

	route := &handler.Handler{DB: h.DB, Auditlogs: h.Auditlogs}
	h.Register(route)

	workspace := h.Resources().UserWorkspace
	env := seedEnvironment(t, h)
	api := h.CreateApi(seed.CreateApiRequest{WorkspaceID: workspace.ID})
	rootKey := h.CreateRootKey(workspace.ID, "environment.*.set_policies")
	headers := authHeaders(rootKey)

	callTyped := func(t *testing.T, policies []openapi.Policy) testutil.TestResponse[openapi.BadRequestErrorResponse] {
		t.Helper()
		return testutil.CallRoute[handler.Request, openapi.BadRequestErrorResponse](h, route, headers, makeRequest(env, policies))
	}

	t.Run("no variant set", func(t *testing.T) {
		res := callTyped(t, []openapi.Policy{{Name: "empty", Enabled: true}})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
		require.Contains(t, res.Body.Error.Detail, "exactly one of")
	})

	t.Run("two variants set", func(t *testing.T) {
		res := callTyped(t, []openapi.Policy{{
			Name:     "double",
			Enabled:  true,
			Firewall: &openapi.FirewallPolicy{Action: "ACTION_DENY"},
			Openapi:  &openapi.OpenapiPolicy{},
		}})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
	})

	t.Run("more than 50 policies in request", func(t *testing.T) {
		policies := make([]openapi.Policy, 51)
		for i := range policies {
			policies[i] = firewallPolicy(fmt.Sprintf("p%d", i), true)
		}
		res := callTyped(t, policies)
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
	})

	t.Run("more than 10 match expressions", func(t *testing.T) {
		match := make([]openapi.MatchExpr, 11)
		for i := range match {
			match[i] = pathMatch(openapi.StringMatch{Prefix: ptr.P(fmt.Sprintf("/p%d", i))})
		}
		p := firewallPolicy("too many matches", true)
		p.Match = &match
		res := callTyped(t, []openapi.Policy{p})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
	})

	t.Run("empty keyspaces", func(t *testing.T) {
		res := callTyped(t, []openapi.Policy{{
			Name:    "k",
			Enabled: true,
			Keyauth: &openapi.KeyauthPolicy{Keyspaces: []string{}},
		}})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
	})

	t.Run("more than 5 keyspaces", func(t *testing.T) {
		ids := make([]string, 6)
		for i := range ids {
			ids[i] = api.KeyAuthID.String
		}
		res := callTyped(t, []openapi.Policy{{
			Name:    "k",
			Enabled: true,
			Keyauth: &openapi.KeyauthPolicy{Keyspaces: ids},
		}})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
	})

	t.Run("permissionQuery over 1000 chars", func(t *testing.T) {
		res := callTyped(t, []openapi.Policy{{
			Name:    "k",
			Enabled: true,
			Keyauth: &openapi.KeyauthPolicy{
				Keyspaces:       []string{api.KeyAuthID.String},
				PermissionQuery: ptr.P(strings.Repeat("a", 1001)),
			},
		}})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
	})

	t.Run("keyauth ratelimit limit without duration", func(t *testing.T) {
		res := callTyped(t, []openapi.Policy{{
			Name:    "k",
			Enabled: true,
			Keyauth: &openapi.KeyauthPolicy{
				Keyspaces:  []string{api.KeyAuthID.String},
				Ratelimits: &[]openapi.KeyRatelimit{{Name: "requests", Limit: ptr.P(int64(10))}},
			},
		}})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
		require.Contains(t, res.Body.Error.Detail, "limit and duration together")
	})

	t.Run("invalid regex is rejected at write time", func(t *testing.T) {
		p := firewallPolicy("bad regex", true)
		p.Match = &[]openapi.MatchExpr{pathMatch(openapi.StringMatch{Regex: ptr.P("[unclosed")})}
		res := callTyped(t, []openapi.Policy{p})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
		require.Contains(t, res.Body.Error.Detail, "not a valid regular expression")
	})

	t.Run("string match with two modes", func(t *testing.T) {
		p := firewallPolicy("m", true)
		p.Match = &[]openapi.MatchExpr{pathMatch(openapi.StringMatch{Exact: ptr.P("/a"), Prefix: ptr.P("/b")})}
		res := callTyped(t, []openapi.Policy{p})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
	})

	t.Run("header match with neither present nor value", func(t *testing.T) {
		p := firewallPolicy("m", true)
		p.Match = &[]openapi.MatchExpr{{Header: &struct {
			Name    string                          `json:"name"`
			Present *openapi.MatchExprHeaderPresent `json:"present,omitempty"`
			Value   *openapi.StringMatch            `json:"value,omitempty"`
		}{Name: "x-kebap"}}}
		res := callTyped(t, []openapi.Policy{p})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
	})

	t.Run("ratelimit identifier with two variants", func(t *testing.T) {
		res := callTyped(t, []openapi.Policy{{
			Name:    "r",
			Enabled: true,
			Ratelimit: &openapi.RatelimitPolicy{
				Limit:    10,
				WindowMs: 1000,
				Identifier: openapi.RatelimitIdentifier{
					RemoteIp: &map[string]any{},
					Path:     &map[string]any{},
				},
			},
		}})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
	})

	// Raw payloads exercise schema-level rejections the typed request cannot
	// express: unknown fields must never reach the stored blob.
	rawPolicy := func(t *testing.T, policy map[string]any) testutil.TestResponse[openapi.BadRequestErrorResponse] {
		t.Helper()
		return testutil.CallRoute[map[string]any, openapi.BadRequestErrorResponse](h, route, headers, map[string]any{
			"project":     env.projectID,
			"app":         env.appID,
			"environment": env.environmentID,
			"policies":    []map[string]any{policy},
		})
	}

	t.Run("jwtauth variant is rejected by the schema", func(t *testing.T) {
		res := rawPolicy(t, map[string]any{"name": "jwt", "enabled": true, "jwtauth": map[string]any{}})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
	})

	t.Run("client-supplied id is rejected by the schema", func(t *testing.T) {
		res := rawPolicy(t, map[string]any{
			"name": "with id", "enabled": true, "id": "pol_client",
			"firewall": map[string]any{"action": "ACTION_DENY"},
		})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
	})

	t.Run("unknown firewall action is rejected by the schema", func(t *testing.T) {
		res := rawPolicy(t, map[string]any{
			"name": "allow", "enabled": true,
			"firewall": map[string]any{"action": "ACTION_ALLOW"},
		})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
	})
}
