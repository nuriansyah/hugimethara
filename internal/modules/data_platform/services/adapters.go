package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/egress"
	model "github.com/nuriansyah/hugimethara/internal/modules/data_platform/model"
	"github.com/nuriansyah/hugimethara/internal/modules/data_platform/repository/queries"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

const maxFetchRows = 50_000

func (s *DataPlatformService) testSource(ctx context.Context, ds model.DataSource, secret []byte) TestResult {
	if !model.Available(ds.Kind) {
		info, _ := model.KindInfo(ds.Kind)
		return TestResult{ErrorCode: "source_kind_unavailable", Message: info.Reason}
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var err error
	switch ds.Kind {
	case "postgresql":
		err = testPostgres(ctx, ds, secret)
	case "rest":
		err = s.testREST(ctx, ds, secret)
	case "file":
		_, err = fileRows(ds.Config, maxFetchRows)
	default:
		err = errors.New("unsupported source kind")
	}
	if err != nil {
		return TestResult{Message: "Connection test failed. Check the host, credentials and network policy.", ErrorCode: "connection_failed"}
	}
	return TestResult{OK: true, Message: "Connection succeeded."}
}

func (s *DataPlatformService) discoverSource(ctx context.Context, ds model.DataSource, secret []byte) (model.Schema, error) {
	if !model.Available(ds.Kind) {
		return model.Schema{}, sourceUnavailable(ds.Kind)
	}
	switch ds.Kind {
	case "postgresql":
		return discoverPostgres(ctx, ds, secret)
	case "rest":
		return s.discoverREST(ctx, ds, secret)
	case "file":
		rows, err := fileRows(ds.Config, 200)
		if err != nil {
			return model.Schema{}, err
		}
		return inferSchema(rows), nil
	default:
		return model.Schema{}, sourceUnavailable(ds.Kind)
	}
}

func (s *DataPlatformService) fetchRows(ctx context.Context, pipeline model.Pipeline, ds model.DataSource, secret []byte, watermark string) ([]map[string]any, error) {
	if !model.Available(ds.Kind) {
		return nil, sourceUnavailable(ds.Kind)
	}
	switch ds.Kind {
	case "postgresql":
		return fetchPostgres(ctx, pipeline, ds, secret, watermark)
	case "rest":
		return s.fetchREST(ctx, pipeline, ds, secret, watermark)
	case "file":
		return fileRows(ds.Config, fetchLimit(pipeline.Config))
	default:
		return nil, sourceUnavailable(ds.Kind)
	}
}

func postgresDSN(ds model.DataSource, secret []byte) (string, error) {
	host, _ := ds.Config["host"].(string)
	database, _ := ds.Config["database"].(string)
	user, _ := ds.Config["user"].(string)
	if strings.TrimSpace(host) == "" || strings.TrimSpace(database) == "" {
		return "", errors.New("postgres: host and database are required")
	}
	if user == "" {
		user = "postgres"
	}
	port := "5432"
	switch p := ds.Config["port"].(type) {
	case float64:
		port = strconv.Itoa(int(p))
	case string:
		if p != "" {
			port = p
		}
	}
	sslmode, _ := ds.Config["sslmode"].(string)
	if sslmode == "" {
		sslmode = "require"
	}
	u := &url.URL{Scheme: "postgres", User: url.UserPassword(user, string(secret)),
		Host: net.JoinHostPort(host, port), Path: "/" + database}
	q := u.Query()
	q.Set("sslmode", sslmode)
	q.Set("connect_timeout", "10")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func openPostgres(ctx context.Context, ds model.DataSource, secret []byte) (*sqlx.DB, error) {
	dsn, err := postgresDSN(ds, secret)
	if err != nil {
		return nil, err
	}
	dbx, err := sqlx.Open("pgx", dsn)
	if err != nil {
		return nil, errors.New("postgres: cannot open connection")
	}
	dbx.SetMaxOpenConns(2)
	dbx.SetMaxIdleConns(1)
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := dbx.PingContext(pingCtx); err != nil {
		_ = dbx.Close()
		return nil, fmt.Errorf("postgres: connection failed: %w", err)
	}
	return dbx, nil
}

func testPostgres(ctx context.Context, ds model.DataSource, secret []byte) error {
	dbx, err := openPostgres(ctx, ds, secret)
	if err != nil {
		return err
	}
	defer func() { _ = dbx.Close() }()
	var one int
	return dbx.GetContext(ctx, &one, "SELECT 1")
}

func pgQuote(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func splitObject(name string) (string, string) {
	parts := strings.Split(strings.TrimSpace(name), ".")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "public", parts[0]
}

func discoverPostgres(ctx context.Context, ds model.DataSource, secret []byte) (model.Schema, error) {
	dbx, err := openPostgres(ctx, ds, secret)
	if err != nil {
		return model.Schema{}, err
	}
	defer func() { _ = dbx.Close() }()
	schemaName, _ := ds.Config["schema"].(string)
	if schemaName == "" {
		schemaName = "public"
	}
	var rows []struct {
		TableName  string `db:"table_name"`
		ColumnName string `db:"column_name"`
		DataType   string `db:"data_type"`
		IsNullable string `db:"is_nullable"`
		TableType  string `db:"table_type"`
	}
	err = dbx.SelectContext(ctx, &rows, `
		SELECT c.table_name, c.column_name, c.data_type, c.is_nullable, t.table_type
		FROM information_schema.columns c
		JOIN information_schema.tables t
		  ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		WHERE c.table_schema = $1
		ORDER BY c.table_name, c.ordinal_position`, schemaName)
	if err != nil {
		return model.Schema{}, fmt.Errorf("postgres: discover: %w", err)
	}
	objects := map[string]*model.SchemaObject{}
	for _, row := range rows {
		obj, ok := objects[row.TableName]
		if !ok {
			kind := "table"
			if strings.Contains(strings.ToUpper(row.TableType), "VIEW") {
				kind = "view"
			}
			obj = &model.SchemaObject{Name: row.TableName, Type: kind}
			objects[row.TableName] = obj
		}
		obj.Columns = append(obj.Columns, model.SchemaColumn{Name: row.ColumnName, Type: row.DataType, Nullable: row.IsNullable == "YES"})
	}
	names := make([]string, 0, len(objects))
	for name := range objects {
		names = append(names, name)
	}
	sort.Strings(names)
	out := model.Schema{Objects: make([]model.SchemaObject, 0, len(names))}
	for _, name := range names {
		out.Objects = append(out.Objects, *objects[name])
	}
	return out, nil
}

func fetchPostgres(ctx context.Context, pipeline model.Pipeline, ds model.DataSource, secret []byte, watermark string) ([]map[string]any, error) {
	dbx, err := openPostgres(ctx, ds, secret)
	if err != nil {
		return nil, err
	}
	defer func() { _ = dbx.Close() }()
	schemaName, tableName := splitObject(pipeline.SourceObject)
	query := "SELECT * FROM " + pgQuote(schemaName) + "." + pgQuote(tableName)
	var args []any
	if pipeline.Mode == "incremental" && watermark != "" {
		pred, err := incrementalPredicate(pipeline.WatermarkColumn, pipeline.WatermarkType, watermark)
		if err != nil {
			return nil, err
		}
		query += " WHERE " + pred.sql
		args = append(args, pred.arg)
	}
	if pipeline.Mode == "incremental" && pipeline.WatermarkColumn != "" {
		query += " ORDER BY " + pgQuote(pipeline.WatermarkColumn)
	}
	query += fmt.Sprintf(" LIMIT %d", fetchLimit(pipeline.Config))
	rows, err := dbx.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: read %s: %w", pipeline.SourceObject, err)
	}
	defer func() { _ = rows.Close() }()
	var out []map[string]any
	for rows.Next() {
		row := map[string]any{}
		if err := rows.MapScan(row); err != nil {
			return nil, fmt.Errorf("postgres: scan: %w", err)
		}
		out = append(out, normalizeRow(row))
		if len(out) >= maxFetchRows {
			break
		}
	}
	return out, rows.Err()
}

type predicate struct {
	sql string
	arg any
}

func incrementalPredicate(column, kind, watermark string) (predicate, error) {
	if err := model.ValidateColumn(column); err != nil {
		return predicate{}, err
	}
	quoted := pgQuote(column)
	switch kind {
	case "integer":
		n, err := strconv.ParseInt(watermark, 10, 64)
		if err != nil {
			return predicate{}, errors.New("incremental watermark is not an integer")
		}
		return predicate{sql: quoted + " > $1", arg: n}, nil
	case "timestamp":
		t, err := time.Parse(time.RFC3339Nano, watermark)
		if err != nil {
			return predicate{}, errors.New("incremental watermark is not a timestamp")
		}
		return predicate{sql: quoted + " > $1", arg: t}, nil
	default:
		return predicate{}, errors.New("unknown watermark type")
	}
}

func fetchLimit(config map[string]any) int {
	n := 1000
	switch v := config["batch_size"].(type) {
	case float64:
		n = int(v)
	case int:
		n = v
	}
	if n <= 0 {
		n = 1000
	}
	if n > maxFetchRows {
		n = maxFetchRows
	}
	return n
}

func (s *DataPlatformService) egressHTTP(ctx context.Context, p principal.Principal, environmentID uuid.UUID) (*http.Client, error) {
	var allowJSON string
	if err := s.tenantTx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &allowJSON, queries.SQL.Get("EnvironmentEgress"), environmentID)
	}); err != nil {
		if db.IsNoRows(err) {
			return nil, errors.New("environment not found")
		}
		return nil, err
	}
	var allow []string
	if allowJSON != "" {
		_ = json.Unmarshal([]byte(allowJSON), &allow)
	}
	policy, err := egress.ParsePolicy(allow)
	if err != nil {
		return nil, fmt.Errorf("egress policy: %w", err)
	}
	return egress.NewClient(egress.Options{Policy: policy, Timeout: 30 * time.Second, MaxResponseBytes: 10 << 20}), nil
}

