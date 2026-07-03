// Package deploylistbench benchmarks candidate SQL shapes for the
// deployments.listDeployments endpoint against a real MySQL instance.
//
// It answers two questions:
//  1. Does the `(? IS NULL OR col = ?)` optional-filter pattern defeat indexes
//     vs clean per-shape predicates?
//  2. Does ORDER BY created_at DESC filesort without a (filter_col, created_at)
//     composite, and do composites remove it?
//
// Run:
//
//	BENCH_ROWS=200000 mise exec -- bazel test //svc/api/internal/deploylistbench:deploylistbench_test \
//	  --test_output=all --test_timeout=1800 --test_env=BENCH_ROWS
package deploylistbench_test

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/pkg/testutil/containers"
)

// createTableDDL is the minimal deployments schema needed for the benchmark,
// lifted from pkg/mysql/schema/deployments.sql (base table only, its own
// single-column indexes; composites are added per index config at runtime).
const createTableDDL = "CREATE TABLE `deployments` (" +
	"`pk` bigint unsigned AUTO_INCREMENT NOT NULL," +
	"`id` varchar(128) NOT NULL," +
	"`k8s_name` varchar(255) NOT NULL," +
	"`workspace_id` varchar(256) NOT NULL," +
	"`project_id` varchar(256) NOT NULL," +
	"`environment_id` varchar(128) NOT NULL," +
	"`app_id` varchar(64) NOT NULL," +
	"`image` varchar(256)," +
	"`build_id` varchar(128)," +
	"`git_commit_sha` varchar(40)," +
	"`git_branch` varchar(256)," +
	"`git_commit_message` text," +
	"`git_commit_author_handle` varchar(256)," +
	"`git_commit_author_avatar_url` varchar(512)," +
	"`git_commit_timestamp` bigint," +
	"`sentinel_config` longblob NOT NULL," +
	"`cpu_millicores` int NOT NULL," +
	"`memory_mib` int NOT NULL," +
	"`storage_mib` int unsigned NOT NULL DEFAULT 0," +
	"`desired_state` enum('running','stopped') NOT NULL DEFAULT 'running'," +
	"`encrypted_environment_variables` longblob NOT NULL," +
	"`command` json NOT NULL," +
	"`port` int NOT NULL DEFAULT 8080," +
	"`shutdown_signal` enum('SIGTERM','SIGINT','SIGQUIT','SIGKILL') NOT NULL DEFAULT 'SIGTERM'," +
	"`upstream_protocol` enum('http1','h2c') NOT NULL DEFAULT 'http1'," +
	"`healthcheck` json," +
	"`pr_number` bigint," +
	"`fork_repository_full_name` varchar(256)," +
	"`github_deployment_id` bigint," +
	"`invocation_id` varchar(256)," +
	"`status` enum('pending','starting','building','deploying','network','finalizing','ready','failed','skipped','awaiting_approval','stopped','superseded','cancelled') NOT NULL DEFAULT 'pending'," +
	"`trigger` enum('unknown','github','api','cli','dashboard','unkey') NOT NULL DEFAULT 'unknown'," +
	"`triggered_by` varchar(256)," +
	"`trigger_reason` varchar(512)," +
	"`created_at` bigint NOT NULL," +
	"`updated_at` bigint," +
	"PRIMARY KEY(`pk`), UNIQUE(`id`), UNIQUE(`k8s_name`)," +
	"INDEX `workspace_idx`(`workspace_id`), INDEX `project_idx`(`project_id`), INDEX `status_idx`(`status`)" +
	")"

// allStatuses mirrors what the handler passes when no status filter is set:
// the full enum, so the IN clause is never empty.
const allStatuses = `'pending','starting','building','deploying','network','finalizing','ready','failed','skipped','awaiting_approval','stopped','superseded','cancelled'`

var statusPool = []string{"ready", "ready", "ready", "ready", "failed", "building", "deploying", "pending", "superseded", "stopped"}

