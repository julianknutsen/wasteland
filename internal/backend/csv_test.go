package backend

import (
	"strings"
	"testing"
)

func TestJSONToCSV_SingleRow(t *testing.T) {
	t.Parallel()
	input := `{
		"query_execution_status": "Success",
		"schema_fragment": [
			{"columnName": "id", "columnType": "varchar(20)"},
			{"columnName": "title", "columnType": "varchar(255)"},
			{"columnName": "status", "columnType": "varchar(20)"}
		],
		"rows": [
			{"id": "w-abc123", "title": "Fix bug", "status": "open"}
		]
	}`

	got, err := JSONToCSV([]byte(input))
	if err != nil {
		t.Fatalf("JSONToCSV error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), got)
	}
	if lines[0] != "id,title,status" {
		t.Errorf("header = %q, want %q", lines[0], "id,title,status")
	}
	if lines[1] != "w-abc123,Fix bug,open" {
		t.Errorf("row = %q, want %q", lines[1], "w-abc123,Fix bug,open")
	}
}

func TestJSONToCSV_MultipleRows(t *testing.T) {
	t.Parallel()
	input := `{
		"query_execution_status": "Success",
		"schema_fragment": [
			{"columnName": "id", "columnType": "varchar(20)"},
			{"columnName": "status", "columnType": "varchar(20)"}
		],
		"rows": [
			{"id": "w-001", "status": "open"},
			{"id": "w-002", "status": "claimed"},
			{"id": "w-003", "status": "completed"}
		]
	}`

	got, err := JSONToCSV([]byte(input))
	if err != nil {
		t.Fatalf("JSONToCSV error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %q", len(lines), got)
	}
}

func TestJSONToCSV_EmptyResult(t *testing.T) {
	t.Parallel()
	input := `{
		"query_execution_status": "Success",
		"schema_fragment": [
			{"columnName": "id", "columnType": "varchar(20)"}
		],
		"rows": []
	}`

	got, err := JSONToCSV([]byte(input))
	if err != nil {
		t.Fatalf("JSONToCSV error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line (header only), got %d: %q", len(lines), got)
	}
	if lines[0] != "id" {
		t.Errorf("header = %q, want %q", lines[0], "id")
	}
}

func TestJSONToCSV_NullValues(t *testing.T) {
	t.Parallel()
	input := `{
		"query_execution_status": "Success",
		"schema_fragment": [
			{"columnName": "id", "columnType": "varchar(20)"},
			{"columnName": "description", "columnType": "text"},
			{"columnName": "claimed_by", "columnType": "varchar(100)"}
		],
		"rows": [
			{"id": "w-001", "description": null, "claimed_by": null}
		]
	}`

	got, err := JSONToCSV([]byte(input))
	if err != nil {
		t.Fatalf("JSONToCSV error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), got)
	}
	// NULL values should be empty in CSV.
	if lines[1] != "w-001,," {
		t.Errorf("row = %q, want %q", lines[1], "w-001,,")
	}
}

func TestJSONToCSV_NumericValues(t *testing.T) {
	t.Parallel()
	input := `{
		"query_execution_status": "Success",
		"schema_fragment": [
			{"columnName": "id", "columnType": "varchar(20)"},
			{"columnName": "priority", "columnType": "int"}
		],
		"rows": [
			{"id": "w-001", "priority": 2}
		]
	}`

	got, err := JSONToCSV([]byte(input))
	if err != nil {
		t.Fatalf("JSONToCSV error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[1] != "w-001,2" {
		t.Errorf("row = %q, want %q", lines[1], "w-001,2")
	}
}

func TestJSONToCSV_CommasInValues(t *testing.T) {
	t.Parallel()
	input := `{
		"query_execution_status": "Success",
		"schema_fragment": [
			{"columnName": "id", "columnType": "varchar(20)"},
			{"columnName": "title", "columnType": "varchar(255)"}
		],
		"rows": [
			{"id": "w-001", "title": "Fix bug, fast"}
		]
	}`

	got, err := JSONToCSV([]byte(input))
	if err != nil {
		t.Fatalf("JSONToCSV error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	// Value with comma should be quoted.
	if lines[1] != `w-001,"Fix bug, fast"` {
		t.Errorf("row = %q, want %q", lines[1], `w-001,"Fix bug, fast"`)
	}
}

func TestJSONToCSV_QueryError(t *testing.T) {
	t.Parallel()
	input := `{
		"query_execution_status": "Error",
		"query_execution_message": "table not found: wanted"
	}`

	_, err := JSONToCSV([]byte(input))
	if err == nil {
		t.Fatal("expected error for query error response")
	}
	if !strings.Contains(err.Error(), "table not found") {
		t.Errorf("error = %q, want to contain 'table not found'", err.Error())
	}
}

func TestJSONToCSV_JSONFieldValues(t *testing.T) {
	t.Parallel()
	input := `{
		"query_execution_status": "Success",
		"schema_fragment": [
			{"columnName": "id", "columnType": "varchar(20)"},
			{"columnName": "tags", "columnType": "json"}
		],
		"rows": [
			{"id": "w-001", "tags": ["go","auth"]}
		]
	}`

	got, err := JSONToCSV([]byte(input))
	if err != nil {
		t.Fatalf("JSONToCSV error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	// JSON array should be marshaled and quoted (contains commas).
	if !strings.Contains(lines[1], `"[""go""`) {
		t.Errorf("row = %q, expected JSON array to be quoted CSV field", lines[1])
	}
}
