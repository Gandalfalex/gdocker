package models

import (
	"testing"

	"gdocker/config"

	tea "charm.land/bubbletea/v2"
)

func feed(t *testing.T, m Model, msgs ...tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, msg := range msgs {
		var got tea.Model
		got, cmd = m.Update(msg)
		m = got.(Model)
	}
	return m, cmd
}

func containerModel(cs ...*Container) Model {
	cfg := config.DefaultAppConfig()
	m := Model{KeyBindings: &cfg.KeyBindings, Height: 40}
	for _, c := range cs {
		m.Items = append(m.Items, ListItem{IsContainer: true, Container: c})
	}
	return m
}

func TestDeleteAsksForConfirmation(t *testing.T) {
	orig := DeleteContainerFunc
	t.Cleanup(func() { DeleteContainerFunc = orig })
	deleted := false
	DeleteContainerFunc = func(*Model) tea.Cmd {
		return func() tea.Msg { deleted = true; return nil }
	}

	m := containerModel(&Container{Name: "api", State: "running"})

	// Any key but y cancels.
	got, cmd := feed(t, m, runes("d"))
	if got.Confirm == nil || cmd != nil {
		t.Fatalf("d should only open a prompt: confirm=%v cmd=%v", got.Confirm, cmd)
	}
	got, _ = feed(t, got, runes("n"))
	if got.Confirm != nil || deleted {
		t.Fatalf("n should cancel: confirm=%v deleted=%v", got.Confirm, deleted)
	}

	got, cmd = feed(t, m, runes("d"), runes("y"))
	if got.Confirm != nil || cmd == nil {
		t.Fatalf("y should confirm: confirm=%v cmd=%v", got.Confirm, cmd)
	}
}

func TestHelpSwallowsActionKeys(t *testing.T) {
	orig := DeleteContainerFunc
	t.Cleanup(func() { DeleteContainerFunc = orig })
	DeleteContainerFunc = func(*Model) tea.Cmd { return func() tea.Msg { return nil } }

	m := containerModel(&Container{Name: "api"}, &Container{Name: "db"})
	got, _ := feed(t, m, runes("?"), runes("d"), runes("j"))
	if !got.HelpMode || got.Confirm != nil || got.Cursor != 0 {
		t.Fatalf("help should swallow keys: help=%v confirm=%v cursor=%d", got.HelpMode, got.Confirm, got.Cursor)
	}
	got, _ = feed(t, got, tea.KeyPressMsg{Code: tea.KeyEscape})
	if got.HelpMode {
		t.Fatal("esc should close help")
	}
}

func TestStatusMessageExpires(t *testing.T) {
	m := containerModel(&Container{Name: "api"})
	got, cmd := feed(t, m, ActionResultMsg{Message: "boom", Success: false})
	if !got.StatusError || cmd == nil {
		t.Fatalf("expected error status with clear timer: err=%v cmd=%v", got.StatusError, cmd)
	}

	// A timer of an older message must not clear a newer one.
	got, _ = feed(t, got, statusClearMsg{seq: got.statusSeq - 1})
	if got.StatusMessage != "boom" {
		t.Fatalf("stale timer cleared status")
	}
	got, _ = feed(t, got, statusClearMsg{seq: got.statusSeq})
	if got.StatusMessage != "" {
		t.Fatalf("status not cleared: %q", got.StatusMessage)
	}
}

func TestRefreshKeepsCursorOnSelectedContainer(t *testing.T) {
	origGroup, origRebuild := GroupByProjectFunc, RebuildItemsFunc
	t.Cleanup(func() { GroupByProjectFunc, RebuildItemsFunc = origGroup, origRebuild })
	GroupByProjectFunc = func(cs []Container) ([]Container, []ComposeGroup) { return cs, nil }
	RebuildItemsFunc = func(m *Model) {
		m.Items = nil
		for i := range m.Standalone {
			m.Items = append(m.Items, ListItem{IsContainer: true, Container: &m.Standalone[i]})
		}
	}

	m := containerModel(&Container{Name: "api"}, &Container{Name: "db"})
	m.Cursor = 1 // db
	got, _ := feed(t, m, ContainersRefreshedMsg{Containers: []Container{{Name: "cache"}, {Name: "db"}, {Name: "api"}}})
	if c := got.SelectedContainer(); c == nil || c.Name != "db" {
		t.Fatalf("cursor moved off db: %+v", c)
	}
}

func TestStaleResultsDoNotSwitchView(t *testing.T) {
	m := containerModel(&Container{Name: "api"})
	got, _ := feed(t, m,
		LogsLoadedMsg{Lines: []string{"x"}},
		InspectLoadedMsg{Data: "{}"},
		StatsLoadedMsg{Stats: &ContainerStats{}, Seq: 7},
	)
	if got.ViewMode != ViewDetails {
		t.Fatalf("late result switched view to %v", got.ViewMode)
	}
}

func TestMotionsScrollDetailViewNotList(t *testing.T) {
	m := containerModel(&Container{Name: "api", Env: []string{"A=1", "B=2", "C=3"}, EnvLoaded: true}, &Container{Name: "db"})
	got, _ := feed(t, m, runes("v"), runes("j"), runes("j"))
	if got.Cursor != 0 || got.EnvScroll != 2 {
		t.Fatalf("env view: cursor=%d scroll=%d", got.Cursor, got.EnvScroll)
	}
}

