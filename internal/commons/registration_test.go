package commons

import (
	"strings"
	"testing"
)

func TestBuildRegistrationSQL(t *testing.T) {
	sql := BuildRegistrationSQL("alice", "alice-org", "Alice Dev", "alice@example.com", "0.1.0")

	if !strings.Contains(sql, "INSERT INTO rigs") {
		t.Error("expected INSERT INTO rigs")
	}
	if !strings.Contains(sql, "ON DUPLICATE KEY UPDATE") {
		t.Error("expected ON DUPLICATE KEY UPDATE")
	}
	if !strings.Contains(sql, "alice-org") {
		t.Error("expected alice-org in SQL")
	}
	if !strings.Contains(sql, "hop://alice@example.com/alice/") {
		t.Error("expected hop URI in SQL")
	}
}

func TestBuildRegistrationSQL_Escaping(t *testing.T) {
	sql := BuildRegistrationSQL("bob's-rig", "org", "Bob O'Brien", "bob@test.com", "dev")

	// Single quotes should be escaped (doubled).
	if strings.Contains(sql, "bob's-rig") {
		t.Error("expected single quotes to be escaped in handle")
	}
	if strings.Contains(sql, "O'Brien") {
		t.Error("expected single quotes to be escaped in display name")
	}
}
