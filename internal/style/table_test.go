package style

import (
	"strings"
	"testing"
)

func TestNewTable_Empty(t *testing.T) {
	t.Parallel()
	tbl := NewTable()
	got := tbl.Render()
	if got != "" {
		t.Errorf("NewTable().Render() = %q, want empty string", got)
	}
}

func TestNewTable_HeaderOnly(t *testing.T) {
	t.Parallel()
	tbl := NewTable(
		Column{Name: "ID", Width: 10},
		Column{Name: "Title", Width: 20},
	)
	got := tbl.Render()
	if got == "" {
		t.Fatal("expected non-empty output for header-only table")
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	// Should have header + separator, no data rows
	if len(lines) != 2 {
		t.Errorf("got %d lines, want 2 (header + separator)", len(lines))
	}
}

func TestNewTable_BasicRender(t *testing.T) {
	t.Parallel()
	tbl := NewTable(
		Column{Name: "ID", Width: 10},
		Column{Name: "Title", Width: 20},
	)
	tbl.AddRow("w-abc", "Fix bug")
	tbl.AddRow("w-def", "Add feature")

	got := tbl.Render()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	// header + separator + 2 rows = 4 lines
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4", len(lines))
	}

	// Check row content (strip ANSI for comparison)
	row1 := stripAnsi(lines[2])
	if !strings.Contains(row1, "w-abc") || !strings.Contains(row1, "Fix bug") {
		t.Errorf("row 1 = %q, want to contain w-abc and Fix bug", row1)
	}
	row2 := stripAnsi(lines[3])
	if !strings.Contains(row2, "w-def") || !strings.Contains(row2, "Add feature") {
		t.Errorf("row 2 = %q, want to contain w-def and Add feature", row2)
	}
}

func TestAddRow_PadsShortRows(t *testing.T) {
	t.Parallel()
	tbl := NewTable(
		Column{Name: "A", Width: 5},
		Column{Name: "B", Width: 5},
		Column{Name: "C", Width: 5},
	)
	tbl.AddRow("x") // only 1 value for 3 columns

	got := tbl.Render()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	// header + separator + 1 row = 3 lines
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	// Should not panic and should render successfully
	row := stripAnsi(lines[2])
	if !strings.Contains(row, "x") {
		t.Errorf("row = %q, want to contain 'x'", row)
	}
}

func TestRender_Truncation(t *testing.T) {
	t.Parallel()
	tbl := NewTable(
		Column{Name: "Val", Width: 8},
	)
	tbl.AddRow("abcdefghijklmnop") // 16 chars, width is 8

	got := tbl.Render()
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	row := stripAnsi(lines[len(lines)-1])
	if !strings.Contains(row, "...") {
		t.Errorf("truncated row = %q, want to contain '...'", row)
	}
	// The truncated value should be width chars: 5 chars + "..." = 8
	// Find the actual value in the row (after indent)
	trimmed := strings.TrimSpace(row)
	if len(trimmed) != 8 {
		t.Errorf("truncated value length = %d, want 8", len(trimmed))
	}
}

func TestSetIndent(t *testing.T) {
	t.Parallel()
	tbl := NewTable(
		Column{Name: "Col", Width: 5},
	)
	tbl.SetIndent(">>>>")
	tbl.AddRow("hi")

	got := tbl.Render()
	for _, line := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
		if !strings.HasPrefix(line, ">>>>") {
			t.Errorf("line %q does not start with custom indent '>>>>'", line)
		}
	}
}

func TestSetHeaderSeparator_Disabled(t *testing.T) {
	t.Parallel()
	tbl := NewTable(
		Column{Name: "Col", Width: 10},
	)
	tbl.SetHeaderSeparator(false)
	tbl.AddRow("val")

	got := tbl.Render()
	if strings.Contains(got, "─") {
		t.Errorf("got separator line when disabled: %q", got)
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	// header + 1 row = 2 lines (no separator)
	if len(lines) != 2 {
		t.Errorf("got %d lines, want 2 (no separator)", len(lines))
	}
}

func TestPad_AlignLeft(t *testing.T) {
	t.Parallel()
	tbl := NewTable()
	got := tbl.pad("hi", "hi", 6, AlignLeft)
	if got != "hi    " {
		t.Errorf("pad(AlignLeft) = %q, want %q", got, "hi    ")
	}
}

func TestPad_AlignRight(t *testing.T) {
	t.Parallel()
	tbl := NewTable()
	got := tbl.pad("hi", "hi", 6, AlignRight)
	if got != "    hi" {
		t.Errorf("pad(AlignRight) = %q, want %q", got, "    hi")
	}
}

func TestPad_AlignCenter(t *testing.T) {
	t.Parallel()
	tbl := NewTable()
	got := tbl.pad("hi", "hi", 6, AlignCenter)
	if got != "  hi  " {
		t.Errorf("pad(AlignCenter) = %q, want %q", got, "  hi  ")
	}
}

func TestPad_ExactWidth(t *testing.T) {
	t.Parallel()
	tbl := NewTable()
	got := tbl.pad("hello", "hello", 5, AlignLeft)
	if got != "hello" {
		t.Errorf("pad(exact width) = %q, want %q", got, "hello")
	}
}

func TestStripAnsi(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain", "hello", "hello"},
		{"bold", "\x1b[1mhello\x1b[0m", "hello"},
		{"color", "\x1b[31mred\x1b[0m", "red"},
		{"multiple", "\x1b[1m\x1b[31mtext\x1b[0m", "text"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := stripAnsi(tt.input)
			if got != tt.want {
				t.Errorf("stripAnsi(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
