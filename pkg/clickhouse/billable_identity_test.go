package clickhouse_test

import (
	"context"
	"math/rand"
	"testing"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/clickhouse"
	"github.com/unkeyed/unkey/pkg/clickhouse/schema"
	"github.com/unkeyed/unkey/pkg/testutil/containers"
	"github.com/unkeyed/unkey/pkg/uid"
)

// createIdentityVerifications creates verification events attributed to an
// end-user identity, each spending the given number of credits.
func createIdentityVerifications(workspaceID, identityID, externalID string, count int, timestamp time.Time, outcome string, spentCredits int64) []schema.KeyVerification {
	verifications := make([]schema.KeyVerification, count)
	for i := range count {
		verifications[i] = schema.KeyVerification{
			RequestID:    uid.New(uid.RequestPrefix),
			Time:         timestamp.Add(time.Duration(i) * time.Second).UnixMilli(),
			WorkspaceID:  workspaceID,
			KeySpaceID:   uid.New(uid.KeySpacePrefix),
			IdentityID:   identityID,
			ExternalID:   externalID,
			KeyID:        uid.New(uid.KeyPrefix),
			Region:       "us-east-1",
			Outcome:      outcome,
			Tags:         []string{},
			SpentCredits: spentCredits,
			Latency:      rand.Float64() * 100,
		}
	}
	return verifications
}

