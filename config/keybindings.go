package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// AppConfig is the root configuration structure loaded from config.yaml.
type AppConfig struct {
	KeyBindings KeyBindings `yaml:"keybindings"`
	UI          UIConfig    `yaml:"ui"`
}

// KeyBindings holds all configurable key bindings
type KeyBindings struct {
	Navigation NavigationKeys `yaml:"navigation"`
	Container  ContainerKeys  `yaml:"container"`
	Views      ViewKeys       `yaml:"views"`
	Logs       LogKeys        `yaml:"logs"`
	Commands   CommandKeys    `yaml:"commands"`
	General    GeneralKeys    `yaml:"general"`
}

type NavigationKeys struct {
	Up              []string `yaml:"up"`
	Down            []string `yaml:"down"`
	Top             []string `yaml:"top"`
	Bottom          []string `yaml:"bottom"`
	ToggleExpand    []string `yaml:"toggle_expand"`
	SwitchContainer []string `yaml:"switch_container"`
	SwitchVolume    []string `yaml:"switch_volume"`
	SwitchImage     []string `yaml:"switch_image"`
	SwitchNetwork   []string `yaml:"switch_network"`
}

type ContainerKeys struct {
	Restart      []string `yaml:"restart"`
	Delete       []string `yaml:"delete"`
	Logs         []string `yaml:"logs"`
	Exec         []string `yaml:"exec"`
	Ports        []string `yaml:"ports"`
	Env          []string `yaml:"env"`
	Stats        []string `yaml:"stats"`
	Inspect      []string `yaml:"inspect"`
	OpenPort     []string `yaml:"open_port"`
	RefreshStats []string `yaml:"refresh_stats"`
}

type ViewKeys struct {
	Back []string `yaml:"back"`
}

type LogKeys struct {
	Search     []string `yaml:"search"`
	NextResult []string `yaml:"next_result"`
	PrevResult []string `yaml:"prev_result"`
}

type CommandKeys struct {
	Enter []string `yaml:"enter"`
}

type GeneralKeys struct {
	ForceQuit []string `yaml:"force_quit"`
}

// UIConfig holds UX display preferences.
type UIConfig struct {
	ShowHeaderContext       bool `yaml:"show_header_context"`
	ShowListHelpHint        bool `yaml:"show_list_help_hint"`
	ShowLineNumbers         bool `yaml:"show_line_numbers"`
	MaxProjectPreviewItems  int  `yaml:"max_project_preview_items"`
	MaxContainerPortPreview int  `yaml:"max_container_port_preview"`
	MaxImageTagPreview      int  `yaml:"max_image_tag_preview"`
}

// Default returns the default key bindings
func Default() *KeyBindings {
	return &KeyBindings{
		Navigation: NavigationKeys{
			Up:              []string{"k", "up"},
			Down:            []string{"j", "down"},
			Top:             []string{"g"},
			Bottom:          []string{"G"},
			ToggleExpand:    []string{" ", "enter"},
			SwitchContainer: []string{"1"},
			SwitchVolume:    []string{"2"},
			SwitchImage:     []string{"3"},
			SwitchNetwork:   []string{"4"},
		},
		Container: ContainerKeys{
			Restart:      []string{"r"},
			Delete:       []string{"d"},
			Logs:         []string{"l"},
			Exec:         []string{"e"},
			Ports:        []string{"p"},
			Env:          []string{"v"},
			Stats:        []string{"t"},
			Inspect:      []string{"i"},
			OpenPort:     []string{"o", "enter"},
			RefreshStats: []string{"t"},
		},
		Views: ViewKeys{
			Back: []string{"esc"},
		},
		Logs: LogKeys{
			Search:     []string{"?"},
			NextResult: []string{"n"},
			PrevResult: []string{"N"},
		},
		Commands: CommandKeys{
			Enter: []string{":"},
		},
		General: GeneralKeys{
			ForceQuit: []string{"ctrl+c"},
		},
	}
}

// DefaultUI returns the default UX preferences.
func DefaultUI() *UIConfig {
	return &UIConfig{
		ShowHeaderContext:       true,
		ShowListHelpHint:        true,
		ShowLineNumbers:         true,
		MaxProjectPreviewItems:  8,
		MaxContainerPortPreview: 4,
		MaxImageTagPreview:      6,
	}
}

// DefaultAppConfig returns a full default config.
func DefaultAppConfig() *AppConfig {
	return &AppConfig{
		KeyBindings: *Default(),
		UI:          *DefaultUI(),
	}
}

// sanitize applies value bounds for numeric UI options.
func (c *AppConfig) sanitize() {
	if c.UI.MaxProjectPreviewItems < 1 {
		c.UI.MaxProjectPreviewItems = 1
	}
	if c.UI.MaxContainerPortPreview < 1 {
		c.UI.MaxContainerPortPreview = 1
	}
	if c.UI.MaxImageTagPreview < 1 {
		c.UI.MaxImageTagPreview = 1
	}
}

// Load loads app config from a config file, falling back to defaults.
func Load() (*AppConfig, error) {
	// Try to load from ~/.config/gdocker/config.yaml
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return DefaultAppConfig(), nil
	}

	configPath := filepath.Join(homeDir, ".config", "gdocker", "config.yaml")

	// If config doesn't exist, return defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultAppConfig(), nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return DefaultAppConfig(), nil
	}

	// Parse YAML over defaults so missing fields keep sane values.
	cfg := DefaultAppConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return DefaultAppConfig(), err
	}
	cfg.sanitize()
	return cfg, nil
}

// Contains checks if a key is in the list of bindings
func Contains(bindings []string, key string) bool {
	for _, b := range bindings {
		if b == key {
			return true
		}
	}
	return false
}

// SaveDefault saves the default configuration to the config file
func SaveDefault() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(homeDir, ".config", "gdocker")
	configPath := filepath.Join(configDir, "config.yaml")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// Create config structure
	config := DefaultAppConfig()

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(configPath, data, 0644)
}
