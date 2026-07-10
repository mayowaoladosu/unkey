// Package portalrbac defines the portal's public capability vocabulary.
//
// A workspace owner creates a portal session with a small, stable vocabulary of
// capabilities ("keys:read", "keys:reroll", ...) scoped to a set of keyspaces.
// Portal handlers authorize these capabilities directly and separately enforce
// the session's workspace, keyspace, and external identity scope.
package portalrbac

import "fmt"

// Capability is one entry in the portal's simplified permission vocabulary. It
// is intentionally coarse and product-oriented; the fine-grained RBAC actions it
// maps to are an implementation detail owned by this package.
type Capability string

const (
	// CapKeysRead lets the end user list and read their own keys.
	CapKeysRead Capability = "keys:read"

	// CapKeysCreate lets the end user create new keys.
	CapKeysCreate Capability = "keys:create"

	// CapKeysReroll lets the end user rotate the secret of an existing key.
	CapKeysReroll Capability = "keys:reroll"

	// CapAnalyticsRead lets the end user read their verification analytics.
	CapAnalyticsRead Capability = "analytics:read"
)

// Parse validates a raw capability string and returns the typed Capability.
// Callers should use this at the API boundary (portal.createSession) to reject
// unknown verbs before they are ever persisted.
func Parse(s string) (Capability, error) {
	switch Capability(s) {
	case CapKeysRead, CapKeysCreate, CapKeysReroll, CapAnalyticsRead:
		return Capability(s), nil
	default:
		return "", fmt.Errorf("unknown portal capability %q", s)
	}
}

// ParseAll validates a slice of raw capability strings, returning the first
// error encountered.
func ParseAll(raw []string) ([]Capability, error) {
	caps := make([]Capability, 0, len(raw))
	for _, s := range raw {
		c, err := Parse(s)
		if err != nil {
			return nil, err
		}
		caps = append(caps, c)
	}
	return caps, nil
}