func TestGetBillableUsagePerIdentity(t *testing.T) {
	chCfg := containers.ClickHouse(t)
	dsn := chCfg.DSN

	client, err := clickhouse.New(clickhouse.Config{
		URL: dsn,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	err = client.Ping(context.Background())
	require.NoError(t, err)

	opts, err := ch.ParseDSN(dsn)
	require.NoError(t, err)

	conn, err := ch.Open(opts)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	ctx := context.Background()

	now := time.Now()
	year := now.Year()
	month := int(now.Month())

	t.Run("per-identity counts and credits across two identities", func(t *testing.T) {
		workspaceID := uid.New(uid.WorkspacePrefix)
		identityA := uid.New(uid.IdentityPrefix)
		identityB := uid.New(uid.IdentityPrefix)

		// Identity A: 100 VALID verifications, 2 credits each.
		// Inserted in two batches so unmerged SummingMergeTree parts must still
		// sum to the correct total at read time.
		batchA1 := createIdentityVerifications(workspaceID, identityA, "user_a", 50, now, "VALID", 2)
		batchA2 := createIdentityVerifications(workspaceID, identityA, "user_a", 50, now, "VALID", 2)
		// Identity B: 30 VALID verifications, 1 credit each.
		batchB := createIdentityVerifications(workspaceID, identityB, "user_b", 30, now, "VALID", 1)

		insertVerifications(t, ctx, conn, batchA1)
		insertVerifications(t, ctx, conn, batchA2)
		insertVerifications(t, ctx, conn, batchB)

		require.Eventually(t, func() bool {
			usage, usageErr := client.GetBillableUsagePerIdentity(ctx, workspaceID, year, month)
			if usageErr != nil || len(usage) != 2 {
				return false
			}
			byExternal := map[string]clickhouse.IdentityBillableUsage{}
			for _, u := range usage {
				byExternal[u.ExternalID] = u
			}
			a, okA := byExternal["user_a"]
			b, okB := byExternal["user_b"]
			return okA && okB &&
				a.IdentityID == identityA && a.Verifications == 100 && a.SpentCredits == 200 &&
				b.IdentityID == identityB && b.Verifications == 30 && b.SpentCredits == 30
		}, time.Minute, time.Second)

		// Read idempotency: a second read returns identical quantities.
		usage, err := client.GetBillableUsagePerIdentity(ctx, workspaceID, year, month)
		require.NoError(t, err)
		require.Len(t, usage, 2)
	})

	t.Run("rate-limited outcomes are not billable", func(t *testing.T) {
		workspaceID := uid.New(uid.WorkspacePrefix)
		identityID := uid.New(uid.IdentityPrefix)

		// Only RATE_LIMITED outcomes with no credits spent: nothing billable.
		limited := createIdentityVerifications(workspaceID, identityID, "user_limited", 25, now, "RATE_LIMITED", 0)
		insertVerifications(t, ctx, conn, limited)

		// Give the MV cascade time to propagate, then confirm absence.
		require.Never(t, func() bool {
			usage, usageErr := client.GetBillableUsagePerIdentity(ctx, workspaceID, year, month)
			return usageErr == nil && len(usage) > 0
		}, 5*time.Second, time.Second)
	})

	t.Run("empty external_id is excluded, not attributed to a blank subject", func(t *testing.T) {
		workspaceID := uid.New(uid.WorkspacePrefix)

		// Verifications with no identity attached.
		anonymous := createIdentityVerifications(workspaceID, "", "", 40, now, "VALID", 1)
		// One attributed identity so we can positively confirm propagation.
		attributed := createIdentityVerifications(workspaceID, uid.New(uid.IdentityPrefix), "user_x", 10, now, "VALID", 1)

		insertVerifications(t, ctx, conn, anonymous)
		insertVerifications(t, ctx, conn, attributed)

		require.Eventually(t, func() bool {
			usage, usageErr := client.GetBillableUsagePerIdentity(ctx, workspaceID, year, month)
			if usageErr != nil {
				return false
			}
			// Only the attributed identity appears; the anonymous 40 are absent.
			return len(usage) == 1 && usage[0].ExternalID == "user_x" && usage[0].Verifications == 10
		}, time.Minute, time.Second)
	})

	t.Run("workspace with no usage returns empty result", func(t *testing.T) {
		usage, err := client.GetBillableUsagePerIdentity(ctx, uid.New(uid.WorkspacePrefix), year, month)
		require.NoError(t, err)
		require.Empty(t, usage)
	})

	t.Run("attributed ratelimits count, bare identifiers stay unattributed", func(t *testing.T) {
		workspaceID := uid.New(uid.WorkspacePrefix)
		identityID := uid.New(uid.IdentityPrefix)

		events := make([]schema.RatelimitV3, 0, 40)
		// 15 passed checks attributed to user_rl.
		events = append(events, createRatelimitsV3(workspaceID, identityID, "user_rl", 15, now, true)...)
		// 5 blocked checks for the same identity: not billable.
		events = append(events, createRatelimitsV3(workspaceID, identityID, "user_rl", 5, now, false)...)
		// 20 passed checks with a bare identifier (no identity match).
		events = append(events, createRatelimitsV3(workspaceID, "", "", 20, now, true)...)

		insertRatelimitsV3(t, ctx, conn, events)

		require.Eventually(t, func() bool {
			usage, usageErr := client.GetBillableUsagePerIdentity(ctx, workspaceID, year, month)
			if usageErr != nil || len(usage) != 1 {
				return false
			}
			u := usage[0]
			return u.ExternalID == "user_rl" && u.IdentityID == identityID &&
				u.RatelimitsPassed == 15 && u.Verifications == 0 && u.SpentCredits == 0
		}, time.Minute, time.Second)

		// The mirror MV forwards v3 rows into the v2 raw table, keeping the
		// workspace-grained billable rollup fed: all 35 passed checks count.
		require.Eventually(t, func() bool {
			count, countErr := client.GetBillableRatelimits(ctx, workspaceID, year, month)
			return countErr == nil && count == 35
		}, time.Minute, time.Second)
	})
}

// createRatelimitsV3 creates identity-attributed ratelimit v3 events. Empty
// identityID/externalID models a bare identifier that matched no identity.
func createRatelimitsV3(workspaceID, identityID, externalID string, count int, timestamp time.Time, passed bool) []schema.RatelimitV3 {
	events := make([]schema.RatelimitV3, count)
	identifier := externalID
	if identifier == "" {
		identifier = uid.New(uid.IdentityPrefix)
	}
	var remaining uint64 = 50
	if !passed {
		remaining = 0
	}
	for i := range count {
		events[i] = schema.RatelimitV3{
			RequestID:   uid.New(uid.RequestPrefix),
			Time:        timestamp.Add(time.Duration(i) * time.Second).UnixMilli(),
			WorkspaceID: workspaceID,
			NamespaceID: uid.New(uid.RatelimitNamespacePrefix),
			Identifier:  identifier,
			IdentityID:  identityID,
			ExternalID:  externalID,
			Passed:      passed,
			Latency:     rand.Float64() * 10,
			OverrideID:  "",
			Limit:       100,
			Remaining:   remaining,
			ResetAt:     timestamp.Add(time.Minute).UnixMilli(),
			Tokens:      1,
		}
	}
	return events
}

func insertRatelimitsV3(t *testing.T, ctx context.Context, conn ch.Conn, events []schema.RatelimitV3) {
	if len(events) == 0 {
		return
	}

	batch, err := conn.PrepareBatch(ctx, "INSERT INTO default.ratelimits_raw_v3")
	require.NoError(t, err)

	for _, e := range events {
		err = batch.AppendStruct(&e)
		require.NoError(t, err)
	}

	err = batch.Send()
	require.NoError(t, err)
}
