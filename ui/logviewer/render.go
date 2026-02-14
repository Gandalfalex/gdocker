package logviewer

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Colors duplicated from ui/icons.go to avoid import cycle.
const (
	colorMuted             = "#858585"
	colorTitle             = "#0e639c"
	colorHighlight         = "#094771"
	colorLogFG             = "#d4d4d4"
	colorSearchHighlightFG = "#000000"
	colorSearchHighlightBG = "#ffff00"
	colorVisualSelectionBG = "#264f78"
	colorCursorBG          = "#c6c6c6"
	colorCursorFG          = "#000000"
)

// View renders the log viewer pane.
func (m *Model) View(width, height int) string {
	var s strings.Builder

	containerName := m.ContainerName
	if containerName == "" {
		containerName = "container"
	}

	mode := "paused"
	if m.FollowingLogs {
		mode = "following"
	}
	wrapLabel := "wrap"
	if !m.WrapLogs {
		wrapLabel = "nowrap"
	}

	subtitle := fmt.Sprintf("%s • %d lines • %s • %s", containerName, len(m.Lines), mode, wrapLabel)
	switch m.VisualType {
	case VisualLine:
		subtitle = "-- VISUAL LINE -- • " + subtitle
	case VisualChar:
		subtitle = "-- VISUAL -- • " + subtitle
	}
	s.WriteString(renderPaneHeader("Logs", subtitle))

	if len(m.Lines) == 0 {
		s.WriteString("No logs available")
		return s.String()
	}

	if m.SearchQuery != "" && !m.SearchMode {
		searchStatus := fmt.Sprintf("Search: %q (%d matches)", m.SearchQuery, len(m.SearchResults))
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render(searchStatus) + "\n\n")
	}

	// Calculate max line width
	gutterWidth := 0
	if m.ShowLineNumbers {
		gutterWidth = 5
	}
	maxLineWidth := width - 4 - gutterWidth
	if maxLineWidth < 12 {
		maxLineWidth = 12
	}

	maxVisibleRows := max(height-7, 1)

	scrollPos := m.ScrollPos
	if scrollPos >= len(m.Lines) {
		scrollPos = len(m.Lines) - 1
	}
	if scrollPos < 0 {
		scrollPos = 0
	}

	// Search result lookup
	searchResultMap := make(map[int]bool)
	for _, idx := range m.SearchResults {
		searchResultMap[idx] = true
	}

	// Visual selection range
	visualLineStart, visualLineEnd := -1, -1
	if m.VisualType == VisualLine {
		visualLineStart, visualLineEnd = m.lineSelectionRange()
	}

	// Char visual range
	charSL, charSC, charEL, charEC := -1, -1, -1, -1
	if m.VisualType == VisualChar {
		charSL, charSC, charEL, charEC = m.charSelectionRange()
	}

	if m.WrapLogs {
		s.WriteString(m.renderWrapped(maxLineWidth, maxVisibleRows, scrollPos, searchResultMap, visualLineStart, visualLineEnd, charSL, charSC, charEL, charEC))
	} else {
		s.WriteString(m.renderTruncated(maxLineWidth, maxVisibleRows, scrollPos, searchResultMap, visualLineStart, visualLineEnd, charSL, charSC, charEL, charEC))
	}

	return s.String()
}

