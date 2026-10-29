package model

import "testing"

func TestManjuRuleAuditModelContracts(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"rule snapshot", (ManjuRuleSnapshot{}).TableName(), "manju_rule_snapshots"},
		{"rule read receipt", (ManjuRuleReadReceipt{}).TableName(), "manju_rule_read_receipts"},
		{"model exchange", (ManjuModelExchange{}).TableName(), "manju_model_exchanges"},
		{"artifact ledger", (ManjuArtifactLedger{}).TableName(), "manju_artifact_ledger"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Fatalf("%s table = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
	if ManjuArtifactVisibilityUser == ManjuArtifactVisibilityInternal ||
		ManjuArtifactVisibilityInternal == ManjuArtifactVisibilityAudit {
		t.Fatal("artifact visibility values must be distinct")
	}
}