type dbtx interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func seed(ctx context.Context, t *testing.T, exec dbtx, total int) {
	t.Helper()

	const (
		numOtherWorkspaces = 19
		hotShare           = 0.40
		projectsPerWs      = 5
		appsPerProject     = 4
		envsPerApp         = 3
	)
	rng := rand.New(rand.NewSource(42))
	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	const spanMs = int64(90) * 24 * 60 * 60 * 1000

	start := time.Now()
	const batch = 1000
	var b strings.Builder
	rowsInBatch := 0

	flush := func() {
		if rowsInBatch == 0 {
			return
		}
		_, err := exec.ExecContext(ctx, b.String())
		require.NoError(t, err)
		b.Reset()
		rowsInBatch = 0
	}

	for i := range total {
		var ws string
		if rng.Float64() < hotShare {
			ws = "ws_hot"
		} else {
			ws = fmt.Sprintf("ws_%d", rng.Intn(numOtherWorkspaces))
		}
		project := fmt.Sprintf("%s_p%d", ws, rng.Intn(projectsPerWs))
		app := fmt.Sprintf("%s_a%d", project, rng.Intn(appsPerProject))
		env := fmt.Sprintf("%s_e%d", app, rng.Intn(envsPerApp))
		status := statusPool[rng.Intn(len(statusPool))]
		createdAt := baseTime + rng.Int63n(spanMs)
		id := fmt.Sprintf("dep_%012d", i)

		if rowsInBatch == 0 {
			b.WriteString("INSERT INTO deployments (id, k8s_name, workspace_id, project_id, environment_id, app_id, sentinel_config, cpu_millicores, memory_mib, storage_mib, desired_state, encrypted_environment_variables, command, port, shutdown_signal, upstream_protocol, status, `trigger`, created_at) VALUES ")
		} else {
			b.WriteString(",")
		}
		fmt.Fprintf(&b,
			"('%s','k8s-%s','%s','%s','%s','%s',_binary '{}',100,128,0,'running',_binary '','[]',8080,'SIGTERM','http1','%s','api',%d)",
			id, id, ws, project, env, app, status, createdAt,
		)
		rowsInBatch++
		if rowsInBatch >= batch {
			flush()
		}
	}
	flush()
	t.Logf("seeded %d rows in %s", total, time.Since(start).Round(time.Millisecond))
}

// candidate is one query shape for a given filter scenario.
type candidate struct {
	shape string // "or-null" or "clean"
	query string
}

// filterCase is a filter scenario plus the two query shapes that satisfy it.
type filterCase struct {
	name       string
	candidates []candidate
}

func filterCases() []filterCase {
	// Hot-tenant target ids: worst case for each filter level.
	const (
		hotWs   = "ws_hot"
		hotProj = "ws_hot_p0"
		hotApp  = "ws_hot_p0_a0"
		hotEnv  = "ws_hot_p0_a0_e0"
	)
	orderLimit := "ORDER BY created_at DESC, id DESC LIMIT 100"

	// or-null always filters on workspace_id then applies the optional predicates.
	orNull := func(project, app, env string) string {
		q := func(v string) string {
			if v == "" {
				return "NULL"
			}
			return "'" + v + "'"
		}
		return fmt.Sprintf(
			"SELECT * FROM deployments WHERE workspace_id='%s' "+
				"AND (%s IS NULL OR project_id=%s) "+
				"AND (%s IS NULL OR app_id=%s) "+
				"AND (%s IS NULL OR environment_id=%s) "+
				"AND status IN (%s) %s",
			hotWs, q(project), q(project), q(app), q(app), q(env), q(env), allStatuses, orderLimit,
		)
	}
	clean := func(where string) string {
		return fmt.Sprintf("SELECT * FROM deployments WHERE %s AND status IN (%s) %s", where, allStatuses, orderLimit)
	}

	// Multi-status "show me problem deployments" — the realistic use of the
	// status[] filter. Two shapes to compare against clean-IN:
	//   cleanMulti: single query, status IN (a,b)
	//   union:      one indexed branch per status, each pre-sorted + limited,
	//               merged and re-limited. Avoids sorting the full union set.
	statuses := []string{"failed", "building"}
	cleanMulti := fmt.Sprintf(
		"SELECT * FROM deployments WHERE environment_id='%s' AND status IN ('failed','building') %s",
		hotEnv, orderLimit,
	)
	var branches []string
	for _, st := range statuses {
		branches = append(branches, fmt.Sprintf(
			"(SELECT * FROM deployments WHERE environment_id='%s' AND status='%s' ORDER BY created_at DESC, id DESC LIMIT 100)",
			hotEnv, st,
		))
	}
	union := strings.Join(branches, " UNION ALL ") + " ORDER BY created_at DESC, id DESC LIMIT 100"

	return []filterCase{
		{
			name: "none (workspace-wide)",
			candidates: []candidate{
				{"or-null", orNull("", "", "")},
				{"clean", clean("workspace_id='" + hotWs + "'")},
			},
		},
		{
			name: "project",
			candidates: []candidate{
				{"or-null", orNull(hotProj, "", "")},
				{"clean", clean("project_id='" + hotProj + "'")},
			},
		},
		{
			name: "environment",
			candidates: []candidate{
				{"or-null", orNull(hotProj, hotApp, hotEnv)},
				{"clean", clean("environment_id='" + hotEnv + "'")},
			},
		},
		{
			name: "env + status[failed,building]",
			candidates: []candidate{
				{"clean", cleanMulti},
				{"union", union},
			},
		},
	}
}

