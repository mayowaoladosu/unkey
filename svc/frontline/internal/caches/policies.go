package caches

import frontlinev1 "github.com/unkeyed/unkey/gen/proto/frontline/v1"

// CachedPolicies is the policies cache value. Err is set when sentinel_config
// fails to parse; callers must still fail the request with that error.
type CachedPolicies struct {
	Policies []*frontlinev1.Policy
	Err      error
}