func restRequest(ctx context.Context, ds model.DataSource, secret []byte, method, path string) (*http.Request, error) {
	base, _ := ds.Config["base_url"].(string)
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return nil, errors.New("rest: invalid base_url")
	}
	if path != "" {
		u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(path, "/")
	}
	if q, ok := ds.Config["query"].(map[string]any); ok && len(q) > 0 {
		values := u.Query()
		for k, v := range q {
			values.Set(k, fmt.Sprint(v))
		}
		u.RawQuery = values.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if headers, ok := ds.Config["headers"].(map[string]any); ok {
		for k, v := range headers {
			req.Header.Set(k, fmt.Sprint(v))
		}
	}
	if len(secret) > 0 {
		var auth struct {
			Header string `json:"header"`
			Value  string `json:"value"`
			Token  string `json:"token"`
		}
		if json.Unmarshal(secret, &auth) == nil && auth.Header != "" {
			req.Header.Set(auth.Header, auth.Value)
		} else if auth.Token != "" {
			req.Header.Set("Authorization", "Bearer "+auth.Token)
		} else {
			req.Header.Set("Authorization", "Bearer "+string(secret))
		}
	}
	return req, nil
}

func (s *DataPlatformService) testREST(ctx context.Context, ds model.DataSource, secret []byte) error {
	p, err := s.principal(ctx)
	if err != nil {
		return err
	}
	client, err := s.egressHTTP(ctx, p, ds.EnvironmentID)
	if err != nil {
		return err
	}
	path, _ := ds.Config["path"].(string)
	method, _ := ds.Config["method"].(string)
	if method == "" {
		method = http.MethodGet
	}
	req, err := restRequest(ctx, ds, secret, method, path)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, egress.LimitBody(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("rest: status %d", resp.StatusCode)
	}
	return nil
}

