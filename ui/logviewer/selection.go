package logviewer

import (
	"fmt"
	"strings"
)

// VisualType represents the current visual selection mode.
type VisualType int

const (
	VisualNone VisualType = iota
	VisualLine
	VisualChar
)

// selectedText returns the text currently selected based on visual mode.
func (m *Model) selectedText() string {
	switch m.VisualType {
	case VisualLine:
		return m.selectedLines()
	case VisualChar:
		return m.selectedChars()
	default:
		return ""
	}
}

func (m *Model) selectedLines() string {
	start, end := m.lineSelectionRange()
	if start < 0 || end < 0 || start >= len(m.Lines) {
		return ""
	}
	return strings.Join(m.Lines[start:end+1], "\n")
}

func (m *Model) selectedChars() string {
	startLine, startCol, endLine, endCol := m.charSelectionRange()
	if startLine < 0 || endLine < 0 {
		return ""
	}

	if startLine == endLine {
		line := m.Lines[startLine]
		runes := []rune(line)
		if startCol >= len(runes) {
			return ""
		}
		ec := endCol + 1
		if ec > len(runes) {
			ec = len(runes)
		}
		return string(runes[startCol:ec])
	}

	var parts []string
	// First line: from startCol to end
	first := []rune(m.Lines[startLine])
	sc := startCol
	if sc > len(first) {
		sc = len(first)
	}
	parts = append(parts, string(first[sc:]))

	// Middle lines: full lines
	for i := startLine + 1; i < endLine; i++ {
		parts = append(parts, m.Lines[i])
	}

	// Last line: from start to endCol
	last := []rune(m.Lines[endLine])
	ec := endCol + 1
	if ec > len(last) {
		ec = len(last)
	}
	parts = append(parts, string(last[:ec]))

	return strings.Join(parts, "\n")
}

// lineSelectionRange returns the ordered start/end line indices.
func (m *Model) lineSelectionRange() (int, int) {
	start := m.AnchorLine
	end := m.ScrollPos
	if start > end {
		start, end = end, start
	}
	if start < 0 {
		start = 0
	}
	if end >= len(m.Lines) {
		end = len(m.Lines) - 1
	}
	return start, end
}

// charSelectionRange returns ordered (startLine, startCol, endLine, endCol).
func (m *Model) charSelectionRange() (int, int, int, int) {
	sl, sc := m.AnchorLine, m.AnchorCol
	el, ec := m.ScrollPos, m.CursorCol

	// Order by line, then by column
	if sl > el || (sl == el && sc > ec) {
		sl, el = el, sl
		sc, ec = ec, sc
	}

	// Clamp
	if sl < 0 {
		sl = 0
	}
	if el >= len(m.Lines) {
		el = len(m.Lines) - 1
	}
	return sl, sc, el, ec
}

// yank copies the selected text to the clipboard and exits visual mode.
func (m *Model) yank() string {
	wasCharMode := m.VisualType == VisualChar
	text := m.selectedText()
	if text == "" {
		m.VisualType = VisualNone
		return "Nothing to yank"
	}

	if m.CopyFn != nil {
		if err := m.CopyFn(text); err != nil {
			m.VisualType = VisualNone
			return "Copy failed: " + err.Error()
		}
	} else {
		m.VisualType = VisualNone
		return "Clipboard not available"
	}

	m.VisualType = VisualNone

	if wasCharMode {
		return "Yanked selection"
	}
	lineCount := strings.Count(text, "\n") + 1
	return fmt.Sprintf("%d line(s) yanked", lineCount)
}
