package handler

import (
	"fmt"
	"regexp"

	"github.com/unkeyed/unkey/pkg/codes"
	"github.com/unkeyed/unkey/pkg/fault"
	"github.com/unkeyed/unkey/pkg/ptr"
	"github.com/unkeyed/unkey/svc/api/openapi"
)

// validatePolicies enforces the rules the OpenAPI schema cannot express;
// a policy passing schema but failing here would otherwise misbehave at
// request time in the gateway.
func validatePolicies(policies []openapi.Policy) error {
	for i, p := range policies {
		if err := validatePolicy(fmt.Sprintf("policies[%d]", i), p); err != nil {
			return err
		}
	}
	return nil
}

// variantName names the set variant for audit metadata. The default is
// unreachable after validatePolicies; it only keeps the switch total.
func variantName(p openapi.Policy) string {
	switch {
	case p.Keyauth != nil:
		return "keyauth"
	case p.Ratelimit != nil:
		return "ratelimit"
	case p.Firewall != nil:
		return "firewall"
	case p.Openapi != nil:
		return "openapi"
	default:
		return "unknown"
	}
}

func validatePolicy(path string, p openapi.Policy) error {
	if err := exactlyOne(path, "keyauth, ratelimit, firewall or openapi",
		p.Keyauth != nil, p.Ratelimit != nil, p.Firewall != nil, p.Openapi != nil); err != nil {
		return err
	}

	// match is a sibling of the oneof, present on every variant.
	for i, m := range ptr.SafeDeref(p.Match) {
		if err := validateMatchExpr(fmt.Sprintf("%s.match[%d]", path, i), m); err != nil {
			return err
		}
	}

	switch {
	case p.Keyauth != nil:
		return validateKeyauth(path+".keyauth", *p.Keyauth)
	case p.Ratelimit != nil:
		id := p.Ratelimit.Identifier
		return exactlyOne(path+".ratelimit.identifier", "remoteIp, header, authenticatedSubject, path or principalField",
			id.RemoteIp != nil, id.Header != nil, id.AuthenticatedSubject != nil, id.Path != nil, id.PrincipalField != nil)
	}
	return nil
}

func validateMatchExpr(path string, m openapi.MatchExpr) error {
	if err := exactlyOne(path, "path, method, header or queryParam",
		m.Path != nil, m.Method != nil, m.Header != nil, m.QueryParam != nil); err != nil {
		return err
	}

	switch {
	case m.Path != nil:
		return validateStringMatch(path+".path.path", m.Path.Path)
	case m.Header != nil:
		return validateFieldMatch(path+".header", m.Header.Present != nil, m.Header.Value)
	case m.QueryParam != nil:
		return validateFieldMatch(path+".queryParam", m.QueryParam.Present != nil, m.QueryParam.Value)
	}
	return nil
}

// present matches mere existence, value matches content; exactly one applies.
func validateFieldMatch(path string, hasPresent bool, value *openapi.StringMatch) error {
	if err := exactlyOne(path, "present or value", hasPresent, value != nil); err != nil {
		return err
	}
	if value != nil {
		return validateStringMatch(path+".value", *value)
	}
	return nil
}

func validateStringMatch(path string, s openapi.StringMatch) error {
	if err := exactlyOne(path, "exact, prefix or regex",
		s.Exact != nil, s.Prefix != nil, s.Regex != nil); err != nil {
		return err
	}

	if s.Regex != nil {
		if _, err := regexp.Compile(*s.Regex); err != nil {
			return invalid(fmt.Sprintf("%s.regex is not a valid regular expression: %s", path, err))
		}
	}
	return nil
}

func validateKeyauth(path string, k openapi.KeyauthPolicy) error {
	for i, loc := range ptr.SafeDeref(k.Locations) {
		if err := exactlyOne(fmt.Sprintf("%s.locations[%d]", path, i), "bearer, header or queryParam",
			loc.Bearer != nil, loc.Header != nil, loc.QueryParam != nil); err != nil {
			return err
		}
	}
	for i, rl := range ptr.SafeDeref(k.Ratelimits) {
		if (rl.Limit == nil) != (rl.Duration == nil) {
			return invalid(fmt.Sprintf("%s.ratelimits[%d] must set limit and duration together.", path, i))
		}
	}
	return nil
}

// exactlyOne mirrors a proto oneof: exactly one variant must be set.
// variants renders verbatim into the error message.
func exactlyOne(path, variants string, set ...bool) error {
	n := 0
	for _, s := range set {
		if s {
			n++
		}
	}
	if n == 1 {
		return nil
	}
	detail := "none are set"
	if n > 1 {
		detail = fmt.Sprintf("%d are set", n)
	}
	return invalid(fmt.Sprintf("%s must set exactly one of %s; %s.", path, variants, detail))
}

func invalid(message string) error {
	return fault.New(
		"invalid policy",
		fault.Code(codes.App.Validation.InvalidInput.URN()),
		fault.Internal("policy validation failed"),
		fault.Public(message),
	)
}
