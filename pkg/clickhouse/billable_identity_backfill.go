package clickhouse

import (
	"context"
	"fmt"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/unkeyed/unkey/pkg/fault"
)

// BackfillBillableIdentityRollups rebuilds the per-identity billable rollups
// (verifications, credits, ratelimits) for one CLOSED billing period from
// their source tables.
//
// Why this exists: the rollups are populated by materialized views, which only
// see rows inserted AFTER the view was created. A workspace whose usage
// predates the views (or a period already elapsed when the feature shipped) is
// therefore invisible to per-identity billing until it is backfilled once.
//
// It is idempotent by period: for each target it deletes the period's existing
// rows and re-derives them, so running it after the views have already
// populated the period does not double-count. The INSERT writes directly into
// the rollup tables (not their source), so it never re-triggers a view.
//
// Constraints:
//   - Run only for a CLOSED period (a past month). A period still accruing
//     usage would be rebuilt from a moving source and could disagree with the
//     views mid-flight.
//   - Verifications and credits are recoverable for as long as their source
//     key_verifications_per_month_v3 is retained (3 years). The ratelimits
//     rollup is derived from ratelimits_raw_v3, which has a 1-month TTL, so
//     only the most recently closed month can be reconstructed; older raw rows
//     are gone and their monthly rollup, once written, is the only record.
func (c *Client) BackfillBillableIdentityRollups(ctx context.Context, year, month int) error {
	// mutations_sync=1 makes each ALTER ... DELETE block until it has applied on
	// this node, so the following INSERT cannot race the delete it depends on.
	steps := []struct {
		name         string
		deleteFrom   string
		insertSelect string
	}{
		{
			name:       "verifications",
			deleteFrom: "billable_verifications_per_identity_per_month_v1",
			insertSelect: `
INSERT INTO default.billable_verifications_per_identity_per_month_v1
  (year, month, workspace_id, identity_id, external_id, count)
SELECT toYear(time), toMonth(time), workspace_id, identity_id, external_id, sum(count)
FROM default.key_verifications_per_month_v3
WHERE outcome = 'VALID' AND external_id != ''
  AND toYear(time) = {year:Int32} AND toMonth(time) = {month:Int32}
GROUP BY workspace_id, identity_id, external_id, toYear(time), toMonth(time)`,
		},
		{
			name:       "credits",
			deleteFrom: "billable_credits_per_identity_per_month_v1",
			insertSelect: `
INSERT INTO default.billable_credits_per_identity_per_month_v1
  (year, month, workspace_id, identity_id, external_id, spent_credits)
SELECT toYear(time), toMonth(time), workspace_id, identity_id, external_id, sum(spent_credits)
FROM default.key_verifications_per_month_v3
WHERE external_id != ''
  AND toYear(time) = {year:Int32} AND toMonth(time) = {month:Int32}
GROUP BY workspace_id, identity_id, external_id, toYear(time), toMonth(time)`,
		},
		{
			name:       "ratelimits",
			deleteFrom: "billable_ratelimits_per_identity_per_month_v1",
			insertSelect: `
INSERT INTO default.billable_ratelimits_per_identity_per_month_v1
  (year, month, workspace_id, identity_id, external_id, count)
SELECT
  toYear(fromUnixTimestamp64Milli(time)),
  toMonth(fromUnixTimestamp64Milli(time)),
  workspace_id, identity_id, external_id, toInt64(countIf(passed))
FROM default.ratelimits_raw_v3
WHERE external_id != ''
  AND toYear(fromUnixTimestamp64Milli(time)) = {year:Int32}
  AND toMonth(fromUnixTimestamp64Milli(time)) = {month:Int32}
GROUP BY workspace_id, identity_id, external_id,
  toYear(fromUnixTimestamp64Milli(time)), toMonth(fromUnixTimestamp64Milli(time))`,
		},
	}

	for _, step := range steps {
		deleteSQL := fmt.Sprintf(
			"ALTER TABLE default.%s DELETE WHERE year = {year:Int32} AND month = {month:Int32} SETTINGS mutations_sync = 1",
			step.deleteFrom,
		)
		if err := c.conn.Exec(ctx, deleteSQL, ch.Named("year", year), ch.Named("month", month)); err != nil {
			return fault.Wrap(err, fault.Internal("failed to clear "+step.name+" rollup for backfill"))
		}
		if err := c.conn.Exec(ctx, step.insertSelect, ch.Named("year", year), ch.Named("month", month)); err != nil {
			return fault.Wrap(err, fault.Internal("failed to backfill "+step.name+" rollup"))
		}
	}

	return nil
}
