package handler

import (
	"context"

	"github.com/unkeyed/unkey/pkg/zen"
	"github.com/unkeyed/unkey/svc/api/internal/portalscope"
	createkey "github.com/unkeyed/unkey/svc/api/routes/v2_keys_create_key"
)

// Handler serves the portal-scoped variant of keys.createKey. It authenticates
// only portal sessions and forces the created key to belong to the session's
// external identity.
type Handler struct {
	*createkey.Handler
}

// Path returns the URL path pattern this route matches.
func (h *Handler) Path() string { return "/v2/portal.createKey" }

// Handle creates a key scoped to the portal session's external identity.
func (h *Handler) Handle(ctx context.Context, s *zen.Session) error {
	externalID, err := portalscope.ExternalID(s)
	if err != nil {
		return err
	}
	return h.Handler.Serve(ctx, s, externalID)
}
