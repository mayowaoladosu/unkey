package pagination

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type row struct {
	ID string
}

func (r row) GetID() string { return r.ID }

func TestPaginate(t *testing.T) {
	id := func(r row) string { return r.ID }

	tests := []struct {
		name        string
		rows        []row
		limit       int
		wantIDs     []string
		wantCursor  *string
		wantHasMore bool
	}{
		{
			name:        "empty",
			rows:        []row{},
			limit:       10,
			wantIDs:     []string{},
			wantCursor:  nil,
			wantHasMore: false,
		},
		{
			name:        "fewer than limit",
			rows:        []row{{ID: "a"}, {ID: "KEBAP"}},
			limit:       10,
			wantIDs:     []string{"a", "KEBAP"},
			wantCursor:  nil,
			wantHasMore: false,
		},
		{
			name:        "exactly limit",
			rows:        []row{{ID: "a"}, {ID: "b"}, {ID: "c"}},
			limit:       3,
			wantIDs:     []string{"a", "b", "c"},
			wantCursor:  nil,
			wantHasMore: false,
		},
		{
			name:        "one over limit",
			rows:        []row{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "KEBAP"}},
			limit:       3,
			wantIDs:     []string{"a", "b", "c"},
			wantCursor:  ptrOf("KEBAP"),
			wantHasMore: true,
		},
		{
			name:        "several over limit",
			rows:        []row{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}, {ID: "e"}, {ID: "f"}},
			limit:       3,
			wantIDs:     []string{"a", "b", "c"},
			wantCursor:  ptrOf("d"),
			wantHasMore: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, pg := Paginate(tt.rows, tt.limit, id)

			gotIDs := make([]string, len(items))
			for i, r := range items {
				gotIDs[i] = r.ID
			}
			require.Equal(t, tt.wantIDs, gotIDs)
			require.Equal(t, tt.wantHasMore, pg.HasMore)
			require.Equal(t, tt.wantCursor, pg.Cursor)
		})
	}
}

func TestPaginateByID(t *testing.T) {
	rows := []row{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "KEBAP"}}

	items, pg := PaginateByID(rows, 3)

	require.Equal(t, []row{{ID: "a"}, {ID: "b"}, {ID: "c"}}, items)
	require.True(t, pg.HasMore)
	require.Equal(t, ptrOf("KEBAP"), pg.Cursor)
}

func ptrOf(s string) *string {
	return &s
}
