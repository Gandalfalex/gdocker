package logviewer

import (
	"fmt"
	"image/color"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
)

// Colors duplicated from ui/icons.go to avoid import cycle.
const (
	colorMuted             = "#565f89"
	colorTitle             = "#7aa2f7"
	colorHighlight         = "#283457"
	colorLogFG             = "#c0caf5"
	colorSearchHighlightFG = "#1a1b26"
	colorSearchHighlightBG = "#e0af68"
	colorVisualSelectionBG = "#3d59a1"
	colorCursorBG          = "#c0caf5"
	colorCursorFG          = "#1a1b26"
)

// How a single character of a log line is drawn.
const (
	spanPlain uint8 = iota
	spanMatch
	spanSelected
	spanCursor
)

var mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))

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

	subtitle := fmt.Sprintf("%s · %d lines · %s · %s", containerName, len(m.Lines), mode, wrapLabel)
	if m.SearchQuery != "" && !m.SearchMode {
		subtitle += fmt.Sprintf(" · /%s (%d matches)", m.SearchQuery, len(m.SearchResults))
	}
	switch m.Sel.Mode {
	case VisualLine:
		subtitle = "-- VISUAL LINE -- · " + subtitle
	case VisualChar:
		subtitle = "-- VISUAL -- · " + subtitle
	}
	s.WriteString(renderPaneHeader("Logs", subtitle))

	if m.Loading {
		s.WriteString(mutedStyle.Render("Loading logs..."))
		return s.String()
	}
	if len(m.Lines) == 0 {
		s.WriteString(mutedStyle.Render("No logs yet"))
		return s.String()
	}

	m.SetSize(width, height)
	lineWidth, rows := m.lineWidth(), m.visibleRows()

	scrollPos := min(max(m.ScrollPos, 0), len(m.Lines)-1)

	var start, end int
	if m.WrapLogs {
		start, end = m.renderWrapped(&s, lineWidth, rows, scrollPos)
	} else {
		start, end = m.renderTruncated(&s, lineWidth, rows, scrollPos)
	}

	s.WriteString("\n")
	progress := (scrollPos + 1) * 100 / len(m.Lines)
	s.WriteString(mutedStyle.Render(fmt.Sprintf("Line %d/%d (%d-%d visible, %d%%)", scrollPos+1, len(m.Lines), start+1, end+1, progress)))
	return s.String()
}

// renderWrapped writes up to rows screen rows starting at viewTop, wrapping
// long lines, and returns the first and last log line shown.
func (m *Model) renderWrapped(s *strings.Builder, lineWidth, rows, scrollPos int) (int, int) {
	start := m.viewTop()
	written := 0
	for i := start; i < len(m.Lines); i++ {
		runes := []rune(m.Lines[i])
		spans := m.lineSpans(i, runes)
		bg := m.rowBackground(i, scrollPos)
		for j := 0; j == 0 || j*lineWidth < len(runes); j++ {
			if written >= rows {
				if j == 0 {
					i-- // line i did not make it on screen at all
				}
				return start, i
			}
			from, to := j*lineWidth, min((j+1)*lineWidth, len(runes))
			s.WriteString(m.renderGutter(i, j, bg) + renderSpans(runes[from:to], spans[from:to], bg, lineWidth) + "\n")
			written++
		}
	}
	return start, len(m.Lines) - 1
}

// renderTruncated writes one screen row per log line, cutting long lines.
func (m *Model) renderTruncated(s *strings.Builder, lineWidth, rows, scrollPos int) (int, int) {
	start := m.viewTop()
	end := min(start+rows, len(m.Lines))

	for i := start; i < end; i++ {
		runes := []rune(m.Lines[i])
		spans := m.lineSpans(i, runes)
		if len(runes) > lineWidth {
			runes = append(runes[:lineWidth-1:lineWidth-1], '…')
			spans = append(spans[:lineWidth-1:lineWidth-1], spanPlain)
		}
		bg := m.rowBackground(i, scrollPos)
		s.WriteString(m.renderGutter(i, 0, bg) + renderSpans(runes, spans, bg, lineWidth) + "\n")
	}
	return start, end - 1
}

// rowBackground returns the background of a whole row: the visual line
// selection, else the cursor line, else none.
func (m *Model) rowBackground(line, scrollPos int) color.Color {
	if m.Sel.Mode == VisualLine {
		if from, to := m.lineSelectionRange(); line >= from && line <= to {
			return lipgloss.Color(colorVisualSelectionBG)
		}
	}
	if line == scrollPos && m.Sel.Mode != VisualChar {
		return lipgloss.Color(colorHighlight)
	}
	return nil
}

