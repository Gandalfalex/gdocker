package app

import (
	"testing"
	"time"

	"gdocker/config"
	"gdocker/ui/logviewer"

	tea "charm.land/bubbletea/v2"
	"gdocker/docker"
	"gdocker/models"
)

func TestUpdateContainersRefreshedSetsRecreatedStatus(t *testing.T) {
	m := Model{}
	gotModel, _ := m.Update(models.ContainersRefreshedMsg{
		Containers: []models.Container{},
		Recreated: []models.ContainerRecreated{
			{Name: "api", OldID: "123456789012aaaa", NewID: "abcdef123456bbbb"},
		},
	})

	got := gotModel.(Model)
	want := "Container recreated: api (123456789012 -> abcdef123456)"
	if got.Status != want {
		t.Fatalf("status message mismatch:\nwant: %q\ngot:  %q", want, got.Status)
	}
}

func TestUpdateContainersRefreshedTriggersEnvLoadInEnvView(t *testing.T) {
	fake := &fakeBackend{loadEnv: func(c *models.Container) tea.Cmd {
		return func() tea.Msg { return models.EnvLoadedMsg{ContainerID: c.ID, Env: []string{"A=B"}} }
	}}

	m := Model{
		ViewMode: models.ViewEnv,
		Cursor:   0,
		Backend:  fake,
	}

	gotModel, cmd := m.Update(models.ContainersRefreshedMsg{
		Containers: []models.Container{{ID: "abc", FullID: "abc", Name: "api", EnvLoaded: false}},
	})

	got := gotModel.(Model)
	if cmd == nil {
		t.Fatalf("expected non-nil load env command")
	}
	if got.Status != "Loading environment variables..." {
		t.Fatalf("unexpected status message: %q", got.Status)
	}
}

func TestApplyContainerEnvUpdatesAllCollections(t *testing.T) {
	targetID := "full-123"
	m := Model{
		Containers: []models.Container{
			{ID: "short-a", FullID: targetID},
			{ID: "short-b", FullID: "other"},
		},
		Standalone: []models.Container{
			{ID: "short-c", FullID: targetID},
		},
		Projects: []models.ComposeGroup{
			{
				Name: "proj",
				Containers: []models.Container{
					{ID: "short-d", FullID: targetID},
				},
			},
		},
	}

	wantEnv := []string{"X=1", "Y=2"}
	applyContainerEnv(&m, targetID, wantEnv)

	if !m.Containers[0].EnvLoaded || len(m.Containers[0].Env) != 2 {
		t.Fatalf("containers not updated: %#v", m.Containers[0])
	}
	if !m.Standalone[0].EnvLoaded || len(m.Standalone[0].Env) != 2 {
		t.Fatalf("standalone not updated: %#v", m.Standalone[0])
	}
	if !m.Projects[0].Containers[0].EnvLoaded || len(m.Projects[0].Containers[0].Env) != 2 {
		t.Fatalf("project container not updated: %#v", m.Projects[0].Containers[0])
	}
}

func TestUpdateEnterCommandModeFromLogs(t *testing.T) {
	cfg := config.DefaultAppConfig()
	m := Model{
		KeyBindings: &cfg.KeyBindings,
		ViewMode:    models.ViewLogs,
		LogViewer:   logviewer.New(&cfg.KeyBindings.Logs, true, true, nil),
	}

	gotModel, _ := m.Update(tea.KeyPressMsg{Code: ':', Text: ":"})
	got := gotModel.(Model)

	if !got.commandOpen() {
		t.Fatalf("expected command mode to be enabled")
	}
	if got.prompt.Value() != "" {
		t.Fatalf("expected empty command input, got %q", got.prompt.Value())
	}
}

