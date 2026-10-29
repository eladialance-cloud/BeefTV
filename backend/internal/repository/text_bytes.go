package repository

import (
	"fmt"
	"strings"
)

// textByteLengthExpr returns a byte length expression understood by the active
// database. SQLite needs the BLOB cast; PostgreSQL has no BLOB type and uses
// octet_length for the same UTF-8 byte count.
func textByteLengthExpr(dialect string, column string) string {
	column = strings.TrimSpace(column)
	if strings.EqualFold(strings.TrimSpace(dialect), "postgres") {
		return fmt.Sprintf("octet_length(COALESCE(%s, ''))", column)
	}
	return fmt.Sprintf("length(CAST(COALESCE(%s, '') AS BLOB))", column)
}

func sumTextBytesExpr(dialect string, columns ...string) string {
	parts := make([]string, 0, len(columns))
	for _, column := range columns {
		parts = append(parts, textByteLengthExpr(dialect, column))
	}
	return "COALESCE(SUM(" + strings.Join(parts, " + ") + "), 0)"
}
