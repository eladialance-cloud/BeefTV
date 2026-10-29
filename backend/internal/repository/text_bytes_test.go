package repository

import "testing"

func TestTextByteLengthExprUsesDatabaseDialect(t *testing.T) {
	t.Parallel()

	if got := textByteLengthExpr("postgres", "payload_json"); got != "octet_length(COALESCE(payload_json, ''))" {
		t.Fatalf("postgres expression = %q", got)
	}
	if got := textByteLengthExpr("sqlite", "payload_json"); got != "length(CAST(COALESCE(payload_json, '') AS BLOB))" {
		t.Fatalf("sqlite expression = %q", got)
	}
}
