package handler_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/uid"
	"github.com/unkeyed/unkey/svc/api/internal/testutil"
	"github.com/unkeyed/unkey/svc/api/openapi"
	handler "github.com/unkeyed/unkey/svc/api/routes/v2_policies_set_policies"
)

func TestSetPoliciesNotFound(t *testing.T) {
	h := testutil.NewHarness(t)

	route := &handler.Handler{DB: h.DB, Auditlogs: h.Auditlogs}
	h.Register(route)

	env := seedEnvironment(t, h)
	rootKey := h.CreateRootKey(env.workspaceID, "environment.*.set_policies")
	headers := authHeaders(rootKey)

	t.Run("nonexistent environment", func(t *testing.T) {
		req := makeRequest(env, []openapi.Policy{firewallPolicy("deny", true)})
		req.Environment = uid.New(uid.EnvironmentPrefix)
		res := testutil.CallRoute[handler.Request, handler.Response](h, route, headers, req)
		require.Equal(t, http.StatusNotFound, res.Status, "expected 404, received: %s", res.RawBody)
	})

	t.Run("nonexistent project", func(t *testing.T) {
		req := makeRequest(env, []openapi.Policy{firewallPolicy("deny", true)})
		req.Project = uid.New(uid.ProjectPrefix)
		res := testutil.CallRoute[handler.Request, handler.Response](h, route, headers, req)
		require.Equal(t, http.StatusNotFound, res.Status, "expected 404, received: %s", res.RawBody)
	})

	t.Run("nonexistent app", func(t *testing.T) {
		req := makeRequest(env, []openapi.Policy{firewallPolicy("deny", true)})
		req.App = uid.New(uid.AppPrefix)
		res := testutil.CallRoute[handler.Request, handler.Response](h, route, headers, req)
		require.Equal(t, http.StatusNotFound, res.Status, "expected 404, received: %s", res.RawBody)
	})

	t.Run("unknown policy id", func(t *testing.T) {
		update := firewallPolicy("ghost", true)
		update.Id = ptr("pol_doesnotexist")
		res := testutil.CallRoute[handler.Request, openapi.NotFoundErrorResponse](h, route, headers,
			makeRequest(env, []openapi.Policy{update}))
		require.Equal(t, http.StatusNotFound, res.Status, "expected 404, received: %s", res.RawBody)
		require.Contains(t, res.Body.Error.Type, "policy_not_found")

		// The rejected request must not have been written: the seeded row keeps
		// its legacy empty blob.
		require.Equal(t, "{}", readStoredBlob(t, h, env))
	})

	t.Run("keyauth referencing a nonexistent keyspace", func(t *testing.T) {
		res := testutil.CallRoute[handler.Request, openapi.NotFoundErrorResponse](h, route, headers,
			makeRequest(env, []openapi.Policy{{
				Name:    "k",
				Enabled: true,
				Keyauth: &openapi.KeyauthPolicy{KeySpaceIds: []string{uid.New(uid.KeySpacePrefix)}},
			}}))
		require.Equal(t, http.StatusNotFound, res.Status, "expected 404, received: %s", res.RawBody)
		require.Contains(t, res.Body.Error.Type, "key_space_not_found")

		require.Equal(t, "{}", readStoredBlob(t, h, env))
	})
}
