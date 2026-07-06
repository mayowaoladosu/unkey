package handler_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/db"
	"github.com/unkeyed/unkey/pkg/ptr"
	"github.com/unkeyed/unkey/pkg/uid"
	"github.com/unkeyed/unkey/svc/api/internal/testutil"
	"github.com/unkeyed/unkey/svc/api/internal/testutil/seed"
	createkey "github.com/unkeyed/unkey/svc/api/routes/v2_keys_create_key"
	handler "github.com/unkeyed/unkey/svc/api/routes/v2_portal_create_key"
)

type (
	Request  = createkey.Request
	Response = createkey.Response
)

// newHandler builds the portal.createKey handler backed by a configured
// keys.createKey handler.
func newHandler(h *testutil.Harness) *handler.Handler {
	return &handler.Handler{
		Handler: &createkey.Handler{
			DB:        h.DB,
			Keys:      h.Keys,
			Auditlogs: h.Auditlogs,
			Vault:     h.Vault,
		},
	}
}

// portalCreateKeySetup holds all objects created for a portal createKey test.
type portalCreateKeySetup struct {
	apiID      string
	keySpaceID string
	workspace  db.Workspace
}

// setupPortalCreateKeyTest creates a workspace, API, and keyspace for portal createKey testing.
func setupPortalCreateKeyTest(t *testing.T, h *testutil.Harness) portalCreateKeySetup {
	t.Helper()
	ctx := context.Background()

	workspace := h.Resources().UserWorkspace

	keySpaceID := uid.New(uid.KeySpacePrefix)
	err := db.Query.InsertKeySpace(ctx, h.DB.RW(), db.InsertKeySpaceParams{
		ID:            keySpaceID,
		WorkspaceID:   workspace.ID,
		CreatedAtM:    time.Now().UnixMilli(),
		DefaultPrefix: sql.NullString{Valid: false},
		DefaultBytes:  sql.NullInt32{Valid: false},
	})
	require.NoError(t, err)

	apiID := uid.New("api")
	err = db.Query.InsertApi(ctx, h.DB.RW(), db.InsertApiParams{
		ID:          apiID,
		Name:        "Portal CreateKey Test API",
		WorkspaceID: workspace.ID,
		AuthType:    db.NullApisAuthType{Valid: true, ApisAuthType: db.ApisAuthTypeKey},
		KeyAuthID:   sql.NullString{Valid: true, String: keySpaceID},
		CreatedAtM:  time.Now().UnixMilli(),
	})
	require.NoError(t, err)

	return portalCreateKeySetup{
		apiID:      apiID,
		keySpaceID: keySpaceID,
		workspace:  workspace,
	}
}

func TestPortalSessionCreateKeyOwnedBySessionIdentity(t *testing.T) {
	h := testutil.NewHarness(t)
	ctx := context.Background()

	route := newHandler(h)
	h.Register(route, h.PortalMiddleware()...)

	setup := setupPortalCreateKeyTest(t, h)

	externalID := "portal_user_A"
	headers := h.CreatePortalSession(setup.workspace.ID, externalID, []string{
		fmt.Sprintf("api.%s.create_key", setup.apiID),
	})

	req := Request{
		ApiId: setup.apiID,
	}

	res := testutil.CallRoute[Request, Response](h, route, headers, req)

	require.Equal(t, 200, res.Status)
	require.NotNil(t, res.Body)
	require.NotEmpty(t, res.Body.Data.KeyId)
	require.NotEmpty(t, res.Body.Data.Key)

	// Verify key is owned by the session's externalId identity
	key, err := db.Query.FindKeyByID(ctx, h.DB.RO(), res.Body.Data.KeyId)
	require.NoError(t, err)
	require.True(t, key.IdentityID.Valid, "key should have an identity assigned")

	identity, err := db.Query.FindIdentityByExternalID(ctx, h.DB.RO(), db.FindIdentityByExternalIDParams{
		WorkspaceID: setup.workspace.ID,
		ExternalID:  externalID,
		Deleted:     false,
	})
	require.NoError(t, err)
	require.Equal(t, identity.ID, key.IdentityID.String)
}