func TestUpdateStopsFollowerWhenToggledOff(t *testing.T) {
	cfg := config.DefaultAppConfig()
	stopped := false
	m := Model{
		Backend:     &fakeBackend{stopFollow: func() { stopped = true }},
		KeyBindings: &cfg.KeyBindings,
		ViewMode:    models.ViewLogs,
		LogViewer:   logviewer.New(&cfg.KeyBindings.Logs, true, true, nil),
	}
	m.LogViewer.FollowingLogs = true

	gotModel, _ := m.Update(tea.KeyPressMsg{Code: 'f', Text: "f"})
	got := gotModel.(Model)

	if got.LogViewer.FollowingLogs {
		t.Fatalf("expected following to be disabled")
	}
	if !stopped {
		t.Fatalf("expected the follower to be stopped")
	}
}

func TestUpdateLogsLoadedStartsFollowAndSetsContainerName(t *testing.T) {
	followCalls := 0
	fake := &fakeBackend{follow: func(*models.Container, time.Time) tea.Cmd {
		followCalls++
		return func() tea.Msg { return nil }
	}}

	since := time.Unix(1700000000, 123)
	c := models.Container{Name: "api"}
	m := Model{
		Backend:  fake,
		Cursor:   0,
		ViewMode: models.ViewLogs, // set as soon as logs are requested
		Items: []models.ListItem{
			{IsContainer: true, Container: &c},
		},
	}

	gotModel, cmd := m.Update(models.LogsLoadedMsg{
		Lines:  []string{"one", "two"},
		Follow: true,
		Since:  since,
	})
	got := gotModel.(Model)

	if got.ViewMode != models.ViewLogs {
		t.Fatalf("expected view mode logs, got %v", got.ViewMode)
	}
	if got.LogViewer.ContainerName != "api" {
		t.Fatalf("expected container name api, got %q", got.LogViewer.ContainerName)
	}
	if got.LogViewer.ScrollPos != 1 {
		t.Fatalf("expected scroll at last line, got %d", got.LogViewer.ScrollPos)
	}
	if !got.LogSince.Equal(since) {
		t.Fatalf("expected LogSince %v, got %v", since, got.LogSince)
	}
	if cmd == nil {
		t.Fatalf("expected non-nil follow command")
	}
	if followCalls != 1 {
		t.Fatalf("expected one follow call, got %d", followCalls)
	}
}

func TestUpdateLogsFollowedChainsForRapidBurstsWhenFollowing(t *testing.T) {
	followCalls := 0
	fake := &fakeBackend{follow: func(*models.Container, time.Time) tea.Cmd {
		followCalls++
		return func() tea.Msg { return nil }
	}}

	m := Model{
		Backend:  fake,
		ViewMode: models.ViewLogs,
		Items:    []models.ListItem{{IsContainer: true, Container: &models.Container{Name: "api"}}},
		LogViewer: logviewer.Model{
			Lines:         []string{"seed"},
			FollowingLogs: true,
		},
	}

	since1 := time.Unix(1700000001, 0)
	gotModel, cmd := m.Update(models.LogsFollowedMsg{
		Lines: []string{"dup", "dup"},
		Since: since1,
	})
	got := gotModel.(Model)

	if cmd == nil {
		t.Fatalf("expected follow command after first burst")
	}
	if followCalls != 1 {
		t.Fatalf("expected one follow call after first burst, got %d", followCalls)
	}
	if len(got.LogViewer.Lines) != 3 {
		t.Fatalf("expected 3 lines after first burst, got %d", len(got.LogViewer.Lines))
	}
	if !got.LogSince.Equal(since1) {
		t.Fatalf("expected LogSince %v after first burst, got %v", since1, got.LogSince)
	}

	since2 := time.Unix(1700000002, 0)
	gotModel2, cmd2 := got.Update(models.LogsFollowedMsg{
		Lines: []string{"next"},
		Since: since2,
	})
	got2 := gotModel2.(Model)

	if cmd2 == nil {
		t.Fatalf("expected follow command after second burst")
	}
	if followCalls != 2 {
		t.Fatalf("expected two follow calls after second burst, got %d", followCalls)
	}
	if len(got2.LogViewer.Lines) != 4 {
		t.Fatalf("expected 4 lines after second burst, got %d", len(got2.LogViewer.Lines))
	}
	if !got2.LogSince.Equal(since2) {
		t.Fatalf("expected LogSince %v after second burst, got %v", since2, got2.LogSince)
	}
}

