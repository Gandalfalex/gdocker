package app

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"gdocker/config"
	"gdocker/models"
	"gdocker/ui/logviewer"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func testModel(width, height int) *Model {
	cfg := config.DefaultAppConfig()
	m := &Model{
		KeyBindings: &cfg.KeyBindings,
		UIConfig:    &cfg.UI,
		Width:       width,
		Height:      height,
		LogViewer:   logviewer.New(&cfg.KeyBindings.Logs, true, true, nil),
	}
	m.LogViewer.SetHeight(height - 2)

	var env []string
	for i := range 60 {
		env = append(env, fmt.Sprintf("VARIABLE_%d=%s", i, strings.Repeat("value ", i)))
	}
	m.Projects = []models.ComposeGroup{{Name: "a-compose-project-with-a-really-long-name-that-cannot-fit", Expanded: true}}
	m.Items = append(m.Items, models.ListItem{IsProject: true, Project: &m.Projects[0]})
	for i := range 40 {
		c := &models.Container{
			Name:      fmt.Sprintf("service-%d-with-a-long-container-name-that-does-not-fit-the-list", i),
			ID:        "0123456789ab",
			Image:     "registry.example.com/team/some-image-with-a-long-name:1.2.3-" + strings.Repeat("x", 80),
			State:     []string{"running", "exited", "paused"}[i%3],
			Project:   []string{"", "p"}[i%2],
			Env:       env,
			EnvLoaded: true,
			Ports:     []models.PortMapping{{PrivatePort: 80, PublicPort: 8080, Type: "tcp"}},
		}
		m.Containers = append(m.Containers, *c)
		m.Items = append(m.Items, models.ListItem{IsContainer: true, Container: c})
	}
	m.Cursor = 3
	return m
}

// TestLayoutFitsTerminal renders every view at several sizes and checks that
// the frame is exactly the terminal size: nothing wraps, nothing overflows.
func TestLayoutFitsTerminal(t *testing.T) {
	var logs []string
	for i := range 200 {
		logs = append(logs, fmt.Sprintf("2026-01-01T00:00:00.000000000Z line %d %s", i, strings.Repeat("log ", i%60)))
	}
	inspect := strings.Repeat("{\n  \"Key\": \""+strings.Repeat("v", 300)+"\"\n}\n", 40)

	views := map[string]func(m *Model){
		"details": func(m *Model) {},
		"project": func(m *Model) { m.Cursor = 0 },
		"logs": func(m *Model) {
			m.ViewMode = models.ViewLogs
			m.LogViewer.SetLines(logs, "svc")
			m.LogViewer.SearchQuery = "log"
		},
		"logs-nowrap": func(m *Model) {
			m.ViewMode = models.ViewLogs
			m.LogViewer.SetLines(logs, "svc")
			m.LogViewer.WrapLogs = false
		},
		"ports":   func(m *Model) { m.ViewMode = models.ViewPorts },
		"env":     func(m *Model) { m.ViewMode = models.ViewEnv; m.EnvScroll = 30 },
		"stats":   func(m *Model) { m.ViewMode = models.ViewStats; m.Stats = &models.ContainerStats{} },
		"inspect": func(m *Model) { m.ViewMode = models.ViewInspect; m.InspectData = inspect; m.InspectScroll = 50 },
		"help":    func(m *Model) { m.HelpMode = true; m.HelpScroll = 5 },
		"confirm": func(m *Model) { m.Confirm = &Confirm{Prompt: "Delete " + strings.Repeat("x", 300) + "?"} },
		"status":  func(m *Model) { m.Status = strings.Repeat("long status\n", 40) },
		"empty":   func(m *Model) { m.Items = nil },
	}
	sizes := [][2]int{{50, 12}, {80, 24}, {121, 37}, {250, 60}}

	for name, setup := range views {
		for _, size := range sizes {
			m := testModel(size[0], size[1])
			setup(m)
			out := RenderView(m)
			if dir := os.Getenv("GDOCKER_DUMP"); dir != "" {
				os.WriteFile(fmt.Sprintf("%s/%s-%dx%d.txt", dir, name, size[0], size[1]), []byte(ansi.Strip(out)), 0o644)
			}
			lines := strings.Split(out, "\n")
			if len(lines) != size[1] {
				t.Errorf("%s %dx%d: %d lines", name, size[0], size[1], len(lines))
			}
			for i, line := range lines {
				if w := lipgloss.Width(line); w > size[0] {
					t.Errorf("%s %dx%d: line %d is %d wide", name, size[0], size[1], i, w)
					break
				}
			}
		}
	}
}

func TestTooSmallTerminal(t *testing.T) {
	if out := ansi.Strip(RenderView(testModel(30, 8))); !strings.Contains(out, "too small") {
		t.Errorf("expected size hint, got %q", out)
	}
}
