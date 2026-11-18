package app

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"gdocker/config"
	"gdocker/models"
	"gdocker/ui/logviewer"

	"github.com/charmbracelet/x/ansi"
)

var update = flag.Bool("update", false, "rewrite the screen snapshots in testdata")

// ago masks relative times ("3d ago"), which move with the clock.
var ago = regexp.MustCompile(`\d+[a-z]+ ago|just now`)

// TestScreens renders every view and mode and compares them with the
// snapshot in testdata (go test -update rewrites it).
func TestScreens(t *testing.T) {
	time.Local = time.UTC
	day := time.Date(2024, 5, 1, 12, 0, 0, 0, time.UTC)
	cfg := config.DefaultAppConfig()
	m := &Model{
		KeyBindings: &cfg.KeyBindings, UIConfig: &cfg.UI, Width: 110, Height: 26,
		LogViewer:  logviewer.New(&cfg.KeyBindings.Logs, true, true, nil),
		DockerHost: "ssh://dev",
	}
	m.LogViewer.SetSize(m.DetailWidth(), m.Height-3)
	web := models.Container{Name: "shop-web-1", ID: "0123456789ab", Image: "nginx:1.27", State: "running", Status: "Up 2 hours",
		Project: "shop", Created: day, Ports: []models.PortMapping{{PrivatePort: 80, PublicPort: 8080, Type: "tcp", IP: "0.0.0.0"}},
		Env: []string{"PATH=/usr/bin", "NGINX_VERSION=1.27"}, EnvLoaded: true}
	db := models.Container{Name: "shop-db-1", ID: "ba9876543210", Image: "postgres:16", State: "exited", Status: "Exited (0) 1 hour ago", Project: "shop", Created: day}
	solo := models.Container{Name: "redis", ID: "aaaaaaaaaaaa", Image: "redis:7", State: "running", Status: "Up 5 minutes", Created: day}
	m.Containers = []models.Container{web, db, solo}
	m.Projects = []models.ComposeGroup{{Name: "shop", Containers: []models.Container{web, db}, Expanded: true}}
	m.Standalone = []models.Container{solo}
	m.Items = []models.ListItem{
		{IsProject: true, Project: &m.Projects[0]},
		{IsContainer: true, Container: &m.Projects[0].Containers[0]},
		{IsContainer: true, Container: &m.Projects[0].Containers[1]},
		{IsContainer: true, Container: &m.Standalone[0]},
	}
	m.Volumes = []models.Volume{{Name: "pgdata", Driver: "local", Mountpoint: "/var/lib/docker/volumes/pgdata", Created: day, Scope: "local"}}
	m.Images = []models.Image{{ID: "sha256:abc123def456", RepoTags: []string{"nginx:1.27"}, Size: 192 << 20, Created: day}}
	m.Networks = []models.Network{{ID: "net123456789", Name: "shop_default", Driver: "bridge", Scope: "local", Created: day}}

	var shots []string
	shot := func(name string) {
		screen := ago.ReplaceAllString(ansi.Strip(RenderView(m)), "N ago")
		shots = append(shots, "== "+name+"\n"+screen)
	}
	shot("project selected")
	m.Cursor = 1
	shot("container details")
	m.Status = "Connected to ssh://dev"
	shot("status message")
	m.Status, m.StatusErr = "Failed to load logs: boom", true
	shot("status error")
	m.Status, m.StatusErr = "", false
	m.ViewMode = models.ViewPorts
	shot("ports")
	m.ViewMode = models.ViewEnv
	shot("env")
	m.ViewMode = models.ViewStats
	m.Stats = &models.ContainerStats{CPUPerc: "1.5%", MemUsage: "20MiB / 1GiB", MemPerc: "2%", NetIO: "1kB / 2kB", BlockIO: "0B / 0B", PIDs: "3"}
	shot("stats")
	m.ViewMode = models.ViewInspect
	m.InspectData = "{\n  \"Id\": \"0123456789ab\",\n  \"Name\": \"/shop-web-1\"\n}"
	shot("inspect")
	m.ViewMode = models.ViewLogs
	m.LogViewer.SetLines([]string{"2024-05-01T12:00:00.000000000Z GET / 200", "2024-05-01T12:00:01.000000000Z GET /favicon.ico 404"}, "shop-web-1")
	shot("logs")
	m.ViewMode = models.ViewDetails
	*m = withCommand(*m, "ss")
	shot("command line")
	m.prompt.Close()
	m.Confirm = &Confirm{Prompt: "Delete container shop-web-1? (y/N)"}
	shot("confirm")
	m.Confirm = nil
	m.HelpMode = true
	shot("help")
	m.HelpMode = false
	for _, nav := range []models.NavigationMode{models.NavVolumes, models.NavImages, models.NavNetworks} {
		m.NavMode, m.Cursor = nav, 0
		switch nav {
		case models.NavVolumes:
			m.Items = []models.ListItem{{IsVolume: true, Volume: &m.Volumes[0]}}
		case models.NavImages:
			m.Items = []models.ListItem{{IsImage: true, Image: &m.Images[0]}}
		case models.NavNetworks:
			m.Items = []models.ListItem{{IsNetwork: true, Network: &m.Networks[0]}}
		}
		shot(navModeLabel(nav))
	}

	got := strings.Join(shots, "\n")
	path := filepath.Join("testdata", "screens.txt")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		gl, wl := strings.Split(got, "\n"), strings.Split(string(want), "\n")
		for i := range max(len(gl), len(wl)) {
			var g, w string
			if i < len(gl) {
				g = gl[i]
			}
			if i < len(wl) {
				w = wl[i]
			}
			if g != w {
				t.Errorf("line %d differs:\n got: %q\nwant: %q", i+1, g, w)
			}
		}
	}
}