// indexConfig is a named set of composite indexes to create before a run.
type indexConfig struct {
	name    string
	creates []string
	drops   []string
}

func indexConfigs() []indexConfig {
	composites := map[string]string{
		"bench_ws_created":     "(workspace_id, created_at, id)",
		"bench_prj_created":    "(project_id, created_at, id)",
		"bench_app_created":    "(app_id, created_at, id)",
		"bench_env_created":    "(environment_id, created_at, id)",
		"bench_env_status_crt": "(environment_id, status, created_at, id)",
	}
	var creates, drops []string
	for name, cols := range composites {
		creates = append(creates, fmt.Sprintf("CREATE INDEX %s ON deployments %s", name, cols))
		drops = append(drops, fmt.Sprintf("DROP INDEX %s ON deployments", name))
	}
	return []indexConfig{
		{name: "no-index", creates: nil, drops: nil},
		{name: "composites", creates: creates, drops: drops},
	}
}

func explain(ctx context.Context, t *testing.T, exec dbtx, query string) (plan string, filesort bool) {
	t.Helper()
	rows, err := exec.QueryContext(ctx, "EXPLAIN ANALYZE "+query)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var sb strings.Builder
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	require.NoError(t, rows.Err())
	plan = sb.String()
	return plan, strings.Contains(plan, "Sort") || strings.Contains(strings.ToLower(plan), "filesort")
}

func timeQuery(ctx context.Context, t *testing.T, exec dbtx, query string, iterations int) time.Duration {
	t.Helper()
	durations := make([]time.Duration, iterations)
	for i := range iterations {
		start := time.Now()
		rows, err := exec.QueryContext(ctx, query)
		require.NoError(t, err)
		for rows.Next() { // drain
		}
		require.NoError(t, rows.Err())
		_ = rows.Close()
		durations[i] = time.Since(start)
	}
	// median
	for i := 1; i < len(durations); i++ {
		for j := i; j > 0 && durations[j] < durations[j-1]; j-- {
			durations[j], durations[j-1] = durations[j-1], durations[j]
		}
	}
	return durations[len(durations)/2]
}

type result struct {
	filter   string
	shape    string
	index    string
	filesort bool
	median   time.Duration
	plan     string
	query    string
}

func TestDeploymentListBench(t *testing.T) {
	ctx := context.Background()

	// Dedicated disk-backed MySQL (no 256MB tmpfs cap) so we can seed millions.
	cfg := containers.MySQL(t, containers.WithDiskStorage())
	sqlDB, err := sql.Open("mysql", cfg.DSN)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, sqlDB.PingContext(ctx))
	exec := sqlDB

	_, _ = exec.ExecContext(ctx, "DROP TABLE IF EXISTS deployments")
	_, err = exec.ExecContext(ctx, createTableDDL)
	require.NoError(t, err, "create deployments table")

	total := envInt("BENCH_ROWS", 100000)
	iterations := envInt("BENCH_ITERS", 15)

	seed(ctx, t, exec, total)

	var results []result

	for _, ic := range indexConfigs() {
		for _, c := range ic.creates {
			_, err := exec.ExecContext(ctx, c)
			require.NoError(t, err, "create index: %s", c)
		}

		for _, fc := range filterCases() {
			for _, cand := range fc.candidates {
				plan, filesort := explain(ctx, t, exec, cand.query)
				median := timeQuery(ctx, t, exec, cand.query, iterations)
				results = append(results, result{fc.name, cand.shape, ic.name, filesort, median, plan, cand.query})
			}
		}

		for _, d := range ic.drops {
			_, err := exec.ExecContext(ctx, d)
			require.NoError(t, err, "drop index: %s", d)
		}
	}

	// Plain-text summary to the test log.
	var tbl strings.Builder
	fmt.Fprintf(&tbl, "\n\nBENCHMARK SUMMARY (rows=%d, iters=%d)\n", total, iterations)
	fmt.Fprintf(&tbl, "%-30s %-8s %-12s %-9s %-10s\n", "FILTER", "SHAPE", "INDEX", "FILESORT", "MEDIAN")
	fmt.Fprintf(&tbl, "%s\n", strings.Repeat("-", 74))
	for _, r := range results {
		fmt.Fprintf(&tbl, "%-30s %-8s %-12s %-9v %-10s\n", r.filter, r.shape, r.index, r.filesort, r.median.Round(time.Microsecond))
	}
	t.Log(tbl.String())

	writeHTML(t, total, iterations, results)
}