func (m *Model) renderWrapped(maxLineWidth, maxVisibleRows, scrollPos int, searchResultMap map[int]bool, vlStart, vlEnd, cSL, cSC, cEL, cEC int) string {
	var s strings.Builder

	type renderedLine struct {
		logIdx int
		rows   []string
	}

	// Render downward from scrollPos
	var downLines []renderedLine
	downRows := 0
	for i := scrollPos; i < len(m.Lines) && downRows < maxVisibleRows; i++ {
		wrapped := wrapLine(m.Lines[i], maxLineWidth)
		downLines = append(downLines, renderedLine{logIdx: i, rows: wrapped})
		downRows += len(wrapped)
	}

	// Fill remaining rows upward
	remainingRows := maxVisibleRows - downRows
	var upLines []renderedLine
	for i := scrollPos - 1; i >= 0 && remainingRows > 0; i-- {
		wrapped := wrapLine(m.Lines[i], maxLineWidth)
		upLines = append(upLines, renderedLine{logIdx: i, rows: wrapped})
		remainingRows -= len(wrapped)
	}

	// Combine
	allLines := make([]renderedLine, 0, len(upLines)+len(downLines))
	for i := len(upLines) - 1; i >= 0; i-- {
		allLines = append(allLines, upLines[i])
	}
	allLines = append(allLines, downLines...)

	rowsWritten := 0
	startIdx, endIdx := -1, -1
	for _, rl := range allLines {
		if rowsWritten >= maxVisibleRows {
			break
		}
		if startIdx == -1 {
			startIdx = rl.logIdx
		}
		endIdx = rl.logIdx

		isCurrentLine := rl.logIdx == scrollPos
		isSearchMatch := m.SearchQuery != "" && searchResultMap[rl.logIdx]
		isLineSelected := vlStart != -1 && rl.logIdx >= vlStart && rl.logIdx <= vlEnd

		for j, row := range rl.rows {
			if rowsWritten >= maxVisibleRows {
				break
			}

			gutter := m.renderGutter(rl.logIdx, j)
			displayRow := row

			if isSearchMatch && j == 0 {
				displayRow = highlightSearchTerm(displayRow, m.SearchQuery)
			}

			// Apply character-level highlighting if in char visual mode.
			inCharVisualRange := m.VisualType == VisualChar && rl.logIdx >= cSL && rl.logIdx <= cEL
			if inCharVisualRange {
				lineRunes := []rune(m.Lines[rl.logIdx])
				lineLen := len(lineRunes)
				if lineLen > 0 {
					// Selection range on this logical log line.
					selStart := 0
					selEnd := lineLen - 1
					if rl.logIdx == cSL {
						selStart = cSC
					}
					if rl.logIdx == cEL {
						selEnd = cEC
					}
					if selStart < 0 {
						selStart = 0
					}
					if selEnd >= lineLen {
						selEnd = lineLen - 1
					}

					// Current wrapped row range inside the logical line.
					rowStart := j * maxLineWidth
					rowEnd := rowStart + len([]rune(row)) - 1

					// Intersect line selection with this wrapped row.
					hlStart := max(selStart, rowStart)
					hlEnd := min(selEnd, rowEnd)
					if hlStart <= hlEnd {
						localStart := hlStart - rowStart
						localEnd := hlEnd - rowStart
						displayRow = applyCharHighlightRange(displayRow, localStart, localEnd)
					}
				}
				displayRow = gutter + lipgloss.NewStyle().
					Foreground(lipgloss.Color(colorLogFG)).
					Render(displayRow)
			} else if isLineSelected {
				// Apply line-level styling for visual line mode
				displayRow = lipgloss.NewStyle().
					Foreground(lipgloss.Color(colorLogFG)).
					Background(lipgloss.Color(colorVisualSelectionBG)).
					Render(gutter + lipgloss.NewStyle().
						Foreground(lipgloss.Color(colorLogFG)).
						Render(displayRow))
			} else if isCurrentLine {
				displayRow = lipgloss.NewStyle().
					Foreground(lipgloss.Color(colorLogFG)).
					Background(lipgloss.Color(colorHighlight)).
					Render(gutter + lipgloss.NewStyle().
						Foreground(lipgloss.Color(colorLogFG)).
						Render(displayRow))
			} else {
				displayRow = gutter + lipgloss.NewStyle().
					Foreground(lipgloss.Color(colorLogFG)).
					Render(displayRow)
			}
			s.WriteString(displayRow + "\n")
			rowsWritten++
		}
	}

	if startIdx == -1 {
		startIdx = 0
	}
	if endIdx == -1 {
		endIdx = 0
	}

	s.WriteString("\n")
	progress := int(float64(scrollPos+1) / float64(len(m.Lines)) * 100)
	indicator := fmt.Sprintf("Line %d/%d (%d-%d visible, %d%%)", scrollPos+1, len(m.Lines), startIdx+1, endIdx+1, progress)
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render(indicator))
	return s.String()
}

