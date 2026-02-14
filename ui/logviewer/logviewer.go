package logviewer

import (
	"gdocker/config"

	tea "github.com/charmbracelet/bubbletea"
)

// Output messages emitted by the component for the parent to handle.
type FollowRequestMsg struct{}
type ExitMsg struct{}
type StatusMsg struct{ Text string }

// Model is a self-contained bubbletea sub-model for the log viewer.
type Model struct {
	Lines         []string
	ScrollPos     int
	ContainerName string

	// Search
	SearchMode      bool
	SearchQuery     string
	SearchResults   []int
	SearchResultIdx int

	// Visual selection
	VisualType VisualType
	AnchorLine int
	AnchorCol  int
	CursorCol  int

	// Display
	FollowingLogs   bool
	WrapLogs        bool
	ShowLineNumbers bool

	// Config
	KeyBindings *config.LogKeys
	CopyFn      func(string) error
}

// New creates a new log viewer component.
func New(keys *config.LogKeys, showLineNumbers bool, wrapLogs bool, copyFn func(string) error) Model {
	return Model{
		KeyBindings:     keys,
		ShowLineNumbers: showLineNumbers,
		WrapLogs:        wrapLogs,
		CopyFn:          copyFn,
	}
}

// SetLines replaces the log lines and scrolls to the bottom.
func (m *Model) SetLines(lines []string, containerName string) {
	m.Lines = lines
	m.ScrollPos = len(lines) - 1
	m.ContainerName = containerName
	m.VisualType = VisualNone
}

// AppendLines adds new lines (from follow mode) and scrolls to the bottom.
func (m *Model) AppendLines(lines []string) {
	for _, line := range lines {
		if len(m.Lines) == 0 || m.Lines[len(m.Lines)-1] != line {
			m.Lines = append(m.Lines, line)
		}
	}
	m.ScrollPos = len(m.Lines) - 1
	// Re-run search if active
	if m.SearchQuery != "" {
		m.performSearch()
	}
}

// IsSearchMode returns whether the component is currently capturing search input.
func (m *Model) IsSearchMode() bool {
	return m.SearchMode
}

// Update handles all key events for the log viewer.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	key := keyMsg.String()

	// Search mode input takes priority
	if m.SearchMode {
		return m.updateSearchMode(key)
	}

	// Visual char mode keys
	if m.VisualType == VisualChar {
		return m.updateCharVisual(key)
	}

	// Visual line mode keys
	if m.VisualType == VisualLine {
		return m.updateLineVisual(key)
	}

	// Normal mode
	return m.updateNormal(key)
}

func (m Model) updateSearchMode(key string) (Model, tea.Cmd) {
	switch key {
	case "enter":
		m.SearchMode = false
		m.performSearch()
		if len(m.SearchResults) > 0 {
			return m, func() tea.Msg { return StatusMsg{Text: m.formatSearchStatus()} }
		}
		return m, func() tea.Msg { return StatusMsg{Text: "No matches found"} }
	case "esc":
		m.SearchMode = false
		m.SearchQuery = ""
		return m, func() tea.Msg { return StatusMsg{Text: ""} }
	case "backspace":
		if len(m.SearchQuery) > 0 {
			m.SearchQuery = m.SearchQuery[:len(m.SearchQuery)-1]
		}
		return m, nil
	default:
		if len(key) == 1 {
			m.SearchQuery += key
		}
		return m, nil
	}
}

func (m Model) updateNormal(key string) (Model, tea.Cmd) {
	kb := m.KeyBindings

	switch {
	// Navigation
	case config.Contains(kb.Search, key):
		m.SearchMode = true
		m.SearchQuery = ""
		return m, func() tea.Msg { return StatusMsg{Text: ""} }

	case config.Contains(kb.NextResult, key):
		if len(m.SearchResults) > 0 {
			m.SearchResultIdx++
			if m.SearchResultIdx >= len(m.SearchResults) {
				m.SearchResultIdx = 0
			}
			m.ScrollPos = m.SearchResults[m.SearchResultIdx]
			return m, func() tea.Msg { return StatusMsg{Text: m.formatSearchStatus()} }
		}

	case config.Contains(kb.PrevResult, key):
		if len(m.SearchResults) > 0 {
			m.SearchResultIdx--
			if m.SearchResultIdx < 0 {
				m.SearchResultIdx = len(m.SearchResults) - 1
			}
			m.ScrollPos = m.SearchResults[m.SearchResultIdx]
			return m, func() tea.Msg { return StatusMsg{Text: m.formatSearchStatus()} }
		}

	case config.Contains(kb.Follow, key):
		m.FollowingLogs = !m.FollowingLogs
		if m.FollowingLogs {
			return m, func() tea.Msg { return FollowRequestMsg{} }
		}
		return m, func() tea.Msg { return StatusMsg{Text: "Log follow paused"} }

	case config.Contains(kb.ToggleWrap, key):
		m.WrapLogs = !m.WrapLogs
		label := "enabled"
		if !m.WrapLogs {
			label = "disabled"
		}
		return m, func() tea.Msg { return StatusMsg{Text: "Line wrap " + label} }

	case config.Contains(kb.VisualLine, key):
		m.VisualType = VisualLine
		m.AnchorLine = m.ScrollPos
		return m, func() tea.Msg { return StatusMsg{Text: "-- VISUAL LINE --"} }

	case config.Contains(kb.CharVisual, key):
		m.VisualType = VisualChar
		m.AnchorLine = m.ScrollPos
		m.AnchorCol = 0
		m.CursorCol = 0
		return m, func() tea.Msg { return StatusMsg{Text: "-- VISUAL --"} }

	case config.Contains(kb.Yank, key):
		// Yank current line in normal mode
		if m.ScrollPos >= 0 && m.ScrollPos < len(m.Lines) {
			line := m.Lines[m.ScrollPos]
			if m.CopyFn != nil {
				if err := m.CopyFn(line); err != nil {
					return m, func() tea.Msg { return StatusMsg{Text: "Copy failed: " + err.Error()} }
				}
				return m, func() tea.Msg { return StatusMsg{Text: "1 line yanked"} }
			}
		}
	}

	// Scroll navigation (use raw key matching for j/k/g/G since these are
	// registered in the parent's navigation keys, not LogKeys)
	switch key {
	case "k", "up":
		if m.ScrollPos > 0 {
			m.ScrollPos--
		}
	case "j", "down":
		if m.ScrollPos < len(m.Lines)-1 {
			m.ScrollPos++
		}
	case "g":
		m.ScrollPos = 0
	case "G":
		m.ScrollPos = len(m.Lines) - 1
	case "esc":
		return m, func() tea.Msg { return ExitMsg{} }
	}

	return m, nil
}

