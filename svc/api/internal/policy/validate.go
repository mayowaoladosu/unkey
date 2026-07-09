package policy

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/unkeyed/unkey/pkg/codes"
	"github.com/unkeyed/unkey/pkg/fault"
	"github.com/unkeyed/unkey/pkg/ptr"
	"github.com/unkeyed/unkey/svc/api/openapi"
)

// ValidatePolicies enforces the invariants the OpenAPI schema cannot express:
// every union in the policy tree is a proto oneof so exactly one variant must
// be set, regexes must compile (the gateway compiles them lazily and fails the
// first matching request otherwise), and inline key ratelimit overrides need
// limit and duration together because the gateway silently drops partial
// overrides.
//
// The tree walked here:
//
//	Policy         oneof{keyauth,ratelimit,firewall,openapi} + match[]
//	MatchExpr      oneof{path,method,header,queryParam}
//	StringMatch    oneof{exact,prefix,regex}
//	KeyauthPolicy  locations[] + ratelimits[]
//	KeyLocation    oneof{bearer,header,queryParam}
//
// Each validator first checks its oneof count, then descends into the set
// variant with a switch on the variant pointer so the compiler catches a
// mistyped variant. A switch is used when more than one variant needs a
// descent, an if when only one does; the rest are leaves.
func ValidatePolicies(policies []openapi.Policy) error {
	for i, p := range policies {
		if err := validatePolicy(fmt.Sprintf("policies[%d]", i), p); err != nil {
			return err
		}
	}
	return nil
}

func validatePolicy(path string, p openapi.Policy) error {
	if err := oneof(
		path,
		opt("keyauth", p.Keyauth != nil),
		opt("ratelimit", p.Ratelimit != nil),
		opt("firewall", p.Firewall != nil),
		opt("openapi", p.Openapi != nil),
	); err != nil {
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
		return validateRatelimitIdentifier(path+".ratelimit", *p.Ratelimit)
	}
	return nil
}

func validateMatchExpr(path string, m openapi.MatchExpr) error {
	if err := oneof(
		path,
		opt("path", m.Path != nil),
		opt("method", m.Method != nil),
		opt("header", m.Header != nil),
		opt("queryParam", m.QueryParam != nil),
	); err != nil {
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

// header and queryParam match a request field one of two ways: `present`
// checks only that the field exists (ignoring its value), while `value` checks
// the field's value against a StringMatch. Exactly one must be set. `present`'s
// generated type differs between the two callers, so the caller passes whether
// it is set rather than the pointer itself.
func validateFieldMatch(path string, matchesExistence bool, value *openapi.StringMatch) error {
	if err := oneof(
		path,
		opt("present", matchesExistence),
		opt("value", value != nil),
	); err != nil {
		return err
	}
	if value != nil {
		return validateStringMatch(path+".value", *value)
	}
	return nil
}

func validateStringMatch(path string, s openapi.StringMatch) error {
	if err := oneof(
		path,
		opt("exact", s.Exact != nil),
		opt("prefix", s.Prefix != nil),
		opt("regex", s.Regex != nil),
	); err != nil {
		return err
	}
	if s.Regex != nil {
		return validateRegex(path+".regex", *s.Regex)
	}
	return nil
}

// The gateway compiles patterns lazily on the first matching request and
// fails that request on a bad pattern, so reject invalid regexes here where
// the caller can still fix them. Both sides use Go's stdlib regexp (see
// svc/frontline/internal/policies/match.go), so compiling here is an exact
// preflight; the gateway's `(?i)` ignoreCase prefix cannot invalidate a
// pattern that compiles.
func validateRegex(path string, pattern string) error {
	if _, err := regexp.Compile(pattern); err != nil {
		return invalid(fmt.Sprintf("%s is not a valid regular expression: %s", path, err))
	}
	return nil
}

func validateKeyauth(path string, k openapi.KeyauthPolicy) error {
	for i, loc := range ptr.SafeDeref(k.Locations) {
		if err := validateKeyLocation(fmt.Sprintf("%s.locations[%d]", path, i), loc); err != nil {
			return err
		}
	}
	for i, rl := range ptr.SafeDeref(k.Ratelimits) {
		if err := validateKeyRatelimit(fmt.Sprintf("%s.ratelimits[%d]", path, i), rl); err != nil {
			return err
		}
	}
	return nil
}

func validateKeyLocation(path string, loc openapi.KeyLocation) error {
	return oneof(
		path,
		opt("bearer", loc.Bearer != nil),
		opt("header", loc.Header != nil),
		opt("queryParam", loc.QueryParam != nil),
	)
}

func validateKeyRatelimit(path string, rl openapi.KeyRatelimit) error {
	if (rl.Limit == nil) != (rl.Duration == nil) {
		return invalid(path + " must set limit and duration together.")
	}
	return nil
}

func validateRatelimitIdentifier(path string, r openapi.RatelimitPolicy) error {
	return oneof(
		path+".identifier",
		opt("remoteIp", r.Identifier.RemoteIp != nil),
		opt("header", r.Identifier.Header != nil),
		opt("authenticatedSubject", r.Identifier.AuthenticatedSubject != nil),
		opt("path", r.Identifier.Path != nil),
		opt("principalField", r.Identifier.PrincipalField != nil),
	)
}

// ── Helpers ─────────────────────────────────────────────────────────────

// option pairs a oneof variant's wire name with whether the request set it.
type option struct {
	name string
	set  bool
}

func opt(name string, set bool) option {
	return option{name: name, set: set}
}

// oneof rejects the value at path unless exactly one variant is set, mirroring
// the proto oneofs the policy tree is built from.
func oneof(path string, options ...option) error {
	set := 0
	for _, o := range options {
		if o.set {
			set++
		}
	}
	if set == 1 {
		return nil
	}

	names := make([]string, len(options))
	for i, o := range options {
		names[i] = o.name
	}
	list := names[0]
	if last := len(names) - 1; last > 0 {
		list = fmt.Sprintf("%s or %s", strings.Join(names[:last], ", "), names[last])
	}
	detail := "none are set"
	if set > 1 {
		detail = fmt.Sprintf("%d are set", set)
	}
	return invalid(fmt.Sprintf("%s must set exactly one of %s; %s.", path, list, detail))
}

func invalid(message string) error {
	return fault.New(
		"invalid policy",
		fault.Code(codes.App.Validation.InvalidInput.URN()),
		fault.Internal("policy validation failed"),
		fault.Public(message),
	)
}
