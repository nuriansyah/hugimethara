package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/analytics/clickhouse"
	model "github.com/nuriansyah/hugimethara/internal/modules/data_platform/model"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
	"github.com/nuriansyah/hugimethara/internal/platform/sqlguard"
)

type ingestionResult struct {
	read      int64
	written   int64
	watermark string
	drift     []string
}

func (s *DataPlatformService) ingest(ctx context.Context, p principal.Principal, pipeline model.Pipeline, source model.DataSource, secret []byte, before string) (ingestionResult, error) {
	if !s.ch.Enabled() {
		return ingestionResult{}, clickhouse.ErrUnavailable
	}
	rows, err := s.fetchRows(ctx, pipeline, source, secret, before)
	if err != nil {
		return ingestionResult{}, err
	}
	if len(rows) == 0 {
		return ingestionResult{watermark: before}, nil
	}
	schema := inferSchema(rows)
	schema.Objects[0].Name = pipeline.TargetTable
	drift, err := s.ensureRawTable(ctx, p.OrgID, pipeline.TargetTable, schema)
	if err != nil {
		return ingestionResult{}, err
	}
	dbName, err := s.ch.EnsureOrgDatabase(ctx, p.OrgID)
	if err != nil {
		return ingestionResult{}, err
	}
	columns := make([]string, len(schema.Objects[0].Columns))
	columnTypes := make(map[string]string, len(columns))
	for i, col := range schema.Objects[0].Columns {
		columns[i] = col.Name
		columnTypes[col.Name] = col.Type
	}
	prepared := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out := make(map[string]any, len(columns))
		for _, col := range columns {
			out[col] = chValue(columnTypes[col], row[col])
		}
		prepared = append(prepared, out)
	}
	written, err := s.ch.InsertJSONEachRow(ctx, dbName, pipeline.TargetTable, columns, prepared)
	if err != nil {
		return ingestionResult{}, fmt.Errorf("clickhouse: insert into %s: %w", pipeline.TargetTable, err)
	}
	watermark := before
	if pipeline.Mode == "incremental" && pipeline.WatermarkColumn != "" {
		watermark = maxWatermark(rows, pipeline.WatermarkColumn, pipeline.WatermarkType, before)
	}
	return ingestionResult{read: int64(len(rows)), written: int64(written), watermark: watermark, drift: drift}, nil
}

func (s *DataPlatformService) ensureRawTable(ctx context.Context, org uuid.UUID, table string, schema model.Schema) ([]string, error) {
	if err := model.ValidateTable(table); err != nil {
		return nil, err
	}
	if len(schema.Objects) == 0 || len(schema.Objects[0].Columns) == 0 {
		return nil, errors.New("ingestion: no columns discovered")
	}
	dbName, err := s.ch.EnsureOrgDatabase(ctx, org)
	if err != nil {
		return nil, err
	}
	desired := map[string]string{}
	for _, col := range schema.Objects[0].Columns {
		if err := model.ValidateColumn(col.Name); err != nil {
			return nil, err
		}
		desired[col.Name] = clickhouseType(col.Type)
	}
	existing, err := s.rawColumns(ctx, dbName, table)
	if err != nil {
		return nil, err
	}
	var drift []string
	if len(existing) == 0 {
		defs := make([]string, 0, len(desired))
		for _, name := range sortedKeys(desired) {
			defs = append(defs, sqlguard.QuoteIdent(name)+" Nullable("+desired[name]+")")
		}
		stmt := "CREATE TABLE IF NOT EXISTS " + sqlguard.QuoteIdent(dbName) + "." + sqlguard.QuoteIdent(table) +
			" (" + strings.Join(defs, ", ") + ") ENGINE = MergeTree ORDER BY tuple()"
		if err := s.ch.Exec(ctx, "", stmt); err != nil {
			return nil, fmt.Errorf("clickhouse: create raw table: %w", err)
		}
		return nil, nil
	}
	for _, name := range sortedKeys(desired) {
		got, ok := existing[name]
		if !ok {
			stmt := "ALTER TABLE " + sqlguard.QuoteIdent(dbName) + "." + sqlguard.QuoteIdent(table) +
				" ADD COLUMN IF NOT EXISTS " + sqlguard.QuoteIdent(name) + " Nullable(" + desired[name] + ")"
			if err := s.ch.Exec(ctx, "", stmt); err != nil {
				return nil, fmt.Errorf("clickhouse: add raw column: %w", err)
			}
			drift = append(drift, "column added: "+table+"."+name)
			continue
		}
		if !strings.Contains(got, desired[name]) {
			drift = append(drift, fmt.Sprintf("column type changed: %s.%s %s -> %s", table, name, got, desired[name]))
		}
	}
	return drift, nil
}

func (s *DataPlatformService) rawColumns(ctx context.Context, dbName, table string) (map[string]string, error) {
	res, err := s.ch.Query(ctx, dbName, "", "SELECT name, type FROM system.columns WHERE database = "+
		sqlguard.QuoteLiteral(dbName)+" AND table = "+sqlguard.QuoteLiteral(table)+" ORDER BY position",
		clickhouse.Limits{MaxResultRows: 10_000})
	if err != nil {
		return nil, fmt.Errorf("clickhouse: inspect raw table: %w", err)
	}
	out := make(map[string]string, len(res.Rows))
	for _, row := range res.Rows {
		if len(row) < 2 {
			continue
		}
		out[fmt.Sprint(row[0])] = fmt.Sprint(row[1])
	}
	return out, nil
}

func clickhouseType(kind string) string {
	switch kind {
	case "boolean":
		return "UInt8"
	case "number":
		return "Float64"
	case "timestamp":
		return "DateTime64(3)"
	default:
		return "String"
	}
}

func chValue(kind string, value any) any {
	if value == nil {
		return nil
	}
	switch kind {
	case "boolean":
		if b, ok := value.(bool); ok {
			if b {
				return 1
			}
			return 0
		}
		return value
	case "number":
		switch v := value.(type) {
		case int:
			return int64(v)
		case int32:
			return int64(v)
		case int64:
			return v
		case float32:
			return float64(v)
		case float64:
			return v
		case jsonNumber:
			if n, err := strconv.ParseFloat(string(v), 64); err == nil {
				return n
			}
		}
		return value
	case "timestamp":
		switch v := value.(type) {
		case time.Time:
			return v.UTC().Format("2006-01-02 15:04:05.000")
		case string:
			if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
				return t.UTC().Format("2006-01-02 15:04:05.000")
			}
			return v
		}
		return value
	default:
		return fmt.Sprint(value)
	}
}

type jsonNumber string

func maxWatermark(rows []map[string]any, column, kind, before string) string {
	best := before
	for _, row := range rows {
		value := fmt.Sprint(row[column])
		if value == "" || value == "<nil>" {
			continue
		}
		switch kind {
		case "integer":
			a, errA := strconv.ParseInt(value, 10, 64)
			b, errB := strconv.ParseInt(best, 10, 64)
			if errA == nil && (errB != nil || a > b) {
				best = strconv.FormatInt(a, 10)
			}
		case "timestamp":
			a, errA := parseTime(value)
			b, errB := parseTime(best)
			if errA == nil && (errB != nil || a.After(b)) {
				best = a.UTC().Format(time.RFC3339Nano)
			}
		}
	}
	return best
}

func parseTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, errors.New("empty time")
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("invalid time")
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