func writeHTML(t *testing.T, total, iterations int, results []result) {
	t.Helper()
	outDir := os.Getenv("TEST_UNDECLARED_OUTPUTS_DIR")
	if outDir == "" {
		outDir = os.TempDir()
	}
	path := outDir + "/bench.html"

	var b strings.Builder
	b.WriteString(`<meta charset="utf-8"><style>
body{font:14px/1.5 -apple-system,system-ui,sans-serif;margin:2rem;color:#111;background:#fafafa}
h1{font-size:1.4rem}h2{font-size:1.05rem;margin-top:2rem;color:#333}
table{border-collapse:collapse;width:100%;margin:.5rem 0 1.5rem;background:#fff;box-shadow:0 1px 3px rgba(0,0,0,.08)}
th,td{padding:.5rem .7rem;text-align:left;border-bottom:1px solid #eee;font-variant-numeric:tabular-nums}
th{background:#f4f4f5;font-weight:600}
tr:hover{background:#fafafa}
.fs-yes{color:#b91c1c;font-weight:600}.fs-no{color:#15803d}
.fast{color:#15803d;font-weight:600}.slow{color:#b91c1c;font-weight:600}
.shape-union{color:#7c3aed;font-weight:600}.shape-clean{color:#0369a1}.shape-or-null{color:#a16207}
details{margin:.3rem 0}summary{cursor:pointer;color:#555}
pre{background:#0d1117;color:#c9d1d9;padding:.8rem;border-radius:6px;overflow:auto;font-size:12px}
.note{color:#666;font-size:.9rem}
</style>`)
	fmt.Fprintf(&b, "<h1>listDeployments query-shape benchmark</h1><p class=note>rows=%d, iterations=%d, median wall-clock, real MySQL 8. Hot tenant holds ~40%% of rows.</p>", total, iterations)

	// Group by index config.
	indexes := []string{}
	seen := map[string]bool{}
	for _, r := range results {
		if !seen[r.index] {
			seen[r.index] = true
			indexes = append(indexes, r.index)
		}
	}
	// Fastest per (filter,index) for relative coloring.
	best := map[string]time.Duration{}
	for _, r := range results {
		k := r.index + "|" + r.filter
		if b, ok := best[k]; !ok || r.median < b {
			best[k] = r.median
		}
	}

	for _, idx := range indexes {
		fmt.Fprintf(&b, "<h2>Index config: %s</h2><table><tr><th>Filter</th><th>Shape</th><th>Filesort</th><th>Median</th><th>Query</th><th>Plan</th></tr>", idx)
		for _, r := range results {
			if r.index != idx {
				continue
			}
			fsClass, fsText := "fs-no", "no"
			if r.filesort {
				fsClass, fsText = "fs-yes", "yes"
			}
			speed := ""
			if r.median == best[idx+"|"+r.filter] {
				speed = "fast"
			} else if r.median > 3*best[idx+"|"+r.filter] {
				speed = "slow"
			}
			fmt.Fprintf(&b,
				"<tr><td>%s</td><td class=shape-%s>%s</td><td class=%s>%s</td><td class=%s>%s</td>"+
					"<td><details><summary>sql</summary><pre>%s</pre></details></td>"+
					"<td><details><summary>plan</summary><pre>%s</pre></details></td></tr>",
				r.filter, r.shape, r.shape, fsClass, fsText, speed, r.median.Round(time.Microsecond),
				htmlEscape(formatSQL(r.query)), htmlEscape(r.plan),
			)
		}
		b.WriteString("</table>")
	}

	require.NoError(t, os.WriteFile(path, []byte(b.String()), 0o644))
	t.Logf("HTML report written to %s", path)
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// formatSQL adds line breaks before major clauses for readability.
func formatSQL(q string) string {
	for _, kw := range []string{" WHERE ", " AND ", " ORDER BY ", " UNION ALL ", " LIMIT "} {
		q = strings.ReplaceAll(q, kw, "\n"+strings.TrimSpace(kw)+" ")
	}
	return q
}
