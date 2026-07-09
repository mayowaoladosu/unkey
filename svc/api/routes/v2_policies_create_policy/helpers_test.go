package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/uid"
	"github.com/unkeyed/unkey/svc/api/internal/testutil"
	"github.com/unkeyed/unkey/svc/api/internal/testutil/seed"
	"github.com/unkeyed/unkey/svc/api/openapi"
	handler "github.com/unkeyed/unkey/svc/api/routes/v2_policies_create_policy"
)

func makeRequest(env seededEnv, policies []openapi.Policy) handler.Request {
	return handler.Request{
		Project:     env.projectID,
		App:         env.appID,
		Environment: env.environmentID,
		Policies:    policies,
	}
}

func firewallPolicy(name string, enabled bool) openapi.Policy {
	return openapi.Policy{
		Name:     name,
		Enabled:  enabled,
		Firewall: &openapi.FirewallPolicy{Action: "ACTION_DENY"},
	}
}

type seededEnv struct {
	workspaceID   string
	projectID     string
	appID         string
	environmentID string
}

func seedEnvironment(t *testing.T, h *testutil.Harness) seededEnv {
	t.Helper()

	workspace := h.Resources().UserWorkspace

	project := h.CreateProject(seed.CreateProjectRequest{
		ID:          uid.New(uid.ProjectPrefix),
		WorkspaceID: workspace.ID,
		Name:        "Payments Service",
		Slug:        strings.ToLower(strings.ReplaceAll(uid.New("test"), "_", "-")),
	})

	app := h.CreateApp(seed.CreateAppRequest{
		ID:            uid.New(uid.AppPrefix),
		WorkspaceID:   workspace.ID,
		ProjectID:     project.ID,
		Name:          "Payments API",
		Slug:          strings.ToLower(strings.ReplaceAll(uid.New("test"), "_", "-")),
		DefaultBranch: "main",
	})

	environment := h.CreateEnvironment(seed.CreateEnvironmentRequest{
		ID:          uid.New(uid.EnvironmentPrefix),
		WorkspaceID: workspace.ID,
		ProjectID:   project.ID,
		AppID:       app.ID,
		Slug:        "production",
		Description: "Production environment",
	})

	return seededEnv{
		workspaceID:   workspace.ID,
		projectID:     project.ID,
		appID:         app.ID,
		environmentID: environment.ID,
	}
}

// seedSentinelConfig overwrites the seeded runtime settings row's blob
// directly, bypassing the handler, so tests can set up pre-existing state
// including policy variants the API cannot create. The environment seeder
// always creates the row (with the legacy "{}" blob).
func seedSentinelConfig(t *testing.T, h *testutil.Harness, env seededEnv, blob string) {
	t.Helper()
	_, err := h.DB.RW().ExecContext(context.Background(),
		"UPDATE app_runtime_settings SET sentinel_config = ? WHERE app_id = ? AND environment_id = ?",
		blob, env.appID, env.environmentID)
	require.NoError(t, err)

	// MySQL reports 0 affected rows when the value is unchanged, so verify by
	// reading back instead.
	var stored []byte
	err = h.DB.RO().QueryRowContext(context.Background(),
		"SELECT sentinel_config FROM app_runtime_settings WHERE app_id = ? AND environment_id = ?",
		env.appID, env.environmentID).Scan(&stored)
	require.NoError(t, err)
	require.Equal(t, blob, string(stored))
}

// readStoredPolicies returns the raw policy documents currently stored for the
// environment, so tests can assert exact wire bytes.
func readStoredPolicies(t *testing.T, h *testutil.Harness, env seededEnv) []json.RawMessage {
	t.Helper()
	var blob []byte
	err := h.DB.RO().QueryRowContext(context.Background(),
		"SELECT sentinel_config FROM app_runtime_settings WHERE app_id = ? AND environment_id = ?",
		env.appID, env.environmentID).Scan(&blob)
	require.NoError(t, err)

	var envelope struct {
		Policies []json.RawMessage `json:"policies"`
	}
	require.NoError(t, json.Unmarshal(blob, &envelope))
	return envelope.Policies
}

func authHeaders(rootKey string) http.Header {
	return http.Header{
		"Content-Type":  {"application/json"},
		"Authorization": {fmt.Sprintf("Bearer %s", rootKey)},
	}
}

func ptr[T any](v T) *T {
	return &v
}
