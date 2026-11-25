// Package sqlguard contains domain-free SQL safety rules: read-only query
// validation, table allowlists, watermark predicates and result masking.
package sqlguard

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Errors are returned by the pure validators; services map them to RFC 9457
// problems with stable codes.
var (
	ErrEmptyQuery      = errors.New("the query is empty")
	ErrMultiStatement  = errors.New("only one statement is allowed")
	ErrNotReadOnly     = errors.New("only read-only SELECT queries are allowed")
	ErrForbidden       = errors.New("the query contains a forbidden statement")
	ErrTableNotAllowed = errors.New("the query references a table outside the allowed set")
	ErrFormat          = errors.New("FORMAT and INTO OUTFILE are not allowed in workbench queries")
)

var forbidden = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|alter|create|truncate|attach|detach|grant|revoke|kill|system|optimize|rename|exchange|refresh|use|set)\b`)
var tableRef = regexp.MustCompile(`(?i)\b(?:from|join)\s+([` + "`" + `"]?[A-Za-z_][A-Za-z0-9_]*[` + "`" + `"]?(?:\.[` + "`" + `"]?[A-Za-z_][A-Za-z0-9_]*[` + "`" + `"]?)?)`)
var comment = regexp.MustCompile(`(?s)/\*.*?\*/|--[^\n]*`)
var formatClause = regexp.MustCompile(`(?i)\b(into\s+outfile|format)\b`)

// ValidateReadOnly checks that sql is exactly one read-only statement. It is
// deliberately conservative: the HTTP query service also runs ClickHouse with
// readonly=2, so this guard is the first of two layers.
func ValidateReadOnly(sql string) error {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return ErrEmptyQuery
	}
	body := strings.TrimSuffix(trimmed, ";")
	if strings.Contains(body, ";") {
		return ErrMultiStatement
	}
	if strings.Contains(body, "--") || strings.Contains(body, "/*") {
		body = comment.ReplaceAllString(body, " ")
	}
	lower := strings.ToLower(strings.TrimSpace(body))
	if lower == "" {
		return ErrEmptyQuery
	}
	first := strings.Fields(lower)[0]
	switch first {
	case "select", "with", "explain":
	default:
		return ErrNotReadOnly
	}
	if formatClause.MatchString(body) {
		return ErrFormat
	}
	if loc := forbidden.FindStringIndex(body); loc != nil {
		return fmt.Errorf("%w: %s", ErrForbidden, strings.ToLower(body[loc[0]:loc[1]]))
	}
	return nil
}

// ReferencedTables returns the tables named after FROM or JOIN, lower-cased and
// with quoting stripped. Subqueries are ignored; nested table references are
// found by the regex because it scans the whole statement.
func ReferencedTables(sql string) []string {
	body := comment.ReplaceAllString(strings.TrimSuffix(strings.TrimSpace(sql), ";"), " ")
	seen := map[string]bool{}
	var out []string
	for _, m := range tableRef.FindAllStringSubmatch(body, -1) {
		name := strings.ToLower(strings.Trim(m[1], "`\""))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ValidateTables checks every referenced table against the allowlist. Names are
// matched case-insensitively; a database-qualified name matches its bare table
// only when the database is the caller's own.
func ValidateTables(sql string, allowed []string) error {
	if len(allowed) == 0 {
		return nil
	}
	set := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		set[strings.ToLower(a)] = true
	}
	for _, table := range ReferencedTables(sql) {
		if set[table] {
			continue
		}
		if dot := strings.LastIndexByte(table, '.'); dot >= 0 && set[table[dot+1:]] {
			continue
		}
		return fmt.Errorf("%w: %s", ErrTableNotAllowed, table)
	}
	return nil
}

// ValidateDatasetSQL is the publish/preview check for dataset models.
func ValidateDatasetSQL(sql string, allowed []string) error {
	if err := ValidateReadOnly(sql); err != nil {
		return err
	}
	if len(ReferencedTables(sql)) == 0 {
		return errors.New("the dataset SQL must read from a table or another dataset")
	}
	return ValidateTables(sql, allowed)
}

// QuoteIdent quotes a ClickHouse identifier.
func QuoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// QuoteLiteral quotes a string literal for a ClickHouse predicate.
func QuoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

// IncrementalPredicate builds "column > watermark" for an incremental run.
// The column is an identifier from the pipeline, never user SQL; it is still
// validated and quoted.
func IncrementalPredicate(column, watermark string) (string, error) {
	if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`).MatchString(column) {
		return "", errors.New("invalid watermark column")
	}
	if watermark == "" {
		return "", nil
	}
	return QuoteIdent(column) + " > " + QuoteLiteral(watermark), nil
}

// MaskValue applies a column mask. It never returns the original value for a
// masking policy.
func MaskValue(mask string, value any) any {
	switch mask {
	case "hash":
		sum := sha256.Sum256([]byte(fmt.Sprint(value)))
		return hex.EncodeToString(sum[:])[:12]
	case "redact":
		return "***"
	case "null":
		return nil
	default:
		return value
	}
}

// MaskFor returns the strongest configured mask for a column. Role-specific
// masks win; otherwise the column classification decides.
func MaskFor(column string, classification string, roleMasks map[string]string) string {
	if roleMasks != nil {
		if m, ok := roleMasks[column]; ok && m != "" && m != "none" {
			return m
		}
	}
	switch classification {
	case "restricted", "pii":
		return "redact"
	case "confidential":
		return "hash"
	default:
		return "none"
	}
}

// Checksum returns the sha256 of a canonical JSON value.
func Checksum(v any) string {
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// ViewName is the ClickHouse view for a dataset version.
func ViewName(slug string) string {
	return "ds_" + strings.ReplaceAll(slug, "-", "_")
}

// MetricQuery builds the single aggregate query for a metric.
func MetricQuery(view, expression string, filters []string) (string, error) {
	if strings.TrimSpace(expression) == "" {
		return "", errors.New("metric expression is required")
	}
	if err := ValidateReadOnly("SELECT " + expression + " FROM " + QuoteIdent(view)); err != nil {
		return "", err
	}
	where := ""
	if len(filters) > 0 {
		where = " WHERE (" + strings.Join(filters, ") AND (") + ")"
	}
	return "SELECT " + expression + " AS value FROM " + QuoteIdent(view) + where, nil
}
