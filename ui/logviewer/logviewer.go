package logviewer

import (
	"strings"
	"unicode/utf8"

	"gdocker/config"

	tea "charm.land/bubbletea/v2"
	"github.com/Gandalfalex/vim-kit/keys"
	"github.com/Gandalfalex/vim-kit/motion"
	"github.com/Gandalfalex/vim-kit/visual"
	"github.com/charmbracelet/x/ansi"
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

	// Visual selection; the cursor is (ScrollPos, CursorCol)
	Sel       visual.Selection
	CursorCol int

	// Display
	Loading         bool // initial fetch in progress
	FollowingLogs   bool
	WrapLogs        bool
	ShowLineNumbers bool

	// Config
	KeyBindings *config.LogKeys
	CopyFn      func(string) error

	keys    keys.Input
	keymaps *logKeymaps
	width   int
	height  int
	top     int // first visible line; see viewTop
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

// SetLoading empties the viewer while the logs of a container are fetched.
func (m *Model) SetLoading(containerName string) {
	m.SetLines(nil, containerName)
	m.Loading = true
	m.FollowingLogs = false
	m.SearchMode = false
	m.SearchQuery = ""
	m.SearchResults = nil
	m.SearchResultIdx = 0
	m.keys.Reset()
}

// SetLines replaces the log lines and scrolls to the bottom.
func (m *Model) SetLines(lines []string, containerName string) {
	m.Lines = make([]string, len(lines))
	for i, line := range lines {
		m.Lines[i] = sanitize(line)
	}
	m.Loading = false
	m.ScrollPos = max(len(lines)-1, 0)
	m.top = 0
	m.CursorCol = 0
	m.ContainerName = containerName
	m.Sel.Clear()
}

// AppendLines adds new lines (from follow mode). The view only keeps up with
// the stream while the cursor sits on the last line, so scrolling back or
// selecting text is not interrupted by new output.
func (m *Model) AppendLines(lines []string) {
	atBottom := m.ScrollPos >= len(m.Lines)-1 && !m.Sel.Active()
	for _, line := range lines {
		m.Lines = append(m.Lines, sanitize(line))
	}
	if atBottom {
		m.ScrollPos = len(m.Lines) - 1
	}
	// Keep search results current without moving the cursor.
	if m.SearchQuery != "" && !m.SearchMode {
		m.SearchResults = m.findMatches()
		m.SearchResultIdx = min(m.SearchResultIdx, max(len(m.SearchResults)-1, 0))
	}
}

// sanitize makes a raw log line safe to lay out: escape sequences and control
// characters would otherwise corrupt the pane or break column math.
func sanitize(line string) string {
	line = ansi.Strip(line)
	if !strings.ContainsFunc(line, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return line
	}
	var b strings.Builder
	for _, r := range line {
		switch {
		case r == '\t':
			b.WriteString("    ")
		case r < 0x20 || r == 0x7f:
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// IsSearchMode returns whether the component is currently capturing search input.
func (m *Model) IsSearchMode() bool {
	return m.SearchMode
}

// Update handles key presses (and pastes into the search prompt).
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		if m.SearchMode {
			m.SearchQuery += msg.Content
		}
	case tea.KeyPressMsg:
		// Search mode input takes priority
		if m.SearchMode {
			return m.updateSearchMode(msg)
		}
		return m.handleKey(msg.String())
	}
	return m, nil
}

func (m Model) updateSearchMode(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.SearchMode = false
		m.performSearch()
		if m.SearchQuery == "" {
			return m, status("")
		}
		if len(m.SearchResults) > 0 {
			return m, status(m.formatSearchStatus())
		}
		return m, status("No matches for " + m.SearchQuery)
	case "esc":
		m.SearchMode = false
		m.SearchQuery = ""
		m.performSearch()
		return m, status("")
	case "backspace":
		if m.SearchQuery == "" {
			m.SearchMode = false
			m.performSearch()
			return m, nil
		}
		_, size := utf8.DecodeLastRuneInString(m.SearchQuery)
		m.SearchQuery = m.SearchQuery[:len(m.SearchQuery)-size]
	default:
		m.SearchQuery += msg.Text
	}
	return m, nil
}

// logKeymaps are built from KeyBindings on first use.
type logKeymaps struct{ normal, line, char *keys.Keymap }

func (m *Model) keymap() *keys.Keymap {
	if m.keymaps == nil {
		kb := m.KeyBindings
		if kb == nil {
			kb = &config.Default().Logs
		}
		km, err := buildLogKeymaps(kb)
		if err != nil { // Load validates; fall back to defaults just in case
			km, _ = buildLogKeymaps(&config.Default().Logs)
		}
		m.keymaps = km
	}
	switch m.Sel.Mode {
	case visual.Line:
		return m.keymaps.line
	case visual.Char:
		return m.keymaps.char
	}
	return m.keymaps.normal
}

func buildLogKeymaps(kb *config.LogKeys) (*logKeymaps, error) {
	var km logKeymaps
	var err error
	if km.normal, err = kb.LogNormalKeymap(); err != nil {
		return nil, err
	}
	if km.line, err = kb.LogVisualKeymap(false); err != nil {
		return nil, err
	}
	if km.char, err = kb.LogVisualKeymap(true); err != nil {
		return nil, err
	}
	return &km, nil
}

// SetSize sets the size passed to View, so motions know what is on screen.
func (m *Model) SetSize(width, height int) { m.width, m.height = width, height }

// SetHeight is SetSize for callers that only care about page motions.
func (m *Model) SetHeight(h int) { m.height = h }

func (m *Model) visibleRows() int { return max(m.height-7, 1) } // pane header and footer

// lineWidth is the number of characters of a log line that fit one row.
func (m *Model) lineWidth() int {
	gutter := 0
	if m.ShowLineNumbers {
		gutter = 5
	}
	return max(m.width-gutter, 12)
}

// rowsOf is the number of screen rows log line i takes.
func (m *Model) rowsOf(i int) int {
	if !m.WrapLogs {
		return 1
	}
	w := m.lineWidth()
	return max((utf8.RuneCountInString(m.Lines[i])+w-1)/w, 1)
}

// viewTop returns the first line to draw: the stored scroll offset, moved
// only as far as needed to bring the cursor line on screen. Wrapped lines
// take several rows, so there the offset is found by counting rows.
func (m *Model) viewTop() int {
	n, rows := len(m.Lines), m.visibleRows()
	if n == 0 {
		return 0
	}
	cur := min(max(m.ScrollPos, 0), n-1)
	if !m.WrapLogs {
		w := motion.Window{Cursor: cur, Top: m.top}
		w.Scroll(n, rows)
		return w.Top
	}
	top := min(max(m.top, cur-rows+1, 0), cur) // every line takes at least a row
	used := 0
	for i := top; i <= cur; i++ {
		used += m.rowsOf(i)
	}
	for used > rows && top < cur {
		used -= m.rowsOf(top)
		top++
	}
	return top
}

// visibleLines counts the whole log lines on screen starting at top.
func (m *Model) visibleLines(top int) int {
	rows, used, count := m.visibleRows(), 0, 0
	for i := top; i < len(m.Lines) && used+m.rowsOf(i) <= rows; i++ {
		used += m.rowsOf(i)
		count++
	}
	return max(count, 1)
}

func (m *Model) cursor() visual.Pos { return visual.Pos{Line: m.ScrollPos, Col: m.CursorCol} }

func status(text string) tea.Cmd { return func() tea.Msg { return StatusMsg{Text: text} } }

// applyMotion moves the cursor line (and, in char visual mode, column).
func (m *Model) applyMotion(ev keys.Event) bool {
	w := motion.Window{Cursor: m.ScrollPos, Top: m.viewTop()}
	if len(m.Lines) > 0 && w.Apply(ev, len(m.Lines), m.visibleLines(w.Top)) {
		m.ScrollPos, m.top = w.Cursor, w.Top
		m.top = m.viewTop()
		if m.Sel.Mode == visual.Char && m.ScrollPos < len(m.Lines) {
			m.CursorCol = motion.ClampCol(m.Lines[m.ScrollPos], m.CursorCol)
		}
		return true
	}
	if m.Sel.Mode == visual.Char && m.ScrollPos >= 0 && m.ScrollPos < len(m.Lines) {
		col, ok := motion.ApplyText(ev, m.Lines[m.ScrollPos], m.CursorCol)
		m.CursorCol = col
		return ok
	}
	return false
}

// Goto runs a motion that did not come from a key press, e.g. ":42".
func (m *Model) Goto(ev keys.Event) { m.applyMotion(ev) }

func (m Model) handleKey(key string) (Model, tea.Cmd) {
	ev, ok := m.keys.Feed(m.keymap(), key)
	if !ok || m.applyMotion(ev) {
		return m, nil
	}

	switch ev.Action {
	case "search":
		m.SearchMode = true
		m.SearchQuery = ""
		return m, status("")

	case "next_result", "prev_result":
		if len(m.SearchResults) > 0 {
			d := ev.Count
			if ev.Action == "prev_result" {
				d = -d
			}
			n := len(m.SearchResults)
			m.SearchResultIdx = ((m.SearchResultIdx+d)%n + n) % n
			m.ScrollPos = m.SearchResults[m.SearchResultIdx]
			return m, status(m.formatSearchStatus())
		}

	case "follow":
		m.FollowingLogs = !m.FollowingLogs
		if m.FollowingLogs {
			return m, func() tea.Msg { return FollowRequestMsg{} }
		}
		return m, status("Log follow paused")

	case "toggle_wrap":
		m.WrapLogs = !m.WrapLogs
		label := "enabled"
		if !m.WrapLogs {
			label = "disabled"
		}
		return m, status("Line wrap " + label)

	case "visual_line":
		if m.Sel.Mode == visual.Line {
			m.Sel.Clear()
			return m, status("")
		}
		m.Sel.Start(visual.Line, m.cursor())
		return m, status("-- VISUAL LINE --")

	case "char_visual":
		if m.Sel.Mode == visual.Char {
			m.Sel.Clear()
			return m, status("")
		}
		// v starts at the beginning of the line
		m.CursorCol = 0
		m.Sel.Start(visual.Char, m.cursor())
		m.Sel.Anchor.Col = 0
		return m, status("-- VISUAL --")

	case "yank":
		if m.Sel.Active() {
			return m, status(m.yank())
		}
		// Yank current line in normal mode
		if m.ScrollPos >= 0 && m.ScrollPos < len(m.Lines) && m.CopyFn != nil {
			if err := m.CopyFn(m.Lines[m.ScrollPos]); err != nil {
				return m, status("Copy failed: " + err.Error())
			}
			return m, status("1 line yanked")
		}

	case config.ActExit:
		if m.Sel.Active() {
			m.Sel.Clear()
			return m, status("")
		}
		return m, func() tea.Msg { return ExitMsg{} }
	}
	return m, nil
}
