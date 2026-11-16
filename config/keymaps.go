package config

import (
	"github.com/Gandalfalex/vim-kit/keys"
	"github.com/Gandalfalex/vim-kit/motion"
)

// Actions without a config field of their own.
const (
	ActExit keys.Action = "exit" // leave the log viewer
)

// motionDefaults are the vim motions every scrolling view supports: cursor
// (j, ctrl+d, gg), screen-relative (H/M/L) and view (zz, ctrl+e) motions.
func motionDefaults() keys.Bindings {
	return motion.VerticalBindings()
}

// mainBindings returns the bindings for the resource list and its detail
// views. Action names are the yaml tags, so navigation up/down/top/bottom
// override the matching vimkit motions.
func (kb *KeyBindings) mainBindings() keys.Bindings {
	b := keys.FromStruct(struct {
		NavigationKeys
		ContainerKeys
		ViewKeys
		CommandKeys
		GeneralKeys
	}{kb.Navigation, kb.Container, kb.Views, kb.Commands, kb.General}, "yaml")
	// refresh_stats runs the same handler as stats; fold it in so sharing a
	// key isn't a conflict.
	b["stats"] = append(b["stats"], b["refresh_stats"]...)
	delete(b, "refresh_stats")
	delete(b, "open_port") // ports view only
	return b
}

// MainKeymap is the keymap for the list and detail views.
func (kb *KeyBindings) MainKeymap() (*keys.Keymap, error) {
	return keys.NewKeymap(motionDefaults(), kb.mainBindings())
}

// PortsKeymap is MainKeymap plus open_port, which replaces toggle_expand
// (both default to enter; expanding only applies to the list).
func (kb *KeyBindings) PortsKeymap() (*keys.Keymap, error) {
	return keys.NewKeymap(motionDefaults(), kb.mainBindings(), keys.Bindings{
		"toggle_expand": {},
		"open_port":     kb.Container.OpenPort,
	})
}

// LogNormalKeymap is the log viewer keymap outside visual mode.
func (k *LogKeys) LogNormalKeymap() (*keys.Keymap, error) {
	return keys.NewKeymap(motionDefaults(), keys.FromStruct(k, "yaml"), keys.Bindings{ActExit: {"esc"}})
}

// LogVisualKeymap is the log viewer keymap in visual line (V) or, with
// horizontal motions, visual char (v) mode.
func (k *LogKeys) LogVisualKeymap(char bool) (*keys.Keymap, error) {
	layers := []keys.Bindings{motionDefaults()}
	if char {
		layers = append(layers, motion.HorizontalBindings())
	}
	layers = append(layers, keys.FromStruct(k, "yaml").Only("yank", "visual_line", "char_visual"),
		keys.Bindings{ActExit: {"esc"}})
	return keys.NewKeymap(layers...)
}

// Validate reports key conflicts in any keymap built from kb.
func (kb *KeyBindings) Validate() error {
	for _, build := range []func() (*keys.Keymap, error){
		kb.MainKeymap, kb.PortsKeymap, kb.Logs.LogNormalKeymap,
		func() (*keys.Keymap, error) { return kb.Logs.LogVisualKeymap(false) },
		func() (*keys.Keymap, error) { return kb.Logs.LogVisualKeymap(true) },
	} {
		if _, err := build(); err != nil {
			return err
		}
	}
	return nil
}