func (m *Model) renderTruncated(maxLineWidth, maxVisibleRows, scrollPos int, searchResultMap map[int]bool, vlStart, vlEnd, cSL, cSC, cEL, cEC int) string {
	var s strings.Builder

	start := max(scrollPos-maxVisibleRows/2, 0)
	end := start + maxVisibleRows
	if end > len(m.Lines) {
		end = len(m.Lines)
		start = max(end-maxVisibleRows, 0)
	}

	for i := start; i < end; i++ {
		line := m.Lines[i]
		gutter := m.renderGutter(i, 0)

		if len(line) > maxLineWidth {
			line = line[:maxLineWidth-3] + "..."
		}

		if m.SearchQuery != "" && searchResultMap[i] {
			line = highlightSearchTerm(line, m.SearchQuery)
		}

		// Apply character-level highlighting if in char visual mode
		inCharVisualRange := m.VisualType == VisualChar && i >= cSL && i <= cEL
		if inCharVisualRange {
			line = m.applyCharHighlight(line, i, cSL, cSC, cEL, cEC)
			line = gutter + lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorLogFG)).
				Render(line)
		} else if vlStart != -1 && i >= vlStart && i <= vlEnd {
			// Apply line-level styling for visual line mode
			line = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorLogFG)).
				Background(lipgloss.Color(colorVisualSelectionBG)).
				Render(gutter + lipgloss.NewStyle().
					Foreground(lipgloss.Color(colorLogFG)).
					Render(line))
		} else if i == scrollPos {
			line = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorLogFG)).
				Background(lipgloss.Color(colorHighlight)).
				Render(gutter + lipgloss.NewStyle().
					Foreground(lipgloss.Color(colorLogFG)).
					Render(line))
		} else {
			line = gutter + lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorLogFG)).
				Render(line)
		}
		s.WriteString(line + "\n")
	}

	s.WriteString("\n")
	progress := int(float64(scrollPos+1) / float64(len(m.Lines)) * 100)
	indicator := fmt.Sprintf("Line %d/%d (%d-%d visible, %d%%)", scrollPos+1, len(m.Lines), start+1, end, progress)
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render(indicator))
	return s.String()
}

func (m *Model) renderGutter(lineIdx, wrapIdx int) string {
	if !m.ShowLineNumbers {
		return ""
	}
	if wrapIdx == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted)).
			Render(fmt.Sprintf("%4d ", lineIdx+1))
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorMuted)).
		Render("   · ")
}

// applyCharHighlight highlights the character-visual selection range within a line.
func (m *Model) applyCharHighlight(line string, lineIdx, startLine, startCol, endLine, endCol int) string {
	runes := []rune(line)
	n := len(runes)
	if n == 0 {
		return line
	}

	// Determine the column range to highlight on this line
	colStart := 0
	colEnd := n - 1
	if lineIdx == startLine {
		colStart = startCol
	}
	if lineIdx == endLine {
		colEnd = endCol
	}
	if colStart >= n {
		colStart = n - 1
	}
	if colEnd >= n {
		colEnd = n - 1
	}
	if colStart < 0 {
		colStart = 0
	}

	return applyCharHighlightRange(line, colStart, colEnd)
}

func applyCharHighlightRange(line string, colStart, colEnd int) string {
	runes := []rune(line)
	n := len(runes)
	if n == 0 {
		return line
	}
	if colStart < 0 {
		colStart = 0
	}
	if colEnd >= n {
		colEnd = n - 1
	}
	if colStart > colEnd {
		return line
	}

	before := string(runes[:colStart])
	selected := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorLogFG)).
		Background(lipgloss.Color(colorVisualSelectionBG)).
		Render(string(runes[colStart : colEnd+1]))
	after := ""
	if colEnd+1 < n {
		after = string(runes[colEnd+1:])
	}
	return before + selected + after
}

func renderPaneHeader(title, subtitle string) string {
	titleStyled := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(colorTitle)).
		Render(title)

	if subtitle == "" {
		return titleStyled + "\n\n"
	}

	subtitleStyled := lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorMuted)).
		Render(subtitle)

	return titleStyled + "\n" + subtitleStyled + "\n\n"
}

// wrapLine splits a string into chunks of at most maxWidth characters.
func wrapLine(line string, maxWidth int) []string {
	if maxWidth < 1 {
		maxWidth = 1
	}
	if len(line) <= maxWidth {
		return []string{line}
	}
	var parts []string
	for len(line) > maxWidth {
		parts = append(parts, line[:maxWidth])
		line = line[maxWidth:]
	}
	if len(line) > 0 {
		parts = append(parts, line)
	}
	return parts
}

// StatusBarHint returns context-appropriate key hints for the status bar.
func (m *Model) StatusBarHint() string {
	if m.SearchMode {
		return "?" + m.SearchQuery + "█ • enter: search • esc: cancel"
	}

	switch m.VisualType {
	case VisualLine:
		return "j/k: extend selection • g/G: top/bottom • y: yank • V/esc: exit visual"
	case VisualChar:
		return "h/l: char • w/b/e: word • 0/$: line start/end • j/k: lines • y: yank • v/esc: exit"
	}

	follow := "off"
	if m.FollowingLogs {
		follow = "on"
	}
	return "j/k: scroll • g/G: top/bottom • ?: search • n/N: next/prev • f: follow(" + follow + ") • w: wrap • V: visual • v: select • esc: back"
}