func (s *DataPlatformService) fetchRESTRecords(ctx context.Context, ds model.DataSource, secret []byte, limit int) ([]map[string]any, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	client, err := s.egressHTTP(ctx, p, ds.EnvironmentID)
	if err != nil {
		return nil, err
	}
	path, _ := ds.Config["path"].(string)
	method, _ := ds.Config["method"].(string)
	if method == "" {
		method = http.MethodGet
	}
	req, err := restRequest(ctx, ds, secret, method, path)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rest: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("rest: status %d", resp.StatusCode)
	}
	var payload any
	dec := json.NewDecoder(egress.LimitBody(resp.Body, 10<<20))
	if err := dec.Decode(&payload); err != nil {
		return nil, fmt.Errorf("rest: invalid JSON: %w", err)
	}
	records := extractRecords(payload, ds.Config)
	out := make([]map[string]any, 0, len(records))
	for _, rec := range records {
		if m, ok := rec.(map[string]any); ok {
			out = append(out, m)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func extractRecords(payload any, config map[string]any) []any {
	path, _ := config["records_path"].(string)
	if path == "" {
		if arr, ok := payload.([]any); ok {
			return arr
		}
		if m, ok := payload.(map[string]any); ok {
			for _, key := range []string{"data", "records", "results", "items"} {
				if arr, ok := m[key].([]any); ok {
					return arr
				}
			}
		}
		return nil
	}
	cur := payload
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[part]
	}
	arr, _ := cur.([]any)
	return arr
}

func (s *DataPlatformService) discoverREST(ctx context.Context, ds model.DataSource, secret []byte) (model.Schema, error) {
	rows, err := s.fetchRESTRecords(ctx, ds, secret, 200)
	if err != nil {
		return model.Schema{}, err
	}
	return inferSchema(rows), nil
}

func (s *DataPlatformService) fetchREST(ctx context.Context, pipeline model.Pipeline, ds model.DataSource, secret []byte, watermark string) ([]map[string]any, error) {
	rows, err := s.fetchRESTRecords(ctx, ds, secret, fetchLimit(pipeline.Config))
	if err != nil {
		return nil, err
	}
	if pipeline.Mode != "incremental" || watermark == "" {
		return rows, nil
	}
	out := rows[:0]
	for _, row := range rows {
		value := fmt.Sprint(row[pipeline.WatermarkColumn])
		if watermarkAfter(value, watermark, pipeline.WatermarkType) {
			out = append(out, row)
		}
	}
	return out, nil
}

func watermarkAfter(value, watermark, kind string) bool {
	switch kind {
	case "integer":
		a, errA := strconv.ParseInt(value, 10, 64)
		b, errB := strconv.ParseInt(watermark, 10, 64)
		return errA == nil && errB == nil && a > b
	case "timestamp":
		a, errA := time.Parse(time.RFC3339Nano, value)
		b, errB := time.Parse(time.RFC3339Nano, watermark)
		return errA == nil && errB == nil && a.After(b)
	default:
		return false
	}
}

func fileRows(config map[string]any, limit int) ([]map[string]any, error) {
	content, _ := config["content"].(string)
	format, _ := config["format"].(string)
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("file: config.content is required")
	}
	switch format {
	case "json":
		var payload any
		if err := json.Unmarshal([]byte(content), &payload); err != nil {
			return nil, fmt.Errorf("file: invalid JSON: %w", err)
		}
		records := extractRecords(payload, config)
		out := make([]map[string]any, 0, len(records))
		for _, rec := range records {
			if m, ok := rec.(map[string]any); ok {
				out = append(out, m)
				if len(out) >= limit {
					break
				}
			}
		}
		return out, nil
	case "csv":
		reader := csv.NewReader(bytes.NewReader([]byte(content)))
		reader.FieldsPerRecord = -1
		records, err := reader.ReadAll()
		if err != nil {
			return nil, fmt.Errorf("file: invalid CSV: %w", err)
		}
		if len(records) < 2 {
			return nil, errors.New("file: CSV needs a header and at least one row")
		}
		header := records[0]
		out := make([]map[string]any, 0, len(records)-1)
		for _, rec := range records[1:] {
			row := make(map[string]any, len(header))
			for i, name := range header {
				if i < len(rec) {
					row[name] = parseScalar(rec[i])
				}
			}
			out = append(out, row)
			if len(out) >= limit {
				break
			}
		}
		return out, nil
	default:
		return nil, errors.New("file: config.format must be csv or json")
	}
}

