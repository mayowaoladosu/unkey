package handler_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/svc/api/internal/testutil"
	"github.com/unkeyed/unkey/svc/api/internal/testutil/seed"
	"github.com/unkeyed/unkey/svc/api/openapi"
	handler "github.com/unkeyed/unkey/svc/api/routes/v2_policies_create_policy"
)

func TestCreatePolicyBadRequest(t *testing.T) {
	h := testutil.NewHarness(t)

	route := &handler.Handler{DB: h.DB, Auditlogs: h.Auditlogs}
	h.Register(route)

	workspace := h.Resources().UserWorkspace
	env := seedEnvironment(t, h)
	api := h.CreateApi(seed.CreateApiRequest{WorkspaceID: workspace.ID})
	rootKey := h.CreateRootKey(workspace.ID, "environment.*.create_policy")
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

	t.Run("empty policies array", func(t *testing.T) {
		res := callTyped(t, []openapi.Policy{})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
	})

	t.Run("more than 10 policies in request", func(t *testing.T) {
		policies := make([]openapi.Policy, 11)
		for i := range policies {
			policies[i] = firewallPolicy(fmt.Sprintf("p%d", i), true)
		}
		res := callTyped(t, policies)
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
	})

	t.Run("merged count exceeding the environment cap", func(t *testing.T) {
		capEnv := seedEnvironment(t, h)
		nine := make([]string, 9)
		for i := range nine {
			nine[i] = fmt.Sprintf(`{"id":"pol_seed%d","name":"seed%d","enabled":true,"firewall":{"action":"ACTION_DENY"}}`, i, i)
		}
		seedSentinelConfig(t, h, capEnv, fmt.Sprintf(`{"policies":[%s]}`, strings.Join(nine, ",")))

		res := testutil.CallRoute[handler.Request, openapi.BadRequestErrorResponse](h, route, headers,
			makeRequest(capEnv, []openapi.Policy{firewallPolicy("a", true), firewallPolicy("b", true)}))
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
		require.Contains(t, res.Body.Error.Detail, "at most 10")

		stored := readStoredPolicies(t, h, capEnv)
		require.Len(t, stored, 9, "nothing may be written when the cap check fails")
	})

	t.Run("more than 10 match expressions", func(t *testing.T) {
		match := make([]openapi.MatchExpr, 11)
		for i := range match {
			match[i] = openapi.MatchExpr{Path: &struct {
				Path openapi.StringMatch `json:"path"`
			}{Path: openapi.StringMatch{Prefix: ptr(fmt.Sprintf("/p%d", i))}}}
		}
		p := firewallPolicy("too many matches", true)
		p.Match = &match
		res := callTyped(t, []openapi.Policy{p})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
	})

	t.Run("empty keySpaceIds", func(t *testing.T) {
		res := callTyped(t, []openapi.Policy{{
			Name:    "k",
			Enabled: true,
			Keyauth: &openapi.KeyauthPolicy{KeySpaceIds: []string{}},
		}})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
	})

	t.Run("more than 5 keySpaceIds", func(t *testing.T) {
		ids := make([]string, 6)
		for i := range ids {
			ids[i] = api.KeyAuthID.String
		}
		res := callTyped(t, []openapi.Policy{{
			Name:    "k",
			Enabled: true,
			Keyauth: &openapi.KeyauthPolicy{KeySpaceIds: ids},
		}})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
	})

	t.Run("permissionQuery over 1000 chars", func(t *testing.T) {
		res := callTyped(t, []openapi.Policy{{
			Name:    "k",
			Enabled: true,
			Keyauth: &openapi.KeyauthPolicy{
				KeySpaceIds:     []string{api.KeyAuthID.String},
				PermissionQuery: ptr(strings.Repeat("a", 1001)),
			},
		}})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
	})

	t.Run("keyauth ratelimit limit without duration", func(t *testing.T) {
		res := callTyped(t, []openapi.Policy{{
			Name:    "k",
			Enabled: true,
			Keyauth: &openapi.KeyauthPolicy{
				KeySpaceIds: []string{api.KeyAuthID.String},
				Ratelimits:  &[]openapi.KeyRatelimit{{Name: "requests", Limit: ptr(int64(10))}},
			},
		}})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
		require.Contains(t, res.Body.Error.Detail, "limit and duration together")
	})

	t.Run("invalid regex is rejected at create time", func(t *testing.T) {
		p := firewallPolicy("bad regex", true)
		p.Match = &[]openapi.MatchExpr{{Path: &struct {
			Path openapi.StringMatch `json:"path"`
		}{Path: openapi.StringMatch{Regex: ptr("[unclosed")}}}}
		res := callTyped(t, []openapi.Policy{p})
		require.Equal(t, http.StatusBadRequest, res.Status, "received: %s", res.RawBody)
		require.Contains(t, res.Body.Error.Detail, "not a valid regular expression")
	})

	t.Run("string match with two modes", func(t *testing.T) {
		p := firewallPolicy("m", true)
		p.Match = &[]openapi.MatchExpr{{Path: &struct {
			Path openapi.StringMatch `json:"path"`
		}{Path: openapi.StringMatch{Exact: ptr("/a"), Prefix: ptr("/b")}}}}
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
					RemoteIp: &map[string]interface{}{},
					Path:     &map[string]interface{}{},
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
			"name": "with id", "enabled": true, "id": "pol_mine",
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
