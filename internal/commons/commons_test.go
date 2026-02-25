package commons

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseSimpleCSV_Empty(t *testing.T) {
	t.Parallel()
	got := parseSimpleCSV("")
	if got != nil {
		t.Errorf("parseSimpleCSV(\"\") = %v, want nil", got)
	}
}

func TestParseSimpleCSV_HeaderOnly(t *testing.T) {
	t.Parallel()
	got := parseSimpleCSV("id,title,status\n")
	if got != nil {
		t.Errorf("parseSimpleCSV(header-only) = %v, want nil", got)
	}
}

func TestParseSimpleCSV_SingleRow(t *testing.T) {
	t.Parallel()
	data := "id,title,status\nw-abc,Fix bug,open"
	got := parseSimpleCSV(data)
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1", len(got))
	}
	if got[0]["id"] != "w-abc" {
		t.Errorf("id = %q, want %q", got[0]["id"], "w-abc")
	}
	if got[0]["title"] != "Fix bug" {
		t.Errorf("title = %q, want %q", got[0]["title"], "Fix bug")
	}
	if got[0]["status"] != "open" {
		t.Errorf("status = %q, want %q", got[0]["status"], "open")
	}
}

func TestParseSimpleCSV_MultiRow(t *testing.T) {
	t.Parallel()
	data := "id,title\nw-1,First\nw-2,Second\nw-3,Third"
	got := parseSimpleCSV(data)
	if len(got) != 3 {
		t.Fatalf("got %d rows, want 3", len(got))
	}
	for i, wantID := range []string{"w-1", "w-2", "w-3"} {
		if got[i]["id"] != wantID {
			t.Errorf("row %d id = %q, want %q", i, got[i]["id"], wantID)
		}
	}
}

func TestParseSimpleCSV_MissingFields(t *testing.T) {
	t.Parallel()
	data := "id,title,status\nw-abc,Fix bug"
	got := parseSimpleCSV(data)
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1", len(got))
	}
	if _, ok := got[0]["status"]; ok {
		t.Error("expected missing 'status' field to not be present")
	}
}

func TestParseSimpleCSV_SkipsBlankLines(t *testing.T) {
	t.Parallel()
	data := "id,title\nw-1,First\n\nw-2,Second\n"
	got := parseSimpleCSV(data)
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2", len(got))
	}
}

func TestParseSimpleCSV_TrimsWhitespace(t *testing.T) {
	t.Parallel()
	data := " id , title \n w-abc , Fix bug "
	got := parseSimpleCSV(data)
	if len(got) != 1 {
		t.Fatalf("got %d rows, want 1", len(got))
	}
	if got[0]["id"] != "w-abc" {
		t.Errorf("id = %q, want %q", got[0]["id"], "w-abc")
	}
	if got[0]["title"] != "Fix bug" {
		t.Errorf("title = %q, want %q", got[0]["title"], "Fix bug")
	}
}

