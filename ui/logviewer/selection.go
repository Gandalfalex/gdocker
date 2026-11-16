package logviewer

import (
	"fmt"
	"strings"

	"github.com/Gandalfalex/vim-kit/visual"
)

// Visual modes, kept as names for rendering.
const (
	VisualNone = visual.None
	VisualLine = visual.Line
	VisualChar = visual.Char
)

// lineSelectionRange returns the ordered start/end line indices.
func (m *Model) lineSelectionRange() (int, int) {
	from, to := m.Sel.Bounds(m.cursor(), m.Lines)
	return from.Line, to.Line
}

// charSelectionRange returns ordered (startLine, startCol, endLine, endCol).
func (m *Model) charSelectionRange() (int, int, int, int) {
	from, to := m.Sel.Bounds(m.cursor(), m.Lines)
	return from.Line, from.Col, to.Line, to.Col
}

// yank copies the selected text to the clipboard and exits visual mode.
func (m *Model) yank() string {
	wasCharMode := m.Sel.Mode == visual.Char
	text := m.Sel.Text(m.cursor(), m.Lines)
	m.Sel.Clear()
	switch {
	case text == "":
		return "Nothing to yank"
	case m.CopyFn == nil:
		return "Clipboard not available"
	}
	if err := m.CopyFn(text); err != nil {
		return "Copy failed: " + err.Error()
	}
	if wasCharMode {
		return "Yanked selection"
	}
	return fmt.Sprintf("%d line(s) yanked", strings.Count(text, "\n")+1)
}
