package caches

import (
	"math"

	frontlinev1 "github.com/unkeyed/unkey/gen/proto/frontline/v1"
	"github.com/unkeyed/unkey/svc/frontline/internal/db"
)

const (
	// Byte budgets for caches whose entry sizes vary widely.
	routeCacheByteBudget    = 32_000_000
	policiesCacheByteBudget = 64_000_000
)

func frontlineRouteCost(route db.FindFrontlineRouteByFQDNRow) uint32 {
	const fixedOverhead = uint32(256)
	return addUint32Clamped(fixedOverhead, uint32(len(route.SentinelConfig)))
}

func policiesCost(policies []*frontlinev1.Policy) uint32 {
	var n uint32 = 64
	for _, policy := range policies {
		n = addUint32Clamped(n, 64)
		openapi, ok := policy.GetConfig().(*frontlinev1.Policy_Openapi)
		if !ok {
			continue
		}
		n = addUint32Clamped(n, uint32(len(openapi.Openapi.SpecYaml)))
	}
	return n
}

func addUint32Clamped(a, b uint32) uint32 {
	if a > math.MaxUint32-b {
		return math.MaxUint32
	}
	return a + b
}
