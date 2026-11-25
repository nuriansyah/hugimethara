// Package clickhouse is the only component that talks to the analytical store.
// It speaks the ClickHouse HTTP interface so the binaries carry no cgo or
// native driver dependency, and it never exposes raw ClickHouse credentials to
// domain code. Every organization gets one database; callers pass the database
// explicitly and the query service is responsible for choosing the caller's.
package clickhouse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrUnavailable is returned when no analytical store is configured.
var ErrUnavailable = errors.New("clickhouse: analytical store is not configured")

// Config is the process-level ClickHouse connection.
type Config struct {
	URL      string
	User     string
	Password string
	Database string
	Timeout  time.Duration
}

// Limits bound one user query.
type Limits struct {
	MaxExecutionTime time.Duration
	MaxMemoryBytes   int64
	MaxResultRows    int64
}

// Result is one query result.
type Result struct {
	Columns  []string
	Types    []string
	Rows     [][]any
	RowCount int64
	Bytes    int64
}

// Client is safe for concurrent use.
type Client struct {
	base     *url.URL
	user     string
	password string
	database string
	http     *http.Client
	enabled  bool
}

// New builds a client. An empty URL yields a disabled client whose methods
// return ErrUnavailable; the rest of the product keeps working without it.
func New(cfg Config) (*Client, error) {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	c := &Client{user: cfg.User, password: cfg.Password, database: cfg.Database,
		http: &http.Client{Timeout: timeout}}
	if cfg.URL == "" {
		return c, nil
	}
	u, err := url.Parse(cfg.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("clickhouse: invalid URL")
	}
	c.base = u
	c.enabled = true
	if c.user == "" {
		c.user = "default"
	}
	if c.database == "" {
		c.database = "hugimethara"
	}
	return c, nil
}

// Enabled reports whether a store is configured.
func (c *Client) Enabled() bool { return c != nil && c.enabled }

// OrgDatabase returns the database name for org without creating it.
func OrgDatabase(org uuid.UUID) string {
	return "org_" + strings.ReplaceAll(org.String(), "-", "")
}

// EnsureOrgDatabase creates the organization database if needed.
func (c *Client) EnsureOrgDatabase(ctx context.Context, org uuid.UUID) (string, error) {
	if !c.Enabled() {
		return "", ErrUnavailable
	}
	name := OrgDatabase(org)
	if err := c.Exec(ctx, "", `CREATE DATABASE IF NOT EXISTS `+quoteIdent(name)); err != nil {
		return "", err
	}
	return name, nil
}

// Ping verifies the store answers.
func (c *Client) Ping(ctx context.Context) error {
	if !c.Enabled() {
		return ErrUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base.String()+"/ping", nil)
	if err != nil {
		return err
	}
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("clickhouse: ping returned %s", resp.Status)
	}
	return nil
}

// Exec runs a statement that is not a user query (DDL, INSERT).
func (c *Client) Exec(ctx context.Context, database, statement string) error {
	if !c.Enabled() {
		return ErrUnavailable
	}
	params := url.Values{"query": {statement}}
	if database != "" {
		params.Set("database", database)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base.String()+"/?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return readError(resp)
	}
	return nil
}

// Query runs a read-only statement with limits and returns at most MaxResultRows.
func (c *Client) Query(ctx context.Context, database, queryID, statement string, limits Limits) (Result, error) {
	if !c.Enabled() {
		return Result{}, ErrUnavailable
	}
	if limits.MaxExecutionTime <= 0 {
		limits.MaxExecutionTime = 20 * time.Second
	}
	if limits.MaxMemoryBytes <= 0 {
		limits.MaxMemoryBytes = 512 << 20
	}
	if limits.MaxResultRows <= 0 {
		limits.MaxResultRows = 1000
	}
	params := url.Values{
		"query":                {statement},
		"default_format":       {"JSONCompact"},
		"readonly":             {"2"},
		"max_execution_time":   {fmt.Sprintf("%d", int(limits.MaxExecutionTime.Seconds()))},
		"max_memory_usage":     {fmt.Sprintf("%d", limits.MaxMemoryBytes)},
		"max_result_rows":      {fmt.Sprintf("%d", limits.MaxResultRows)},
		"result_overflow_mode": {"break"},
	}
	if database != "" {
		params.Set("database", database)
	}
	if queryID != "" {
		params.Set("query_id", queryID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base.String()+"/?"+params.Encode(), nil)
	if err != nil {
		return Result{}, err
	}
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return Result{}, readError(resp)
	}
	var payload struct {
		Meta []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"meta"`
		Data       [][]any `json:"data"`
		Rows       int64   `json:"rows"`
		Statistics struct {
			BytesRead int64 `json:"bytes_read"`
		} `json:"statistics"`
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 64<<20))
	if err := dec.Decode(&payload); err != nil {
		return Result{}, fmt.Errorf("clickhouse: decode result: %w", err)
	}
	out := Result{RowCount: payload.Rows, Bytes: payload.Statistics.BytesRead, Rows: payload.Data}
	for _, m := range payload.Meta {
		out.Columns = append(out.Columns, m.Name)
		out.Types = append(out.Types, m.Type)
	}
	if out.RowCount == 0 {
		out.RowCount = int64(len(out.Rows))
	}
	return out, nil
}

// InsertJSONEachRow appends rows to table. Columns are the JSON field names;
// missing fields become defaults in ClickHouse.
func (c *Client) InsertJSONEachRow(ctx context.Context, database, table string, columns []string, rows []map[string]any) (int, error) {
	if !c.Enabled() {
		return 0, ErrUnavailable
	}
	if len(rows) == 0 {
		return 0, nil
	}
	var body bytes.Buffer
	enc := json.NewEncoder(&body)
	for _, row := range rows {
		if err := enc.Encode(row); err != nil {
			return 0, err
		}
	}
	query := "INSERT INTO " + quoteIdent(table) + " (" + joinIdents(columns) + ") FORMAT JSONEachRow"
	params := url.Values{"query": {query}, "database": {database}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base.String()+"/?"+params.Encode(), &body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return 0, readError(resp)
	}
	return len(rows), nil
}

// Cancel asks ClickHouse to kill a running query by its id.
func (c *Client) Cancel(ctx context.Context, queryID string) error {
	if !c.Enabled() {
		return ErrUnavailable
	}
	if queryID == "" || strings.ContainsAny(queryID, "'\\;") {
		return errors.New("clickhouse: invalid query id")
	}
	return c.Exec(ctx, "", "KILL QUERY WHERE query_id = '"+queryID+"' SYNC")
}

func (c *Client) auth(req *http.Request) {
	if c.user != "" {
		req.Header.Set("X-ClickHouse-User", c.user)
	}
	if c.password != "" {
		req.Header.Set("X-ClickHouse-Key", c.password)
	}
}

func readError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = resp.Status
	}
	return fmt.Errorf("clickhouse: %s", msg)
}

func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func joinIdents(names []string) string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = quoteIdent(n)
	}
	return strings.Join(out, ", ")
}
