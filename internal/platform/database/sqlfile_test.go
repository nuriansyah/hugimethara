package db

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestLoadQueries(t *testing.T) {
	fsys := fstest.MapFS{"a.sql": {Data: []byte(`-- header comment, ignored
-- name: First
SELECT 1;

-- name: Second
SELECT *
FROM t
WHERE id = $1; -- trailing comment kept
`)}}
	q, err := Load(fsys, "*.sql")
	require.NoError(t, err)
	require.Equal(t, []string{"First", "Second"}, q.Names())
	require.Equal(t, "SELECT 1", q.Get("First"))
	require.Contains(t, q.Get("Second"), "WHERE id = $1")
	require.Panics(t, func() { q.Get("Missing") })
}

func TestLoadRejectsBadFiles(t *testing.T) {
	_, err := Load(fstest.MapFS{"a.sql": {Data: []byte("-- name: A\nSELECT 1;\n-- name: A\nSELECT 2;")}}, "*.sql")
	require.ErrorContains(t, err, "duplicate")
	_, err = Load(fstest.MapFS{"a.sql": {Data: []byte("-- name: Empty\n\n-- name: B\nSELECT 1;")}}, "*.sql")
	require.ErrorContains(t, err, "empty")
	_, err = Load(fstest.MapFS{}, "*.sql")
	require.ErrorContains(t, err, "no query files")
}
