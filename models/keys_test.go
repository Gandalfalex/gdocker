package models

import (
	"testing"

	"gdocker/config"

	tea "charm.land/bubbletea/v2"
)

func runes(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
}

func TestListMotionsWithCountsAndDigitTabs(t *testing.T) {
	origRebuild := RebuildVolumeItemsFunc
	t.Cleanup(func() { RebuildVolumeItemsFunc = origRebuild })
	RebuildVolumeItemsFunc = func(*Model) {}

	cfg := config.DefaultAppConfig()
	m := Model{KeyBindings: &cfg.KeyBindings, Items: make([]ListItem, 20), Height: 40}
	step := func(k string) {
		got, _ := m.Update(runes(k))
		m = got.(Model)
	}
	for _, k := range []string{"5", "j"} {
		step(k)
	}
	if m.Cursor != 5 {
		t.Fatalf("5j: cursor %d", m.Cursor)
	}
	step("G")
	step("g")
	step("g")
	if m.Cursor != 0 {
		t.Fatalf("gg: cursor %d", m.Cursor)
	}
	step("2") // bound to switch_volume, not a count
	if m.NavMode != NavVolumes {
		t.Fatalf("2 should switch to volumes, nav=%v", m.NavMode)
	}
}

func TestEnterOpensPortOnlyInPortsView(t *testing.T) {
	origRebuild, origOpen := RebuildItemsFunc, OpenPortInBrowserFunc
	t.Cleanup(func() { RebuildItemsFunc, OpenPortInBrowserFunc = origRebuild, origOpen })
	toggled, opened := false, false
	RebuildItemsFunc = func(*Model) { toggled = true }
	OpenPortInBrowserFunc = func(*Model) tea.Cmd { opened = true; return nil }

	cfg := config.DefaultAppConfig()
	enter := tea.KeyPressMsg{Code: tea.KeyEnter}

	m := Model{KeyBindings: &cfg.KeyBindings, ViewMode: ViewDetails,
		Projects: []ComposeGroup{{}}, Items: []ListItem{{IsProject: true}}}
	m.Update(enter)
	if !toggled || opened {
		t.Errorf("details view: toggled=%v opened=%v", toggled, opened)
	}

	toggled = false
	m = Model{KeyBindings: &cfg.KeyBindings, ViewMode: ViewPorts,
		Items: []ListItem{{IsContainer: true, Container: &Container{}}}}
	m.Update(enter)
	if toggled || !opened {
		t.Errorf("ports view: toggled=%v opened=%v", toggled, opened)
	}
}
