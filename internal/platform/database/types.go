package db

import "github.com/jackc/pgx/v5/pgtype"

// TextArray scans a PostgreSQL text[] column. database/sql has no array
// support, and the pgx stdlib driver hands arrays to Scan in text form, so a
// plain []string destination fails at runtime. NULL scans to nil.
type TextArray []string

// Scan implements sql.Scanner.
func (a *TextArray) Scan(src any) error {
	if src == nil {
		*a = nil
		return nil
	}
	var out []string
	if err := pgtype.NewMap().SQLScanner(&out).Scan(src); err != nil {
		return err
	}
	*a = out
	return nil
}
