package logviewer

import (
	"fmt"
	"strings"
)

// findMatches returns the indices of all lines containing the search query.
func (m *Model) findMatches() []int {
	if m.SearchQuery == "" {
		return nil
	}
	var results []int
	query := strings.ToLower(m.SearchQuery)
	for i, line := range m.Lines {
		if strings.Contains(strings.ToLower(line), query) {
			results = append(results, i)
		}
	}
	return results
}

// performSearch runs the search and jumps to the most recent match at or
// above the cursor (logs are read from the bottom up), else the first below.
func (m *Model) performSearch() {
	m.SearchResults = m.findMatches()
	m.SearchResultIdx = 0
	if len(m.SearchResults) == 0 {
		return
	}
	for i, line := range m.SearchResults {
		if line > m.ScrollPos {
			break
		}
		m.SearchResultIdx = i
	}
	m.ScrollPos = m.SearchResults[m.SearchResultIdx]
}

func (m *Model) formatSearchStatus() string {
	if len(m.SearchResults) == 0 {
		return "No matches"
	}
	return fmt.Sprintf("Match %d/%d", m.SearchResultIdx+1, len(m.SearchResults))
}
