package handler

import (
	"context"

	"github.com/unkeyed/unkey/pkg/zen"
	"github.com/unkeyed/unkey/svc/api/internal/portalscope"
	deletekey "github.com/unkeyed/unkey/svc/api/routes/v2_keys_delete_key"
)

// Handler serves the portal-scoped variant of keys.deleteKey. It authenticates
// only portal sessions and may only delete keys owned by the session's external
// identity.
type Handler struct {
	*deletekey.Handler
}

// Path returns the URL path pattern this route matches.
func (h *Handler) Path() string { return "/v2/portal.deleteKey" }

// Handle deletes a key scoped to the portal session's external identity.
func (h *Handler) Handle(ctx context.Context, s *zen.Session) error {
	externalID, err := portalscope.ExternalID(s)
	if err != nil {
		return err
	}
	return h.Handler.Serve(ctx, s, externalID)
}