// lineSpans classifies every character of a log line: search match, part of
// the character selection, or the cursor.
func (m *Model) lineSpans(line int, runes []rune) []uint8 {
	spans := make([]uint8, len(runes))

	if query := []rune(strings.ToLower(m.SearchQuery)); len(query) > 0 && !m.SearchMode {
		for i := 0; i+len(query) <= len(runes); i++ {
			match := true
			for k, q := range query {
				if unicode.ToLower(runes[i+k]) != q {
					match = false
					break
				}
			}
			if match {
				for k := range query {
					spans[i+k] = spanMatch
				}
				i += len(query) - 1
			}
		}
	}

	if m.Sel.Mode == VisualChar && len(runes) > 0 {
		sl, sc, el, ec := m.charSelectionRange()
		if line >= sl && line <= el {
			from, to := 0, len(runes)-1
			if line == sl {
				from = max(sc, 0)
			}
			if line == el {
				to = min(ec, len(runes)-1)
			}
			for i := from; i <= to; i++ {
				spans[i] = spanSelected
			}
		}
		if line == m.ScrollPos {
			spans[min(max(m.CursorCol, 0), len(runes)-1)] = spanCursor
		}
	}
	return spans
}

// renderSpans draws runes with their span styles on background bg. Each run
// is styled on its own, so a highlight never resets the colors after it, and
// rows with a background are padded to the full width.
func renderSpans(runes []rune, spans []uint8, bg color.Color, width int) string {
	base := lipgloss.NewStyle().Foreground(lipgloss.Color(colorLogFG))
	if bg != nil {
		base = base.Background(bg)
	}
	style := func(span uint8) lipgloss.Style {
		switch span {
		case spanMatch:
			return base.Foreground(lipgloss.Color(colorSearchHighlightFG)).Background(lipgloss.Color(colorSearchHighlightBG))
		case spanSelected:
			return base.Background(lipgloss.Color(colorVisualSelectionBG))
		case spanCursor:
			return base.Foreground(lipgloss.Color(colorCursorFG)).Background(lipgloss.Color(colorCursorBG))
		}
		return base
	}

	var s strings.Builder
	for i := 0; i < len(runes); {
		j := i
		for j < len(runes) && spans[j] == spans[i] {
			j++
		}
		s.WriteString(style(spans[i]).Render(string(runes[i:j])))
		i = j
	}
	if pad := width - lipgloss.Width(string(runes)); bg != nil && pad > 0 {
		s.WriteString(base.Render(strings.Repeat(" ", pad)))
	}
	return s.String()
}

func (m *Model) renderGutter(lineIdx, wrapIdx int, bg color.Color) string {
	if !m.ShowLineNumbers {
		return ""
	}
	style := mutedStyle
	if bg != nil {
		style = style.Background(bg)
	}
	if wrapIdx == 0 {
		return style.Render(fmt.Sprintf("%4d ", lineIdx+1))
	}
	return style.Render("   · ")
}

func renderPaneHeader(title, subtitle string) string {
	titleStyled := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(colorTitle)).
		Render(title)

	if subtitle == "" {
		return titleStyled + "\n\n"
	}
	return titleStyled + "\n" + mutedStyle.Render(subtitle) + "\n\n"
}

// SearchPrompt is the search input as shown while typing a query.
func (m *Model) SearchPrompt() string { return "/" + m.SearchQuery }

// StatusBarHint returns context-appropriate key hints for the status bar.
func (m *Model) StatusBarHint() string {
	if m.SearchMode {
		return "enter search · esc cancel"
	}

	switch m.Sel.Mode {
	case VisualLine:
		return "j/k extend · gg/G top/bottom · y yank · V/esc leave visual"
	case VisualChar:
		return "h/l char · w/b/e word · 0/$ line start/end · j/k lines · y yank · v/esc leave visual"
	}

	follow := "follow"
	if m.FollowingLogs {
		follow = "pause"
	}
	return "j/k gg/G ^d/^u · / search · n/N next/prev · f " + follow + " · w wrap · V/v select · y yank · : command · esc back"
}

// Partial returns a typed-but-incomplete count or key sequence, e.g. "5g".
func (m *Model) Partial() string { return m.keys.Partial() }
