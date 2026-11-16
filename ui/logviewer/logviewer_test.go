package logviewer

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

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

func TestAppendLinesDoesNotMoveCursorWhenScrolledBack(t *testing.T) {
	m := Model{SearchQuery: "error"}
	m.SetLines([]string{"error a", "b", "c"}, "c")
	m.performSearch()
	m.ScrollPos = 1 // user scrolled up

	m.AppendLines([]string{"error d"})
	if m.ScrollPos != 1 {
		t.Fatalf("cursor jumped to %d", m.ScrollPos)
	}
	if len(m.SearchResults) != 2 {
		t.Fatalf("search results not refreshed: %v", m.SearchResults)
	}

	m.ScrollPos = len(m.Lines) - 1 // back at the bottom: keep following
	m.AppendLines([]string{"e"})
	if m.ScrollPos != len(m.Lines)-1 {
		t.Fatalf("cursor should follow new lines, at %d", m.ScrollPos)
	}
}

func TestSetLinesStripsControlSequences(t *testing.T) {
	var m Model
	m.SetLines([]string{"\x1b[31mred\x1b[0m\tdone\r"}, "c")
	if m.Lines[0] != "red    done" {
		t.Fatalf("got %q", m.Lines[0])
	}
}

func TestViewFitsWidth(t *testing.T) {
	m := New(nil, true, true, nil)
	m.SetLines([]string{strings.Repeat("ä", 95), "error here"}, "c")
	m.SearchQuery = "ERR"
	m.performSearch()
	for _, wrap := range []bool{true, false} {
		m.WrapLogs = wrap
		for _, row := range strings.Split(m.View(40, 20), "\n")[3:] {
			if w := lipgloss.Width(row); w > 40 {
				t.Errorf("wrap=%v: row is %d wide: %q", wrap, w, row)
			}
		}
	}
}
