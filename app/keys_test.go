package app

import (
	"testing"

	"gdocker/config"

	tea "charm.land/bubbletea/v2"
	"gdocker/models"
)

func runes(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
}

func TestListMotionsWithCountsAndDigitTabs(t *testing.T) {
	cfg := config.DefaultAppConfig()
	m := Model{KeyBindings: &cfg.KeyBindings, Items: make([]models.ListItem, 20), Height: 40}
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
	if m.NavMode != models.NavVolumes {
		t.Fatalf("2 should switch to volumes, nav=%v", m.NavMode)
	}
}

func TestEnterOpensPortOnlyInPortsView(t *testing.T) {
	opened := false
	fake := &fakeBackend{openPort: func(*models.Container, int) tea.Cmd { opened = true; return nil }}

	cfg := config.DefaultAppConfig()
	enter := tea.KeyPressMsg{Code: tea.KeyEnter}

	m := Model{KeyBindings: &cfg.KeyBindings, ViewMode: models.ViewDetails, Backend: fake,
		Projects: []models.ComposeGroup{{}}, Items: []models.ListItem{{IsProject: true}}}
	got, _ := m.Update(enter)
	if toggled := got.(Model).Projects[0].Expanded; !toggled || opened {
		t.Errorf("details view: toggled=%v opened=%v", toggled, opened)
	}

	m = Model{KeyBindings: &cfg.KeyBindings, ViewMode: models.ViewPorts, Backend: fake,
		Projects: []models.ComposeGroup{{}}, Items: []models.ListItem{{IsContainer: true, Container: &models.Container{}}}}
	got, _ = m.Update(enter)
	if toggled := got.(Model).Projects[0].Expanded; toggled || !opened {
		t.Errorf("ports view: toggled=%v opened=%v", toggled, opened)
	}
}
