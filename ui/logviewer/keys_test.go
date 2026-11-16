package logviewer

import (
	"strings"
	"testing"

	"gdocker/config"

	tea "charm.land/bubbletea/v2"
)

func press(m Model, ks ...string) Model {
	for _, k := range ks {
		var msg tea.KeyPressMsg
		switch k {
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		case "ctrl+d":
			msg = tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl}
		default:
			msg = tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
		}
		m, _ = m.Update(msg)
	}
	return m
}

func newTestViewer(copied *string) Model {
	m := New(&config.Default().Logs, true, true, func(s string) error { *copied = s; return nil })
	m.SetLines([]string{"zero", "one two.three", "two", "three", "four", "five"}, "c")
	m.SetHeight(10) // 3 visible rows
	return m
}

func TestLogViewerMotions(t *testing.T) {
	var copied string
	m := newTestViewer(&copied)
	cases := []struct {
		keys []string
		want int
	}{
		{[]string{"g", "g"}, 0},
		{[]string{"g", "g", "3", "j"}, 3},
		{[]string{"2", "G"}, 1},
		{[]string{"g", "g", "ctrl+d"}, 1},
		{[]string{"G", "k"}, 4},
	}
	for _, c := range cases {
		if got := press(m, c.keys...).ScrollPos; got != c.want {
			t.Errorf("%v: line %d, want %d", c.keys, got, c.want)
		}
	}
}

func TestLogViewerCharVisualYank(t *testing.T) {
	var copied string
	m := newTestViewer(&copied)
	m = press(m, "2", "G", "v", "w", "e", "y")
	if copied != "one two" {
		t.Errorf("yanked %q, want %q", copied, "one two")
	}
	if m.Sel.Active() {
		t.Error("yank should leave visual mode")
	}

	m = press(m, "V", "j", "y")
	if copied != "one two.three\ntwo" {
		t.Errorf("line yank %q", copied)
	}

	// w toggles wrap outside visual mode
	if got := press(m, "w").WrapLogs; got {
		t.Error("w should toggle wrap off in normal mode")
	}
}

func TestLogViewerEscLeavesVisualThenExits(t *testing.T) {
	var copied string
	m := newTestViewer(&copied)
	m = press(m, "V", "esc")
	if m.Sel.Active() {
		t.Error("esc should end visual mode")
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape}); cmd == nil {
		t.Fatal("expected exit cmd")
	} else if _, ok := cmd().(ExitMsg); !ok {
		t.Error("expected ExitMsg")
	}
}

func TestLogViewerWrappedScrollKeepsCursorVisible(t *testing.T) {
	m := New(&config.Default().Logs, false, true, nil)
	var lines []string
	for i := range 50 {
		lines = append(lines, strings.Repeat("x", 10+i%3*25)) // 1-3 rows at width 20
	}
	m.SetLines(lines, "c")
	m.SetSize(20, 17) // 10 rows

	visible := func(m Model) bool {
		top, used := m.viewTop(), 0
		for i := top; i <= m.ScrollPos; i++ {
			used += m.rowsOf(i)
		}
		return top <= m.ScrollPos && used <= m.visibleRows()
	}
	for _, keys := range [][]string{{"g", "g"}, {"ctrl+d"}, {"1", "5", "j"}, {"L"}, {"H"}, {"M"}, {"z", "z"}, {"G"}, {"k"}} {
		m = press(m, keys...)
		if !visible(m) {
			t.Fatalf("after %v: cursor line %d not on screen (top %d)", keys, m.ScrollPos, m.viewTop())
		}
	}

	// H and L stay on screen: they must not scroll the view.
	m = press(m, "g", "g", "1", "0", "j")
	top := m.viewTop()
	if m = press(m, "L"); m.viewTop() != top {
		t.Fatalf("L scrolled the view: top %d -> %d", top, m.viewTop())
	}
	if m = press(m, "H"); m.viewTop() != top || m.ScrollPos != top {
		t.Fatalf("H: cursor %d, top %d -> %d", m.ScrollPos, top, m.viewTop())
	}
}
