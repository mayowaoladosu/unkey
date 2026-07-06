package handler

import (
	"context"

	"github.com/unkeyed/unkey/pkg/zen"
	"github.com/unkeyed/unkey/svc/api/internal/portalscope"
	rerollkey "github.com/unkeyed/unkey/svc/api/routes/v2_keys_reroll_key"
)

// Handler serves the portal-scoped variant of keys.rerollKey. It authenticates
// only portal sessions and may only reroll keys owned by the session's external
// identity.
type Handler struct {
	*rerollkey.Handler
}

// Path returns the URL path pattern this route matches.
func (h *Handler) Path() string { return "/v2/portal.rerollKey" }

// Handle rerolls a key scoped to the portal session's external identity.
func (h *Handler) Handle(ctx context.Context, s *zen.Session) error {
	externalID, err := portalscope.ExternalID(s)
	if err != nil {
		return err
	}
	return h.Handler.Serve(ctx, s, externalID)
}
