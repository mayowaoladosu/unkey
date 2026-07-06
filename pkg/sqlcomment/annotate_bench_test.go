package sqlcomment

import (
	"context"
	"testing"
)

var (
	benchStatic = ForService("frontline", "eu-central-1", "production")
	benchSQLC   = `-- name: FindKeyForVerification :one
SELECT
  k.id,
  k.workspace_id,
  k.hash,
  k.start,
  k.expires,
  k.remaining,
  k.enabled,
  k.deleted
FROM keys AS k
WHERE k.hash = ?
  AND k.deleted = 0`
	benchPlain = `SELECT k.id FROM keys AS k WHERE k.hash = ? AND k.deleted = 0`
)

func BenchmarkAnnotate_disabled(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = Annotate(benchSQLC, Static{}, "ro", Dynamic{Route: "", Source: ""})
	}
}

func BenchmarkAnnotate_sqlcHeader(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = Annotate(benchSQLC, benchStatic, "ro", Dynamic{Route: "", Source: ""})
	}
}

func BenchmarkAnnotate_plainQuery(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = Annotate(benchPlain, benchStatic, "rw", Dynamic{Route: "", Source: ""})
	}
}

func BenchmarkAnnotate_withDynamicTags(b *testing.B) {
	dynamic := Dynamic{Route: "POST /v1/keys.verifyKey", Source: "http"}
	b.ReportAllocs()
	for b.Loop() {
		_ = Annotate(benchSQLC, benchStatic, "ro", dynamic)
	}
}

func BenchmarkAnnotate_parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = Annotate(benchSQLC, benchStatic, "ro", Dynamic{Route: "", Source: ""})
		}
	})
}

func BenchmarkAnnotate_withContext(b *testing.B) {
	ctx := WithDynamic(context.Background(), Dynamic{
		Route:  "POST /v1/keys.verifyKey",
		Source: "http",
	})
	dynamic := DynamicFromContext(ctx)

	b.ReportAllocs()
	for b.Loop() {
		_ = Annotate(benchSQLC, benchStatic, "ro", dynamic)
	}
}

func BenchmarkStripSQLCHeader_fast(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, _ = stripSQLCHeader(benchSQLC)
	}
}

func BenchmarkAnnotate_sqlcHeader_cached(b *testing.B) {
	// Warm the per-query cache so the benchmark measures steady-state cost.
	_ = Annotate(benchSQLC, benchStatic, "ro", Dynamic{Route: "", Source: ""})

	b.ReportAllocs()
	for b.Loop() {
		_ = Annotate(benchSQLC, benchStatic, "ro", Dynamic{Route: "", Source: ""})
	}
}