func TestUpdateLogsFollowedDoesNotChainWhenFollowDisabled(t *testing.T) {
	followCalls := 0
	fake := &fakeBackend{follow: func(*models.Container, time.Time) tea.Cmd {
		followCalls++
		return func() tea.Msg { return nil }
	}}

	m := Model{
		Backend:  fake,
		ViewMode: models.ViewLogs,
		Items:    []models.ListItem{{IsContainer: true, Container: &models.Container{Name: "api"}}},
		LogViewer: logviewer.Model{
			Lines:         []string{"seed"},
			FollowingLogs: false,
		},
	}

	gotModel, cmd := m.Update(models.LogsFollowedMsg{
		Lines: []string{"one"},
		Since: time.Unix(1700000003, 0),
	})
	got := gotModel.(Model)

	if cmd != nil {
		t.Fatalf("expected no follow command when following is disabled")
	}
	if followCalls != 0 {
		t.Fatalf("expected no follow call, got %d", followCalls)
	}
	if len(got.LogViewer.Lines) != 2 {
		t.Fatalf("expected line append even when follow disabled, got %d lines", len(got.LogViewer.Lines))
	}
}

func TestExecuteCommandSSHResolvesAlias(t *testing.T) {
	origResolver := sshConfigHostEndpointResolver
	t.Cleanup(func() { sshConfigHostEndpointResolver = origResolver })

	called := false
	var gotHost string
	fake := &fakeBackend{connect: func(host string) tea.Cmd {
		called = true
		gotHost = host
		return nil
	}}
	sshConfigHostEndpointResolver = func(string) (string, bool) { return "", false }

	m := &Model{
		Backend:     fake,
		HostAliases: map[string]string{"orangepi": "ssh://pi@orangepi"},
	}

	_ = executeCommand(m, "ssh orangepi")
	if !called {
		t.Fatalf("expected connect function to be called")
	}
	if gotHost != "ssh://pi@orangepi" {
		t.Fatalf("expected aliased host, got %q", gotHost)
	}
}

func TestExecuteCommandSSHPrefixesBareHost(t *testing.T) {
	origResolver := sshConfigHostEndpointResolver
	t.Cleanup(func() { sshConfigHostEndpointResolver = origResolver })

	var gotHost string
	fake := &fakeBackend{connect: func(host string) tea.Cmd {
		gotHost = host
		return nil
	}}
	sshConfigHostEndpointResolver = func(string) (string, bool) { return "", false }

	m := &Model{Backend: fake}
	_ = executeCommand(m, "ssh orangepi.local")

	if gotHost != "ssh://orangepi.local" {
		t.Fatalf("expected ssh:// prefix, got %q", gotHost)
	}
}

func TestExecuteCommandSSHLocalUsesEmptyHost(t *testing.T) {
	origResolver := sshConfigHostEndpointResolver
	t.Cleanup(func() { sshConfigHostEndpointResolver = origResolver })

	var gotHost string
	fake := &fakeBackend{connect: func(host string) tea.Cmd {
		gotHost = host
		return nil
	}}
	sshConfigHostEndpointResolver = func(string) (string, bool) { return "", false }

	m := &Model{Backend: fake}
	_ = executeCommand(m, "ssh local")

	if gotHost != "" {
		t.Fatalf("expected empty host for local, got %q", gotHost)
	}
}

