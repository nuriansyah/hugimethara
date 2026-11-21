package pagination

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

const (
	DefaultPageLimit = 50
	MaxPageLimit     = 200
)

// Page is the request side of cursor pagination.
type Page struct {
	Limit  int
	Cursor string // opaque; decode with DecodeCursor
}

// PageResult is the list envelope returned by every collection endpoint.
type PageResult[T any] struct {
	Data       []T    `json:"data"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// ParsePage reads ?limit and ?cursor, applying defaults and bounds.
func ParsePage(r *http.Request) (Page, error) {
	p := Page{Limit: DefaultPageLimit, Cursor: r.URL.Query().Get("cursor")}
	if s := r.URL.Query().Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > MaxPageLimit {
			return Page{}, problem.Validation(problem.FieldError{Field: "limit", Code: "out_of_range",
				Message: "limit must be an integer between 1 and " + strconv.Itoa(MaxPageLimit) + "."})
		}
		p.Limit = n
	}
	return p, nil
}

// EncodeCursor serializes a keyset position (e.g. {created_at, id}) into an opaque cursor.
func EncodeCursor(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// DecodeCursor parses a cursor produced by EncodeCursor into v.
func DecodeCursor(cursor string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || json.Unmarshal(b, v) != nil {
		return problem.Validation(problem.FieldError{Field: "cursor", Code: "invalid", Message: "cursor is malformed."})
	}
	return nil
}

// SortField is one parsed ?sort entry.
type SortField struct {
	Field string
	Desc  bool
}

// ParseSort parses ?sort=-created_at,name against an allowlist of sortable fields.
func ParseSort(r *http.Request, allowed ...string) ([]SortField, error) {
	raw := r.URL.Query().Get("sort")
	if raw == "" {
		return nil, nil
	}
	ok := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		ok[a] = true
	}
	var out []SortField
	for _, part := range strings.Split(raw, ",") {
		f := SortField{Field: strings.TrimPrefix(part, "-"), Desc: strings.HasPrefix(part, "-")}
		if !ok[f.Field] {
			return nil, problem.Validation(problem.FieldError{Field: "sort", Code: "unsupported",
				Message: "cannot sort by " + strconv.Quote(f.Field) + "."})
		}
		out = append(out, f)
	}
	return out, nil
}
