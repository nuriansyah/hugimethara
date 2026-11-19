package db

import (
	"bufio"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"
)

// Queries are named SQL statements loaded from plain .sql files. Each
// statement starts with a marker comment and runs until the next marker:
//
//	-- name: GetOrganization
//	SELECT id, slug, name FROM organizations WHERE id = $1;
//
// Statements use PostgreSQL positional parameters ($1, $2, …).
type Queries struct {
	stmts map[string]string
}

var nameRe = regexp.MustCompile(`^--\s*name:\s*([A-Za-z][A-Za-z0-9_]*)\s*$`)

// MustLoad parses every file matching patterns in fsys. It panics on
// malformed or duplicate names: query files are part of the binary, so this
// is a programming error caught by any test that imports the package.
func MustLoad(fsys fs.FS, patterns ...string) *Queries {
	q, err := Load(fsys, patterns...)
	if err != nil {
		panic(err)
	}
	return q
}

// Load is MustLoad returning an error.
func Load(fsys fs.FS, patterns ...string) (*Queries, error) {
	q := &Queries{stmts: map[string]string{}}
	for _, pat := range patterns {
		files, err := fs.Glob(fsys, pat)
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			return nil, fmt.Errorf("db: no query files match %q", pat)
		}
		for _, f := range files {
			b, err := fs.ReadFile(fsys, f)
			if err != nil {
				return nil, err
			}
			if err := q.parse(f, string(b)); err != nil {
				return nil, err
			}
		}
	}
	return q, nil
}

func (q *Queries) parse(file, src string) error {
	var name string
	var body strings.Builder
	flush := func() error {
		if name == "" {
			return nil
		}
		stmt := strings.TrimSpace(body.String())
		stmt = strings.TrimSpace(strings.TrimSuffix(stmt, ";"))
		if stmt == "" {
			return fmt.Errorf("db: %s: query %q is empty", file, name)
		}
		if _, dup := q.stmts[name]; dup {
			return fmt.Errorf("db: %s: duplicate query name %q", file, name)
		}
		q.stmts[name] = stmt
		body.Reset()
		return nil
	}
	sc := bufio.NewScanner(strings.NewReader(src))
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if m := nameRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			if err := flush(); err != nil {
				return err
			}
			name = m[1]
			continue
		}
		if name == "" {
			continue // file header comments before the first query
		}
		body.WriteString(line)
		body.WriteByte('\n')
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return flush()
}

// Get returns the named statement. It panics if the name is unknown — every
// call site is exercised by tests, so typos fail fast.
func (q *Queries) Get(name string) string {
	s, ok := q.stmts[name]
	if !ok {
		panic(fmt.Sprintf("db: unknown query %q", name))
	}
	return s
}

// Names lists loaded statement names (sorted), for tests.
func (q *Queries) Names() []string {
	out := make([]string, 0, len(q.stmts))
	for n := range q.stmts {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