func (m Model) updateLineVisual(key string) (Model, tea.Cmd) {
	kb := m.KeyBindings

	switch {
	case config.Contains(kb.Yank, key):
		msg := m.yank()
		return m, func() tea.Msg { return StatusMsg{Text: msg} }

	case config.Contains(kb.VisualLine, key), key == "esc":
		m.VisualType = VisualNone
		return m, func() tea.Msg { return StatusMsg{Text: ""} }

	case config.Contains(kb.CharVisual, key):
		// Switch to char visual
		m.VisualType = VisualChar
		m.AnchorCol = 0
		m.CursorCol = 0
		return m, func() tea.Msg { return StatusMsg{Text: "-- VISUAL --"} }
	}

	// Navigation within visual line mode
	switch key {
	case "k", "up":
		if m.ScrollPos > 0 {
			m.ScrollPos--
		}
	case "j", "down":
		if m.ScrollPos < len(m.Lines)-1 {
			m.ScrollPos++
		}
	case "g":
		m.ScrollPos = 0
	case "G":
		m.ScrollPos = len(m.Lines) - 1
	}

	return m, nil
}

func (m Model) updateCharVisual(key string) (Model, tea.Cmd) {
	kb := m.KeyBindings

	switch {
	case config.Contains(kb.Yank, key):
		msg := m.yank()
		return m, func() tea.Msg { return StatusMsg{Text: msg} }

	case config.Contains(kb.CharVisual, key), key == "esc":
		m.VisualType = VisualNone
		return m, func() tea.Msg { return StatusMsg{Text: ""} }

	case config.Contains(kb.VisualLine, key):
		// Switch to line visual
		m.VisualType = VisualLine
		return m, func() tea.Msg { return StatusMsg{Text: "-- VISUAL LINE --"} }
	}

	lineLen := 0
	if m.ScrollPos >= 0 && m.ScrollPos < len(m.Lines) {
		lineLen = len([]rune(m.Lines[m.ScrollPos]))
	}

	switch key {
	case "h", "left":
		if m.CursorCol > 0 {
			m.CursorCol--
		}
	case "l", "right":
		if m.CursorCol < lineLen-1 {
			m.CursorCol++
		}
	case "w":
		if m.ScrollPos >= 0 && m.ScrollPos < len(m.Lines) {
			m.CursorCol = wordForward(m.Lines[m.ScrollPos], m.CursorCol)
		}
	case "b":
		if m.ScrollPos >= 0 && m.ScrollPos < len(m.Lines) {
			m.CursorCol = wordBackward(m.Lines[m.ScrollPos], m.CursorCol)
		}
	case "e":
		if m.ScrollPos >= 0 && m.ScrollPos < len(m.Lines) {
			m.CursorCol = wordEnd(m.Lines[m.ScrollPos], m.CursorCol)
		}
	case "0":
		m.CursorCol = 0
	case "$":
		if lineLen > 0 {
			m.CursorCol = lineLen - 1
		}
	case "k", "up":
		if m.ScrollPos > 0 {
			m.ScrollPos--
			// Clamp column
			newLen := len([]rune(m.Lines[m.ScrollPos]))
			if m.CursorCol >= newLen && newLen > 0 {
				m.CursorCol = newLen - 1
			}
		}
	case "j", "down":
		if m.ScrollPos < len(m.Lines)-1 {
			m.ScrollPos++
			// Clamp column
			newLen := len([]rune(m.Lines[m.ScrollPos]))
			if m.CursorCol >= newLen && newLen > 0 {
				m.CursorCol = newLen - 1
			}
		}
	case "g":
		m.ScrollPos = 0
		newLen := len([]rune(m.Lines[m.ScrollPos]))
		if m.CursorCol >= newLen && newLen > 0 {
			m.CursorCol = newLen - 1
		}
	case "G":
		m.ScrollPos = len(m.Lines) - 1
		newLen := len([]rune(m.Lines[m.ScrollPos]))
		if m.CursorCol >= newLen && newLen > 0 {
			m.CursorCol = newLen - 1
		}
	}

	return m, nil
}