func TestEscapeSQL_SingleQuotes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"it's", "it''s"},
		{"", ""},
		{"'; DROP TABLE wanted;--", "''; DROP TABLE wanted;--"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := EscapeSQL(tt.input)
			if got != tt.want {
				t.Errorf("EscapeSQL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEscapeSQL_Backslashes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{`path\to\file`, `path\\to\\file`},
		{`trailing\`, `trailing\\`},
		{`it\'s`, `it\\''s`},
		{`no special`, `no special`},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := EscapeSQL(tt.input)
			if got != tt.want {
				t.Errorf("EscapeSQL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateWantedID_Format(t *testing.T) {
	t.Parallel()
	id := GenerateWantedID("Test Title")
	if !strings.HasPrefix(id, "w-") {
		t.Errorf("GenerateWantedID() = %q, want prefix 'w-'", id)
	}
	// "w-" + 10 hex chars = 12 chars total
	if len(id) != 12 {
		t.Errorf("GenerateWantedID() length = %d, want 12", len(id))
	}
	hexPart := id[2:]
	for _, c := range hexPart {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Errorf("GenerateWantedID() contains non-hex char %q in %q", string(c), id)
		}
	}
}

func TestIsNothingToCommit_MatchingError(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("error: Nothing to commit")
	if !isNothingToCommit(err) {
		t.Error("isNothingToCommit should return true for matching error")
	}
}

func TestIsNothingToCommit_NonMatchingError(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("some other database error")
	if isNothingToCommit(err) {
		t.Error("isNothingToCommit should return false for non-matching error")
	}
}

func TestIsNothingToCommit_Nil(t *testing.T) {
	t.Parallel()
	if isNothingToCommit(nil) {
		t.Error("isNothingToCommit should return false for nil error")
	}
}

func TestGenerateWantedID_Uniqueness(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateWantedID("Same Title")
		if seen[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}

func TestCommitSQL_Unsigned(t *testing.T) {
	t.Parallel()
	got := CommitSQL("wl post: Fix bug", false)
	want := "CALL DOLT_COMMIT('-m', 'wl post: Fix bug');\n"
	if got != want {
		t.Errorf("CommitSQL(unsigned) = %q, want %q", got, want)
	}
}

func TestCommitSQL_Signed(t *testing.T) {
	t.Parallel()
	got := CommitSQL("wl post: Fix bug", true)
	want := "CALL DOLT_COMMIT('-S', '-m', 'wl post: Fix bug');\n"
	if got != want {
		t.Errorf("CommitSQL(signed) = %q, want %q", got, want)
	}
}

func TestCommitSQL_EscapesQuotes(t *testing.T) {
	t.Parallel()
	got := CommitSQL("wl post: it's a test", true)
	if !strings.Contains(got, "it''s a test") {
		t.Errorf("commitSQL did not escape single quotes: %q", got)
	}
	if !strings.Contains(got, "'-S'") {
		t.Errorf("commitSQL missing -S flag: %q", got)
	}
}

func TestFormatTagsJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tags []string
		want string
	}{
		{"empty", nil, "NULL"},
		{"single tag", []string{"go"}, `'["go"]'`},
		{"multiple tags", []string{"go", "auth"}, `'["go","auth"]'`},
		{"single quote", []string{"it's"}, `'["it''s"]'`},
		{"double quote", []string{`say "hello"`}, `'["say \"hello\""]'`},
		{"backslash", []string{`path\to`}, `'["path\\to"]'`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatTagsJSON(tt.tags)
			if got != tt.want {
				t.Errorf("formatTagsJSON(%v) = %s, want %s", tt.tags, got, tt.want)
			}
		})
	}
}

func TestBuildBrowseQuery_MyItems(t *testing.T) {
	t.Parallel()
	f := BrowseFilter{Priority: -1, MyItems: "my-rig"}
	q := BuildBrowseQuery(f)
	if !strings.Contains(q, "(posted_by = 'my-rig' OR claimed_by = 'my-rig')") {
		t.Errorf("MyItems should produce OR clause, got:\n%s", q)
	}
	// MyItems should suppress separate PostedBy/ClaimedBy.
	if strings.Contains(q, "AND posted_by =") || strings.Contains(q, "AND claimed_by =") {
		t.Error("MyItems should suppress separate posted_by/claimed_by conditions")
	}
}

func TestBuildBrowseQuery_MyItems_OverridesPostedClaimedBy(t *testing.T) {
	t.Parallel()
	f := BrowseFilter{Priority: -1, MyItems: "my-rig", PostedBy: "other", ClaimedBy: "other"}
	q := BuildBrowseQuery(f)
	if !strings.Contains(q, "(posted_by = 'my-rig' OR claimed_by = 'my-rig')") {
		t.Errorf("MyItems should take priority, got:\n%s", q)
	}
	if strings.Contains(q, "posted_by = 'other'") {
		t.Error("PostedBy should be ignored when MyItems is set")
	}
}

func TestBuildBrowseQuery_SortPriority(t *testing.T) {
	t.Parallel()
	f := BrowseFilter{Priority: -1, Sort: SortPriority}
	q := BuildBrowseQuery(f)
	if !strings.Contains(q, "ORDER BY priority ASC, created_at DESC") {
		t.Errorf("SortPriority should order by priority, got:\n%s", q)
	}
}

func TestBuildBrowseQuery_SortNewest(t *testing.T) {
	t.Parallel()
	f := BrowseFilter{Priority: -1, Sort: SortNewest}
	q := BuildBrowseQuery(f)
	if !strings.Contains(q, "ORDER BY created_at DESC") {
		t.Errorf("SortNewest should order by created_at DESC, got:\n%s", q)
	}
	if strings.Contains(q, "priority ASC") {
		t.Error("SortNewest should not include priority ordering")
	}
}

func TestBuildBrowseQuery_SortAlpha(t *testing.T) {
	t.Parallel()
	f := BrowseFilter{Priority: -1, Sort: SortAlpha}
	q := BuildBrowseQuery(f)
	if !strings.Contains(q, "ORDER BY title ASC") {
		t.Errorf("SortAlpha should order by title ASC, got:\n%s", q)
	}
}

func TestBuildBrowseQuery_PriorityFilter(t *testing.T) {
	t.Parallel()
	f := BrowseFilter{Priority: 1}
	q := BuildBrowseQuery(f)
	if !strings.Contains(q, "priority = 1") {
		t.Errorf("Priority=1 should filter by priority, got:\n%s", q)
	}
}

func TestValidPriorities(t *testing.T) {
	t.Parallel()
	got := ValidPriorities()
	want := []int{-1, 0, 1, 2, 3, 4}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestPriorityLabel(t *testing.T) {
	t.Parallel()
	if got := PriorityLabel(-1); got != "all" {
		t.Errorf("PriorityLabel(-1) = %q, want %q", got, "all")
	}
	if got := PriorityLabel(0); got != "P0" {
		t.Errorf("PriorityLabel(0) = %q, want %q", got, "P0")
	}
	if got := PriorityLabel(3); got != "P3" {
		t.Errorf("PriorityLabel(3) = %q, want %q", got, "P3")
	}
}

func TestSortLabel(t *testing.T) {
	t.Parallel()
	if got := SortLabel(SortPriority); got != "priority" {
		t.Errorf("SortLabel(SortPriority) = %q", got)
	}
	if got := SortLabel(SortNewest); got != "newest" {
		t.Errorf("SortLabel(SortNewest) = %q", got)
	}
	if got := SortLabel(SortAlpha); got != "alpha" {
		t.Errorf("SortLabel(SortAlpha) = %q", got)
	}
}

func TestFormatTagsJSON_RoundTrip(t *testing.T) {
	t.Parallel()
	tags := []string{"it's", "go", `say "hi"`}
	result := formatTagsJSON(tags)
	// Strip SQL quoting: outer single quotes and unescape ''
	inner := result[1 : len(result)-1]
	inner = strings.ReplaceAll(inner, "''", "'")
	parsed := parseTagsJSON(inner)
	if len(parsed) != len(tags) {
		t.Fatalf("round-trip got %d tags, want %d", len(parsed), len(tags))
	}
	for i, want := range tags {
		if parsed[i] != want {
			t.Errorf("tag[%d] = %q, want %q", i, parsed[i], want)
		}
	}
}
