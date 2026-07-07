package pagination

import (
	"github.com/unkeyed/unkey/pkg/ptr"
	"github.com/unkeyed/unkey/svc/api/openapi"
)

// Identifiable constrains PaginateByID to types that expose their cursor id via
// GetID (not a plain .ID field) because Go generics cannot read a struct field
// off a type parameter. Row types implement GetID in [pkg/db/identifiable.go];
// add new list-query row types there.
type Identifiable interface {
	GetID() string
}

func PaginateByID[T Identifiable](rows []T, limit int) ([]T, *openapi.Pagination) {
	return Paginate(rows, limit, T.GetID)
}

// Paginate trims an over-fetched result set to limit and builds the cursor
// pagination response. Callers must query limit+1 rows so the extra row can
// serve as the next cursor. Pairs with the inclusive `id >= cursor` /
// `ORDER BY id ASC` convention used by the v2 list queries: the returned cursor
// is the first row of the next page.
func Paginate[T any](rows []T, limit int, id func(T) string) ([]T, *openapi.Pagination) {
	hasMore := len(rows) > limit
	var cursor *string
	if hasMore {
		cursor = ptr.P(id(rows[limit]))
		rows = rows[:limit]
	}

	return rows, &openapi.Pagination{
		Cursor:  cursor,
		HasMore: hasMore,
	}
}