func TestCommandInputTextSpaceAndPaste(t *testing.T) {
	m := containerModel()
	got, _ := feed(t, m,
		runes(":"), runes("s"), runes("s"), runes("h"),
		tea.KeyPressMsg{Code: tea.KeySpace, Text: " "},
		tea.KeyPressMsg{Code: 'ü', Text: "ü"},
		tea.PasteMsg{Content: "host\n"},
	)
	if got.CommandInput != "ssh ühost" {
		t.Fatalf("command input %q", got.CommandInput)
	}
	got, _ = feed(t, got, tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyBackspace},
		tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyBackspace}, tea.KeyPressMsg{Code: tea.KeyBackspace})
	if got.CommandInput != "ssh " {
		t.Fatalf("backspace should remove whole runes, got %q", got.CommandInput)
	}
}

func TestSpaceTogglesProject(t *testing.T) {
	orig := RebuildItemsFunc
	t.Cleanup(func() { RebuildItemsFunc = orig })
	RebuildItemsFunc = func(*Model) {}

	cfg := config.DefaultAppConfig()
	m := Model{KeyBindings: &cfg.KeyBindings, Projects: []ComposeGroup{{Name: "web"}}}
	m.Items = []ListItem{{IsProject: true, Project: &m.Projects[0]}}
	feed(t, m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	if !m.Projects[0].Expanded {
		t.Fatal("space should expand the project")
	}
}

func TestListKeepsScrollOffset(t *testing.T) {
	cfg := config.DefaultAppConfig()
	m := Model{KeyBindings: &cfg.KeyBindings, Items: make([]ListItem, 100), Height: 29} // 20 rows

	// Moving inside the window must not scroll it (no re-centering).
	got, _ := feed(t, m, runes("5"), runes("0"), runes("j")) // 1-4 switch lists, so no 30j
	top := got.ListTop
	got, _ = feed(t, got, runes("k"), runes("k"))
	if got.ListTop != top || top == 0 || got.Cursor != 48 {
		t.Fatalf("window scrolled on small move: top %d -> %d, cursor %d", top, got.ListTop, got.Cursor)
	}

	// Screen-relative and view motions from vimkit.
	// H and L stop scrolloff rows short of the edge and leave the view alone.
	got, _ = feed(t, got, runes("H"))
	if got.Cursor != top+2 || got.ListTop != top {
		t.Fatalf("H: cursor %d, top %d -> %d", got.Cursor, top, got.ListTop)
	}
	got, _ = feed(t, got, runes("L"))
	if got.Cursor != top+17 || got.ListTop != top {
		t.Fatalf("L: cursor %d, top %d -> %d", got.Cursor, top, got.ListTop)
	}
	got, _ = feed(t, got, runes("z"), runes("z"))
	if got.ListTop != got.Cursor-10 {
		t.Fatalf("zz: cursor %d, top %d", got.Cursor, got.ListTop)
	}
	start, end := Visible(got.Cursor, got.ListTop, 100, 20)
	if start != got.ListTop || end != start+20 {
		t.Fatalf("render window %d-%d does not match top %d", start, end, got.ListTop)
	}
}

func TestCommandLineCompletionHistoryAndLineJump(t *testing.T) {
	origQuit := QuitFunc
	t.Cleanup(func() { QuitFunc = origQuit })
	QuitFunc = func(*Model) {}

	cfg := config.DefaultAppConfig()
	m := Model{KeyBindings: &cfg.KeyBindings, Items: make([]ListItem, 100), Height: 29}
	tab := tea.KeyPressMsg{Code: tea.KeyTab}
	enter := tea.KeyPressMsg{Code: tea.KeyEnter}
	up := tea.KeyPressMsg{Code: tea.KeyUp}

	// Command names complete and cycle; a lone match moves on to its argument.
	got, _ := feed(t, m, runes(":"), runes("s"), tab)
	if got.CommandInput != "start" {
		t.Fatalf("first candidate for s: %q", got.CommandInput)
	}
	got, _ = feed(t, got, tab)
	if got.CommandInput != "stop" {
		t.Fatalf("second candidate for s: %q", got.CommandInput)
	}
	got, _ = feed(t, m, runes(":"), runes("s"), runes("s"), tab, tab)
	if got.CommandInput != "ssh " {
		t.Fatalf("ssh should complete to its argument, got %q", got.CommandInput)
	}

	// :42 jumps to line 42, a unique prefix runs its command, and both are
	// kept in the history.
	got, _ = feed(t, m, runes(":"), runes("4"), runes("2"), enter)
	if got.Cursor != 41 || got.CommandMode {
		t.Fatalf(":42: cursor %d", got.Cursor)
	}
	got, _ = feed(t, got, runes(":"), runes("h"), runes("e"), runes("l"), enter)
	if !got.HelpMode {
		t.Fatal(":hel should open help")
	}
	got, _ = feed(t, got, tea.KeyPressMsg{Code: tea.KeyEscape}, runes(":"), up, up)
	if got.CommandInput != "42" {
		t.Fatalf("history: %q", got.CommandInput)
	}

	got, _ = feed(t, m, runes(":"), runes("x"), runes("y"), enter)
	if !got.StatusError || got.StatusMessage == "" {
		t.Fatalf("unknown command should report an error: %q", got.StatusMessage)
	}
}
