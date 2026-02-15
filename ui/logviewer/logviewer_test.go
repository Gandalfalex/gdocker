package logviewer

import "testing"

func TestAppendLinesKeepsDuplicateLines(t *testing.T) {
	m := Model{
		Lines: []string{"first"},
	}

	m.AppendLines([]string{"repeat", "repeat", "repeat"})

	if len(m.Lines) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(m.Lines))
	}
	if m.Lines[1] != "repeat" || m.Lines[2] != "repeat" || m.Lines[3] != "repeat" {
		t.Fatalf("expected duplicate lines to be preserved: %#v", m.Lines)
	}
}

func TestAppendLinesRecomputesSearchResults(t *testing.T) {
	m := Model{
		Lines:       []string{"ok"},
		SearchQuery: "error",
	}
	m.performSearch()
	if len(m.SearchResults) != 0 {
		t.Fatalf("expected no initial search results")
	}

	m.AppendLines([]string{"error one", "something", "ERROR two"})

	if len(m.SearchResults) != 2 {
		t.Fatalf("expected 2 search matches, got %d", len(m.SearchResults))
	}
	if m.SearchResults[0] != 1 || m.SearchResults[1] != 3 {
		t.Fatalf("unexpected search result indexes: %#v", m.SearchResults)
	}
}
