package handler_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/db"
	"github.com/unkeyed/unkey/pkg/uid"
	"github.com/unkeyed/unkey/svc/api/internal/testutil"
	handler "github.com/unkeyed/unkey/svc/api/routes/v2_portal_exchange_session"
)

func marshalPortalGrant(t *testing.T, keyspaceIDs, permissions []string) []byte {
	t.Helper()

	grant, err := json.Marshal(struct {
		KeyspaceIDs []string `json:"keyspaceIds"`
		Permissions []string `json:"permissions"`
	}{
		KeyspaceIDs: keyspaceIDs,
		Permissions: permissions,
	})
	require.NoError(t, err)
	return grant
}

func TestExchangeSessionSuccess(t *testing.T) {
	h := testutil.NewHarness(t)
	ctx := context.Background()

	route := &handler.Handler{DB: h.DB, Auditlogs: h.Auditlogs}
	h.Register(route, h.PublicMiddleware()...)

	workspaceID := h.Resources().UserWorkspace.ID
	portalConfigID := uid.New(uid.PortalConfigPrefix)
	now := time.Now().UnixMilli()

	keyspaceID := uid.New(uid.KeySpacePrefix)
	err := db.Query.InsertPortalConfig(ctx, h.DB.RW(), db.InsertPortalConfigParams{
		ID:          portalConfigID,
		WorkspaceID: workspaceID,
		Slug:        "test-portal",
		KeyAuthID:   sql.NullString{Valid: true, String: keyspaceID},
		Enabled:     true,
		CreatedAt:   now,
	})
	require.NoError(t, err)

	headers := http.Header{
		"Content-Type": {"application/json"},
	}

	t.Run("valid exchange", func(t *testing.T) {
		tokenID := uid.New(uid.PortalSessionTokenPrefix)
		permissions := marshalPortalGrant(t, []string{keyspaceID}, []string{"keys:read", "keys:reroll"})

		err := db.Query.InsertPortalSessionToken(ctx, h.DB.RW(), db.InsertPortalSessionTokenParams{
			ID:             tokenID,
			WorkspaceID:    workspaceID,
			PortalConfigID: portalConfigID,
			ExternalID:     "user_valid",
			Permissions:    permissions,
			ExpiresAt:      now + int64(15*time.Minute/time.Millisecond),
			CreatedAt:      now,
		})
		require.NoError(t, err)

		before := time.Now()

		req := handler.Request{SessionId: tokenID}
		res := testutil.CallRoute[handler.Request, handler.Response](h, route, headers, req)
		require.Equal(t, 200, res.Status)
		require.NotNil(t, res.Body)

		require.NotEmpty(t, res.Body.Data.Token)
		require.NotZero(t, res.Body.Data.ExpiresAt)
		require.NotEmpty(t, res.Body.Meta.RequestId)

		// Browser session expiry must be ~24 hours from now.
		after := time.Now()
		expectedLow := before.Add(24 * time.Hour).UnixMilli()
		expectedHigh := after.Add(24 * time.Hour).UnixMilli()
		require.GreaterOrEqual(t, res.Body.Data.ExpiresAt, expectedLow)
		require.LessOrEqual(t, res.Body.Data.ExpiresAt, expectedHigh)

		// Verify the browser session was persisted.
		session, err := db.Query.FindValidPortalSession(ctx, h.DB.RO(), db.FindValidPortalSessionParams{
			ID:  res.Body.Data.Token,
			Now: time.Now().UnixMilli(),
		})
		require.NoError(t, err)
		require.Equal(t, workspaceID, session.WorkspaceID)
		require.Equal(t, "user_valid", session.ExternalID)
		require.Equal(t, portalConfigID, session.PortalConfigID)
		require.JSONEq(t, string(permissions), string(session.Permissions))
	})

	t.Run("single-use enforcement", func(t *testing.T) {
		tokenID := uid.New(uid.PortalSessionTokenPrefix)
		permissions := marshalPortalGrant(t, []string{keyspaceID}, []string{"keys:read"})

		err := db.Query.InsertPortalSessionToken(ctx, h.DB.RW(), db.InsertPortalSessionTokenParams{
			ID:             tokenID,
			WorkspaceID:    workspaceID,
			PortalConfigID: portalConfigID,
			ExternalID:     "user_single_use",
			Permissions:    permissions,
			ExpiresAt:      now + int64(15*time.Minute/time.Millisecond),
			CreatedAt:      now,
		})
		require.NoError(t, err)

		req := handler.Request{SessionId: tokenID}

		// First exchange succeeds.
		res1 := testutil.CallRoute[handler.Request, handler.Response](h, route, headers, req)
		require.Equal(t, 200, res1.Status)

		// Second exchange must fail.
		res2 := testutil.CallRoute[handler.Request, handler.Response](h, route, headers, req)
		require.Equal(t, 401, res2.Status)
	})
}