func TestPortalSessionCreateKeyIgnoresClientExternalId(t *testing.T) {
	h := testutil.NewHarness(t)
	ctx := context.Background()

	route := newHandler(h)
	h.Register(route, h.PortalMiddleware()...)

	setup := setupPortalCreateKeyTest(t, h)

	sessionExternalID := "portal_user_A"
	headers := h.CreatePortalSession(setup.workspace.ID, sessionExternalID, []string{
		fmt.Sprintf("api.%s.create_key", setup.apiID),
	})

	// Pre-create identity for user B to make sure the key does NOT get assigned to it
	otherExternalID := "portal_user_B"
	h.CreateIdentity(seed.CreateIdentityRequest{
		WorkspaceID: setup.workspace.ID,
		ExternalID:  otherExternalID,
	})

	// Client supplies a different externalId — it should be ignored
	req := Request{
		ApiId:      setup.apiID,
		ExternalId: &otherExternalID,
	}

	res := testutil.CallRoute[Request, Response](h, route, headers, req)

	require.Equal(t, 200, res.Status)
	require.NotNil(t, res.Body)
	require.NotEmpty(t, res.Body.Data.KeyId)

	// Verify key is owned by session's identity (user A), not client-supplied (user B)
	key, err := db.Query.FindKeyByID(ctx, h.DB.RO(), res.Body.Data.KeyId)
	require.NoError(t, err)
	require.True(t, key.IdentityID.Valid)

	identity, err := db.Query.FindIdentityByExternalID(ctx, h.DB.RO(), db.FindIdentityByExternalIDParams{
		WorkspaceID: setup.workspace.ID,
		ExternalID:  sessionExternalID,
		Deleted:     false,
	})
	require.NoError(t, err)
	require.Equal(t, identity.ID, key.IdentityID.String)
}

func TestPortalSessionsCreateKeysWithDistinctIdentities(t *testing.T) {
	h := testutil.NewHarness(t)

	createRoute := newHandler(h)
	h.Register(createRoute, h.PortalMiddleware()...)

	setup := setupPortalCreateKeyTest(t, h)

	externalID := "portal_user_A"
	headers := h.CreatePortalSession(setup.workspace.ID, externalID, []string{
		fmt.Sprintf("api.%s.create_key", setup.apiID),
	})

	// Create a key via portal session A
	req := Request{
		ApiId: setup.apiID,
		Name:  ptr.P("Portal Created Key"),
	}

	res := testutil.CallRoute[Request, Response](h, createRoute, headers, req)
	require.Equal(t, 200, res.Status)
	require.NotEmpty(t, res.Body.Data.KeyId)

	// Create a key via a different portal session (user B)
	differentExternalID := "portal_user_B"
	differentHeaders := h.CreatePortalSession(setup.workspace.ID, differentExternalID, []string{
		fmt.Sprintf("api.%s.create_key", setup.apiID),
	})

	res2 := testutil.CallRoute[Request, Response](h, createRoute, differentHeaders, Request{
		ApiId: setup.apiID,
		Name:  ptr.P("Different User Key"),
	})
	require.Equal(t, 200, res2.Status)
	require.NotEmpty(t, res2.Body.Data.KeyId)

	// Each key should be owned by its respective session's identity
	ctx := context.Background()
	key1, err := db.Query.FindKeyByID(ctx, h.DB.RO(), res.Body.Data.KeyId)
	require.NoError(t, err)
	key2, err := db.Query.FindKeyByID(ctx, h.DB.RO(), res2.Body.Data.KeyId)
	require.NoError(t, err)

	require.True(t, key1.IdentityID.Valid)
	require.True(t, key2.IdentityID.Valid)
	require.NotEqual(t, key1.IdentityID.String, key2.IdentityID.String,
		"keys created by different portal sessions should belong to different identities")
}

func TestPortalSessionCreateKeyAuditLog(t *testing.T) {
	h := testutil.NewHarness(t)
	ctx := context.Background()

	route := newHandler(h)
	h.Register(route, h.PortalMiddleware()...)

	setup := setupPortalCreateKeyTest(t, h)

	externalID := "portal_user_audit"
	headers := h.CreatePortalSession(setup.workspace.ID, externalID, []string{
		fmt.Sprintf("api.%s.create_key", setup.apiID),
	})

	res := testutil.CallRoute[Request, Response](h, route, headers, Request{
		ApiId: setup.apiID,
	})
	require.Equal(t, 200, res.Status)
	require.NotEmpty(t, res.Body.Data.KeyId)
	require.NotEmpty(t, res.Body.Data.Key)

	auditLogs := h.FindAuditLogsByTargetID(ctx, t, res.Body.Data.KeyId)
	require.NotEmpty(t, auditLogs, "expected audit logs for the key created via portal session")

	var foundCreateEvent bool
	for _, ev := range auditLogs {
		if ev.Event != "key.create" {
			continue
		}
		foundCreateEvent = true
		require.Equal(t, "portalEndUser", ev.Actor.Type, "portal-created key must be attributed to a portalEndUser actor")
		require.Equal(t, externalID, ev.Actor.ID, "portal actor ID must be the end user's externalId")
		require.NotContains(t, ev.Actor.Meta, "sessionId", "session token must not be persisted in audit metadata")
	}
	require.True(t, foundCreateEvent, "expected a key.create audit log event")

	// The plaintext key value must never appear in any audit log.
	for _, ev := range auditLogs {
		payload, marshalErr := json.Marshal(ev)
		require.NoError(t, marshalErr)
		require.NotContains(t, string(payload), res.Body.Data.Key,
			"audit log must not contain the plaintext key value")
	}
}
