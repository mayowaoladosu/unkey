package handler

import (
	"context"

	"github.com/unkeyed/unkey/pkg/zen"
	"github.com/unkeyed/unkey/svc/api/internal/portalscope"
	getverifications "github.com/unkeyed/unkey/svc/api/routes/v2_analytics_get_verifications"
)

// Handler serves the portal-scoped variant of analytics.getVerifications. It
// authenticates only portal sessions and restricts results to verification
// events attributed to the session's external identity.
type Handler struct {
	*getverifications.Handler
}

// Path returns the URL path pattern this route matches.
func (h *Handler) Path() string { return "/v2/portal.getVerifications" }

// Handle returns verification analytics scoped to the portal session's external
// identity.
func (h *Handler) Handle(ctx context.Context, s *zen.Session) error {
	externalID, err := portalscope.ExternalID(s)
	if err != nil {
		return err
	}
	return h.Handler.Serve(ctx, s, externalID)
}
