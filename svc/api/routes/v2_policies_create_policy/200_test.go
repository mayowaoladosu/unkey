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
	handler "github.com/unkeyed/unkey/svc/api/routes/v2_policies_create_policy"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestCreatePolicySuccessfully(t *testing.T) {
	h := testutil.NewHarness(t)

	route := &handler.Handler{DB: h.DB, Auditlogs: h.Auditlogs}
	h.Register(route)

	ctx := context.Background()
	workspace := h.Resources().UserWorkspace
	rootKey := h.CreateRootKey(workspace.ID, "environment.*.create_policy")
	headers := authHeaders(rootKey)

	call := func(t *testing.T, req handler.Request) handler.Response {
		t.Helper()
		res := testutil.CallRoute[handler.Request, handler.Response](h, route, headers, req)
		require.Equal(t, 200, res.Status, "expected 200, received: %s", res.RawBody)
		require.NotEmpty(t, res.Body.Meta.RequestId)
		return *res.Body
	}

	t.Run("batch of all four variants stores dashboard-compatible wire JSON", func(t *testing.T) {
		env := seedEnvironment(t, h)
		api := h.CreateApi(seed.CreateApiRequest{WorkspaceID: workspace.ID})

		call(t, makeRequest(env, []openapi.Policy{
			{
				Name:    "keyauth",
				Enabled: true,
				Keyauth: &openapi.KeyauthPolicy{KeySpaceIds: []string{api.KeyAuthID.String}},
			},
			{
				Name:    "ratelimit",
				Enabled: true,
				Ratelimit: &openapi.RatelimitPolicy{
					Limit:      100,
					WindowMs:   60000,
					Identifier: openapi.RatelimitIdentifier{RemoteIp: &map[string]interface{}{}},
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
		full, err := json.Marshal(map[string]any{"policies": stored})
		require.NoError(t, err)
		require.NoError(t, protojson.Unmarshal(full, &frontlinev1.Config{}))

		// The dashboard reads the blob through a strict schema: enabled must be
		// present even when false, ids must exist, and no type field may appear.
		type storedPolicy struct {
			keys map[string]json.RawMessage
			id   string
		}
		byName := make(map[string]storedPolicy)
		for _, raw := range stored {
			var keys map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(raw, &keys))
			require.NotContains(t, keys, "type")
			require.Contains(t, keys, "enabled")

			var id, name string
			require.NoError(t, json.Unmarshal(keys["id"], &id))
			require.NoError(t, json.Unmarshal(keys["name"], &name))
			require.True(t, strings.HasPrefix(id, "pol_"), "expected server-generated pol_ id, got %q", id)
			byName[name] = storedPolicy{keys: keys, id: id}
		}

		require.JSONEq(t, `false`, string(byName["KEBAP"].keys["enabled"]))
		require.JSONEq(t, `{"action":"ACTION_DENY"}`, string(byName["KEBAP"].keys["firewall"]))
		require.JSONEq(t, `{}`, string(byName["openapi"].keys["openapi"]))
		require.JSONEq(t,
			`{"limit":100,"windowMs":60000,"identifier":{"remoteIp":{}}}`,
			string(byName["ratelimit"].keys["ratelimit"]),
		)

		logs := h.FindAuditLogsByTargetID(ctx, t, env.environmentID)
		require.Len(t, logs, 4)
		for _, l := range logs {
			require.Contains(t, l.Description, "Created policy")
		}
	})

	t.Run("append preserves existing policies byte for byte including jwtauth", func(t *testing.T) {
		env := seedEnvironment(t, h)
		jwtauth := `{"id":"pol_jwt","name":"legacy jwt","enabled":true,"jwtauth":{}}`
		seedSentinelConfig(t, h, env, fmt.Sprintf(`{"policies":[%s]}`, jwtauth))

		call(t, makeRequest(env, []openapi.Policy{firewallPolicy("deny", true)}))

		stored := readStoredPolicies(t, h, env)
		require.Len(t, stored, 2)
		require.Equal(t, jwtauth, string(stored[0]), "existing policy must survive byte for byte")
		require.Contains(t, string(stored[1]), `"name":"deny"`)
	})

	t.Run("legacy empty-object blob counts as no policies", func(t *testing.T) {
		env := seedEnvironment(t, h)
		seedSentinelConfig(t, h, env, "{}")

		call(t, makeRequest(env, []openapi.Policy{firewallPolicy("deny", true)}))

		stored := readStoredPolicies(t, h, env)
		require.Len(t, stored, 1)
	})

	t.Run("concurrent creates lose no policies", func(t *testing.T) {
		env := seedEnvironment(t, h)

		const workers = 5
		var wg sync.WaitGroup
		errs := make([]int, workers)
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				res := testutil.CallRoute[handler.Request, handler.Response](h, route, headers,
					makeRequest(env, []openapi.Policy{firewallPolicy(fmt.Sprintf("policy-%d", i), true)}))
				errs[i] = res.Status
			}(i)
		}
		wg.Wait()

		for i, status := range errs {
			require.Equal(t, 200, status, "worker %d", i)
		}

		stored := readStoredPolicies(t, h, env)
		require.Len(t, stored, workers, "every concurrent create must survive the read-modify-write")
	})
}