func TestExecuteCommandSSHPrefersSSHConfigHostAlias(t *testing.T) {
	origResolver := sshConfigHostEndpointResolver
	t.Cleanup(func() { sshConfigHostEndpointResolver = origResolver })

	var gotHost string
	fake := &fakeBackend{connect: func(host string) tea.Cmd {
		gotHost = host
		return nil
	}}
	sshConfigHostEndpointResolver = func(alias string) (string, bool) {
		if alias != "orangepi" {
			return "", false
		}
		return "ssh://orangepi", true
	}

	m := &Model{
		Backend:     fake,
		HostAliases: map[string]string{"orangepi": "ssh://override@other"},
	}
	_ = executeCommand(m, "ssh orangepi")

	if gotHost != "ssh://orangepi" {
		t.Fatalf("expected ssh config alias to win, got %q", gotHost)
	}
}

func TestUpdateDockerConnectedMsgRebuildsCurrentNavAndSetsStatus(t *testing.T) {
	m := Model{
		NavMode:  models.NavVolumes,
		ViewMode: models.ViewLogs,
	}
	m.Status = "old"

	gotModel, _ := m.Update(docker.ConnectedMsg{Host: "ssh://pi@orangepi", Snapshot: docker.Snapshot{
		Containers: []models.Container{{ID: "c1"}},
		Volumes:    []models.Volume{{Name: "vol1"}},
		Images:     []models.Image{{ID: "img1"}},
		Networks:   []models.Network{{ID: "net1"}},
	}})
	got := gotModel.(Model)

	if len(got.Items) != 1 || !got.Items[0].IsVolume {
		t.Fatalf("expected the volume list for nav mode volumes, got %+v", got.Items)
	}
	if got.ViewMode != models.ViewDetails {
		t.Fatalf("expected view mode details, got %v", got.ViewMode)
	}
	if got.DockerHost != "ssh://pi@orangepi" {
		t.Fatalf("unexpected docker host: %q", got.DockerHost)
	}
	if got.Status != "Connected to ssh://pi@orangepi" {
		t.Fatalf("unexpected status message: %q", got.Status)
	}
}

func TestCommandModeTabCompletesSSHAndCyclesMatches(t *testing.T) {
	origListProvider := sshConfigHostAliasListProvider
	t.Cleanup(func() { sshConfigHostAliasListProvider = origListProvider })

	sshConfigHostAliasListProvider = func() []string {
		return []string{"orangepi", "office"}
	}

	m := withCommand(Model{
		HostAliases: map[string]string{
			"orbit": "ssh://orbit",
		},
	}, "ssh o")

	gotModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	got := gotModel.(Model)
	if got.prompt.Value() != "ssh office" {
		t.Fatalf("expected first completion ssh office, got %q", got.prompt.Value())
	}

	gotModel, _ = got.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	got = gotModel.(Model)
	if got.prompt.Value() != "ssh orangepi" {
		t.Fatalf("expected second completion ssh orangepi, got %q", got.prompt.Value())
	}

	gotModel, _ = got.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	got = gotModel.(Model)
	if got.prompt.Value() != "ssh orbit" {
		t.Fatalf("expected third completion ssh orbit, got %q", got.prompt.Value())
	}
}

func TestCommandModeTabResetsCompletionAfterTyping(t *testing.T) {
	origListProvider := sshConfigHostAliasListProvider
	t.Cleanup(func() { sshConfigHostAliasListProvider = origListProvider })

	sshConfigHostAliasListProvider = func() []string {
		return []string{"orangepi", "office"}
	}

	m := withCommand(Model{}, "ssh o")

	gotModel, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	got := gotModel.(Model)
	if got.prompt.Value() != "ssh office" {
		t.Fatalf("expected first completion ssh office, got %q", got.prompt.Value())
	}

	gotModel, _ = got.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	got = gotModel.(Model)
	if got.prompt.Value() != "ssh officex" {
		t.Fatalf("expected typed suffix to append, got %q", got.prompt.Value())
	}

	// Now tab should use the new prefix and not cycle old matches.
	gotModel, _ = got.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	got = gotModel.(Model)
	if got.prompt.Value() != "ssh officex" {
		t.Fatalf("expected no completion for unmatched prefix, got %q", got.prompt.Value())
	}
}