func parseScalar(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if b, err := strconv.ParseBool(trimmed); err == nil {
		return b
	}
	if n, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return f
	}
	return value
}

func normalizeRow(row map[string]any) map[string]any {
	out := make(map[string]any, len(row))
	for k, v := range row {
		switch value := v.(type) {
		case []byte:
			out[k] = string(value)
		case time.Time:
			out[k] = value.UTC().Format(time.RFC3339Nano)
		default:
			out[k] = value
		}
	}
	return out
}

func inferSchema(rows []map[string]any) model.Schema {
	if len(rows) == 0 {
		return model.Schema{Objects: []model.SchemaObject{}}
	}
	order := []string{}
	types := map[string]string{}
	for _, row := range rows {
		for key, value := range row {
			if _, ok := types[key]; !ok {
				order = append(order, key)
			}
			types[key] = mergeType(types[key], value)
		}
	}
	sort.Strings(order)
	cols := make([]model.SchemaColumn, len(order))
	for i, name := range order {
		cols[i] = model.SchemaColumn{Name: name, Type: types[name], Nullable: true}
	}
	return model.Schema{Objects: []model.SchemaObject{{Name: "records", Type: "table", Columns: cols}}}
}

func mergeType(current string, value any) string {
	if value == nil {
		return current
	}
	next := "string"
	switch value.(type) {
	case bool:
		next = "boolean"
	case int, int32, int64, float32, float64, json.Number:
		next = "number"
	case time.Time:
		next = "timestamp"
	}
	if current == "" {
		return next
	}
	if current == next {
		return current
	}
	if (current == "number" && next == "boolean") || (current == "boolean" && next == "number") {
		return "number"
	}
	return "string"
}
