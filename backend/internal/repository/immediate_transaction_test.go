package repository

import "testing"

func TestImmediateTransactionBeginStatement(t *testing.T) {
	if got := immediateTransactionBegin("sqlite"); got != "BEGIN IMMEDIATE" {
		t.Fatalf("sqlite begin = %q", got)
	}
	if got := immediateTransactionBegin("sqlite3"); got != "BEGIN IMMEDIATE" {
		t.Fatalf("sqlite3 begin = %q", got)
	}
	if got := immediateTransactionBegin("postgres"); got != "" {
		t.Fatalf("postgres must use the standard GORM transaction, got %q", got)
	}
}
