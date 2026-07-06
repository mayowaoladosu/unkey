// Package portalscope resolves the external identity a portal route must scope
// to. Portal routes run behind the portal-only authenticator, so their
// principal is always a portal session; this centralizes the defensive checks
// and the fail-closed behavior every portal wrapper needs.
package portalscope

import (
	authprincipal "github.com/unkeyed/unkey/pkg/auth/principal"
	"github.com/unkeyed/unkey/pkg/codes"
	"github.com/unkeyed/unkey/pkg/fault"
	"github.com/unkeyed/unkey/pkg/zen"
)

// ExternalID returns the external identity the request must be scoped to.
//
// It requires the authenticated principal to be a portal session and to carry a
// non-empty external identity. Both failures are treated as broken invariants
// rather than routine rejections: portal routes are only ever registered behind
// the portal-only authenticator, so a non-portal principal or an empty external
// identity means the request reached the handler through a misconfiguration.
func ExternalID(s *zen.Session) (string, error) {
	principal, err := s.GetPrincipal()
	if err != nil {
		return "", err
	}

	src, ok := principal.Source.(authprincipal.PortalSessionSource)
	if !ok {
		return "", fault.New("non-portal principal on portal route",
			fault.Code(codes.Auth.Authorization.Forbidden.URN()),
			fault.Internal("principal source is not a portal session"),
			fault.Public("This endpoint is only accessible with a portal session."),
		)
	}

	if src.ExternalID == "" {
		return "", fault.New("portal session missing identity",
			fault.Code(codes.App.Internal.UnexpectedError.URN()),
			fault.Internal("portal session externalId is empty"),
			fault.Public("An internal error occurred."),
		)
	}

	return src.ExternalID, nil
}
