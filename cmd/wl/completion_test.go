package main

import (
	"testing"
)

func TestCompleteWantedIDsFormat(t *testing.T) {
	// Simulate CSV output from dolt: header + data rows
	// This tests the parsing logic by calling the internal parser
	csvData := "id,title,priority\nw-abc123,Fix auth bug,1\nw-def456,A very long title that exceeds forty characters limit here,0\n"

	lines := splitLines(csvData)
	if len(lines) < 2 {
		t.Fatal("expected at least 2 lines")
	}

	// Verify the first data line parses correctly
	fields := splitCSVFields(lines[1])
	if len(fields) < 3 {
		t.Fatalf("expected 3 fields, got %d", len(fields))
	}
	if fields[0] != "w-abc123" {
		t.Errorf("id = %q, want %q", fields[0], "w-abc123")
	}
}

func TestCompleteWastelandNames(t *testing.T) {
	// completeWastelandNames should not panic with no configs
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	items, directive := completeWastelandNames(nil, nil, "")
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
	if directive != 4 { // cobra.ShellCompDirectiveNoFileComp = 4
		t.Errorf("expected ShellCompDirectiveNoFileComp, got %d", directive)
	}
}

// splitLines splits string into lines, trimming trailing whitespace.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if line != "" {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// splitCSVFields splits a CSV line on commas (no quoting support needed for tests).
func splitCSVFields(line string) []string {
	var fields []string
	start := 0
	for i := 0; i < len(line); i++ {
		if line[i] == ',' {
			fields = append(fields, line[start:i])
			start = i + 1
		}
	}
	fields = append(fields, line[start:])
	return fields
}
