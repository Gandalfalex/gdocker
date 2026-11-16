package config

import "testing"

func TestDefaultKeymapsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestLegacySingleGTop(t *testing.T) {
	kb := Default()
	kb.Navigation.Top = []string{"g"} // pre-vimkit default in user configs
	km, err := kb.MainKeymap()
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := km.Lookup("g"); a != "top" {
		t.Errorf("g = %q, want top", a)
	}
}

func TestConflictReported(t *testing.T) {
	kb := Default()
	kb.Container.Restart = []string{"d"} // also delete
	if err := kb.Validate(); err == nil {
		t.Error("expected conflict error")
	}
}

func TestLegacyKeySpellingsNormalized(t *testing.T) {
	cfg := DefaultAppConfig()
	cfg.KeyBindings.Navigation.ToggleExpand = []string{" ", "Enter"} // pre-v2 config
	cfg.KeyBindings.General.ForceQuit = []string{"Ctrl+C"}
	cfg.sanitize()
	if got := cfg.KeyBindings.Navigation.ToggleExpand; got[0] != "space" || got[1] != "enter" {
		t.Errorf("toggle_expand = %q", got)
	}
	if got := cfg.KeyBindings.General.ForceQuit[0]; got != "ctrl+c" {
		t.Errorf("force_quit = %q", got)
	}
}
