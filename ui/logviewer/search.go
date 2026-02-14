package logviewer

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// performSearch finds all lines matching the search query.
func (m *Model) performSearch() {
	if m.SearchQuery == "" {
		m.SearchResults = nil
		m.SearchResultIdx = 0
		return
	}

	var results []int
	query := strings.ToLower(m.SearchQuery)
	for i, line := range m.Lines {
		if strings.Contains(strings.ToLower(line), query) {
			results = append(results, i)
		}
	}

	m.SearchResults = results
	m.SearchResultIdx = 0

	if len(results) > 0 {
		m.ScrollPos = results[0]
	}
}

func (m *Model) formatSearchStatus() string {
	if len(m.SearchResults) == 0 {
		return "No matches"
	}
	return fmt.Sprintf("Match %d/%d", m.SearchResultIdx+1, len(m.SearchResults))
}

// highlightSearchTerm highlights all occurrences of query in line.
func highlightSearchTerm(line, query string) string {
	if query == "" {
		return line
	}

	lowerLine := strings.ToLower(line)
	lowerQuery := strings.ToLower(query)

	idx := strings.Index(lowerLine, lowerQuery)
	if idx == -1 {
		return line
	}

	var result strings.Builder
	lastIdx := 0

	for idx != -1 {
		result.WriteString(line[lastIdx:idx])

		match := line[idx : idx+len(query)]
		highlighted := lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorSearchHighlightFG)).
			Background(lipgloss.Color(colorSearchHighlightBG)).
			Render(match)
		result.WriteString(highlighted)

		lastIdx = idx + len(query)
		idx = strings.Index(lowerLine[lastIdx:], lowerQuery)
		if idx != -1 {
			idx += lastIdx
		}
	}

	result.WriteString(line[lastIdx:])
	return result.String()
}
