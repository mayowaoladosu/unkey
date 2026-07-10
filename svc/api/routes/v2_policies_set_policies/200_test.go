package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	frontlinev1 "github.com/unkeyed/unkey/gen/proto/frontline/v1"
	"github.com/unkeyed/unkey/svc/api/internal/testutil"
	"github.com/unkeyed/unkey/svc/api/internal/testutil/seed"
	"github.com/unkeyed/unkey/svc/api/openapi"
	handler "github.com/unkeyed/unkey/svc/api/routes/v2_policies_set_policies"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestSetPoliciesSuccessfully(t *testing.T) {
	h := testutil.NewHarness(t)

	route := &handler.Handler{DB: h.DB, Auditlogs: h.Auditlogs}
	h.Register(route)

	ctx := context.Background()
	workspace := h.Resources().UserWorkspace
	rootKey := h.CreateRootKey(workspace.ID, "environment.*.set_policies")
	headers := authHeaders(rootKey)

	call := func(t *testing.T, req handler.Request) {
		t.Helper()
		res := testutil.CallRoute[handler.Request, handler.Response](h, route, headers, req)
		require.Equal(t, 200, res.Status, "expected 200, received: %s", res.RawBody)
		require.NotEmpty(t, res.Body.Meta.RequestId)
	}

	t.Run("batch of all four variants stores dashboard-compatible wire JSON", func(t *testing.T) {
		env := seedEnvironment(t, h)
		api := h.CreateApi(seed.CreateApiRequest{WorkspaceID: workspace.ID})

		call(t, makeRequest(env, []openapi.Policy{
			{
				Name:    "keyauth",
				Enabled: true,
				Keyauth: &openapi.KeyauthPolicy{Keyspaces: []string{api.KeyAuthID.String}},
			},
			{
				Name:    "ratelimit",
				Enabled: true,
				Ratelimit: &openapi.RatelimitPolicy{
					Limit:      100,
					WindowMs:   60000,
					Identifier: openapi.RatelimitIdentifier{RemoteIp: &map[string]any{}},
				},
			},
			firewallPolicy("KEBAP", false),
			{
				Name:    "openapi",
				Enabled: true,
				Openapi: &openapi.OpenapiPolicy{},
			},
		}))

		stored := readStoredPolicies(t, h, env)
		require.Len(t, stored, 4)

		// The gateway must be able to parse the stored blob the way its
		// ParseMiddleware does.
		require.NoError(t, protojson.Unmarshal([]byte(readStoredBlob(t, h, env)), &frontlinev1.Config{}))

		// The dashboard reads the blob through a strict schema: enabled must be
		// present even when false, ids must exist, and no type field may appear.
		names := make([]string, 0, len(stored))
		byName := make(map[string]map[string]json.RawMessage)
		for _, raw := range stored {
			var keys map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(raw, &keys))
			require.NotContains(t, keys, "type")
			require.Contains(t, keys, "enabled")

			var id, name string
			require.NoError(t, json.Unmarshal(keys["id"], &id))
			require.NoError(t, json.Unmarshal(keys["name"], &name))
			require.NotEmpty(t, id)
			names = append(names, name)
			byName[name] = keys
		}

		require.Equal(t, []string{"keyauth", "ratelimit", "KEBAP", "openapi"}, names,
			"stored order must be the request order")
		require.JSONEq(t, `false`, string(byName["KEBAP"]["enabled"]))
		require.JSONEq(t, `{"action":"ACTION_DENY"}`, string(byName["KEBAP"]["firewall"]))
		require.JSONEq(t,
			fmt.Sprintf(`{"keySpaceIds":["%s"]}`, api.KeyAuthID.String),
			string(byName["keyauth"]["keyauth"]),
		)
		require.JSONEq(t, `{}`, string(byName["openapi"]["openapi"]))
		require.JSONEq(
			t,
			`{"limit":100,"windowMs":60000,"identifier":{"remoteIp":{}}}`,
			string(byName["ratelimit"]["ratelimit"]),
		)

		logs := h.FindAuditLogsByTargetID(ctx, t, env.environmentID)
		require.Len(t, logs, 4)
		for _, l := range logs {
			require.Contains(t, l.Description, "Set policy")
		}
	})

	t.Run("set replaces stored policies including variants this API cannot create", func(t *testing.T) {
		env := seedEnvironment(t, h)
		jwtauth := `{"id":"pol_jwt","name":"legacy jwt","enabled":true,"jwtauth":{}}`
		seedSentinelConfig(t, h, env, fmt.Sprintf(`{"policies":[%s]}`, jwtauth))

		call(t, makeRequest(env, []openapi.Policy{firewallPolicy("deny", true)}))

		stored := readStoredPolicies(t, h, env)
		require.Len(t, stored, 1)
		require.Contains(t, string(stored[0]), `"name":"deny"`)
		require.NotContains(t, readStoredBlob(t, h, env), "pol_jwt")
	})

	t.Run("second set replaces the first and regenerates ids", func(t *testing.T) {
		env := seedEnvironment(t, h)
		call(t, makeRequest(env, []openapi.Policy{
			firewallPolicy("first", true),
			firewallPolicy("second", true),
		}))
		before := storedPolicyIDs(t, h, env)
		require.Len(t, before, 2)

		call(t, makeRequest(env, []openapi.Policy{firewallPolicy("only", false)}))

		after := storedPolicyIDs(t, h, env)
		require.Len(t, after, 1)
		require.NotContains(t, before, after[0], "every set generates fresh ids")
		stored := readStoredPolicies(t, h, env)
		require.Contains(t, string(stored[0]), `"name":"only"`)
	})

	t.Run("empty list removes all policies", func(t *testing.T) {
		env := seedEnvironment(t, h)
		call(t, makeRequest(env, []openapi.Policy{firewallPolicy("doomed", true)}))

		call(t, makeRequest(env, []openapi.Policy{}))

		stored := readStoredPolicies(t, h, env)
		require.Empty(t, stored)

		logs := h.FindAuditLogsByTargetID(ctx, t, env.environmentID)
		var removed bool
		for _, l := range logs {
			if strings.Contains(l.Description, "Removed all policies") {
				removed = true
			}
		}
		require.True(t, removed, "clearing must leave a dedicated audit entry")
	})

	t.Run("concurrent sets end with exactly one intact list", func(t *testing.T) {
		env := seedEnvironment(t, h)

		const workers = 5
		var wg sync.WaitGroup
		statuses := make([]int, workers)
		for i := range workers {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				res := testutil.CallRoute[handler.Request, handler.Response](h, route, headers,
					makeRequest(env, []openapi.Policy{firewallPolicy(fmt.Sprintf("policy-%d", i), true)}))
				statuses[i] = res.Status
			}(i)
		}
		wg.Wait()

		for i, status := range statuses {
			require.Equal(t, 200, status, "worker %d", i)
		}

		// Smoke: concurrent sets all succeed and last writer wins.
		stored := readStoredPolicies(t, h, env)
		require.Len(t, stored, 1)
		require.Regexp(t, `"name":"policy-\d"`, string(stored[0]))
	})
}
