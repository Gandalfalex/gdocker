package models

import (
	"testing"
	"time"

	"gdocker/config"
	"gdocker/ui/logviewer"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdateContainersRefreshedSetsRecreatedStatus(t *testing.T) {
	origGroup := GroupByProjectFunc
	origRebuild := RebuildItemsFunc
	t.Cleanup(func() {
		GroupByProjectFunc = origGroup
		RebuildItemsFunc = origRebuild
	})

	GroupByProjectFunc = func(cs []Container) ([]Container, []ComposeGroup) {
		return cs, nil
	}
	RebuildItemsFunc = func(*Model) {}

	m := Model{}
	gotModel, _ := m.Update(ContainersRefreshedMsg{
		Containers: []Container{},
		Recreated: []ContainerRecreated{
			{Name: "api", OldID: "123456789012aaaa", NewID: "abcdef123456bbbb"},
		},
	})

	got := gotModel.(Model)
	want := "Container recreated: api (123456789012 -> abcdef123456)"
	if got.StatusMessage != want {
		t.Fatalf("status message mismatch:\nwant: %q\ngot:  %q", want, got.StatusMessage)
	}
}

func TestUpdateContainersRefreshedTriggersEnvLoadInEnvView(t *testing.T) {
	origGroup := GroupByProjectFunc
	origRebuild := RebuildItemsFunc
	origLoadEnv := LoadEnvFunc
	t.Cleanup(func() {
		GroupByProjectFunc = origGroup
		RebuildItemsFunc = origRebuild
		LoadEnvFunc = origLoadEnv
	})

	GroupByProjectFunc = func(cs []Container) ([]Container, []ComposeGroup) {
		return cs, nil
	}
	RebuildItemsFunc = func(m *Model) {
		if len(m.Containers) == 0 {
			m.Items = nil
			return
		}
		m.Items = []ListItem{{IsContainer: true, Container: &m.Containers[0]}}
	}
	LoadEnvFunc = func(m *Model) tea.Cmd {
		id := ""
		if len(m.Items) > 0 && m.Items[0].Container != nil {
			id = m.Items[0].Container.ID
		}
		return func() tea.Msg {
			return EnvLoadedMsg{ContainerID: id, Env: []string{"A=B"}}
		}
	}

	m := Model{
		ViewMode: ViewEnv,
		Cursor:   0,
	}

	gotModel, cmd := m.Update(ContainersRefreshedMsg{
		Containers: []Container{{ID: "abc", FullID: "abc", Name: "api", EnvLoaded: false}},
	})

	got := gotModel.(Model)
	if cmd == nil {
		t.Fatalf("expected non-nil load env command")
	}
	if got.StatusMessage != "Loading environment variables..." {
		t.Fatalf("unexpected status message: %q", got.StatusMessage)
	}
}

func TestApplyContainerEnvUpdatesAllCollections(t *testing.T) {
	targetID := "full-123"
	m := Model{
		Containers: []Container{
			{ID: "short-a", FullID: targetID},
			{ID: "short-b", FullID: "other"},
		},
		Standalone: []Container{
			{ID: "short-c", FullID: targetID},
		},
		Projects: []ComposeGroup{
			{
				Name: "proj",
				Containers: []Container{
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
		ViewMode:    ViewLogs,
		LogViewer:   logviewer.New(&cfg.KeyBindings.Logs, true, true, nil),
	}

	gotModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	got := gotModel.(Model)

	if !got.CommandMode {
		t.Fatalf("expected command mode to be enabled")
	}
	if got.CommandInput != "" {
		t.Fatalf("expected empty command input, got %q", got.CommandInput)
	}
}

func TestUpdateStopsFollowerWhenToggledOff(t *testing.T) {
	cfg := config.DefaultAppConfig()
	origStop := StopFollowLogsFunc
	t.Cleanup(func() { StopFollowLogsFunc = origStop })

	stopped := false
	StopFollowLogsFunc = func(*Model) { stopped = true }

	m := Model{
		KeyBindings: &cfg.KeyBindings,
		ViewMode:    ViewLogs,
		LogViewer:   logviewer.New(&cfg.KeyBindings.Logs, true, true, nil),
	}
	m.LogViewer.FollowingLogs = true

	gotModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	got := gotModel.(Model)

	if got.LogViewer.FollowingLogs {
		t.Fatalf("expected following to be disabled")
	}
	if !stopped {
		t.Fatalf("expected StopFollowLogsFunc to be called")
	}
}

func TestUpdateLogsLoadedStartsFollowAndSetsContainerName(t *testing.T) {
	origFollow := FollowLogsFunc
	t.Cleanup(func() { FollowLogsFunc = origFollow })

	followCalls := 0
	FollowLogsFunc = func(*Model) tea.Cmd {
		followCalls++
		return func() tea.Msg { return nil }
	}

	since := time.Unix(1700000000, 123)
	c := Container{Name: "api"}
	m := Model{
		Cursor: 0,
		Items: []ListItem{
			{IsContainer: true, Container: &c},
		},
	}

	gotModel, cmd := m.Update(LogsLoadedMsg{
		Lines:  []string{"one", "two"},
		Follow: true,
		Since:  since,
	})
	got := gotModel.(Model)

	if got.ViewMode != ViewLogs {
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
	origFollow := FollowLogsFunc
	t.Cleanup(func() { FollowLogsFunc = origFollow })

	followCalls := 0
	FollowLogsFunc = func(*Model) tea.Cmd {
		followCalls++
		return func() tea.Msg { return nil }
	}

	m := Model{
		ViewMode: ViewLogs,
		LogViewer: logviewer.Model{
			Lines:         []string{"seed"},
			FollowingLogs: true,
		},
	}

	since1 := time.Unix(1700000001, 0)
	gotModel, cmd := m.Update(LogsFollowedMsg{
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
	gotModel2, cmd2 := got.Update(LogsFollowedMsg{
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
	origFollow := FollowLogsFunc
	t.Cleanup(func() { FollowLogsFunc = origFollow })

	followCalls := 0
	FollowLogsFunc = func(*Model) tea.Cmd {
		followCalls++
		return func() tea.Msg { return nil }
	}

	m := Model{
		ViewMode: ViewLogs,
		LogViewer: logviewer.Model{
			Lines:         []string{"seed"},
			FollowingLogs: false,
		},
	}

	gotModel, cmd := m.Update(LogsFollowedMsg{
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
	origConnect := ConnectDockerHostFunc
	origResolver := sshConfigHostEndpointResolver
	t.Cleanup(func() { ConnectDockerHostFunc = origConnect })
	t.Cleanup(func() { sshConfigHostEndpointResolver = origResolver })

	called := false
	var gotHost string
	ConnectDockerHostFunc = func(_ *Model, host string) tea.Cmd {
		called = true
		gotHost = host
		return nil
	}
	sshConfigHostEndpointResolver = func(string) (string, bool) { return "", false }

	m := &Model{
		HostAliases:  map[string]string{"orangepi": "ssh://pi@orangepi"},
		CommandInput: "ssh orangepi",
	}

	_ = executeCommand(m)
	if !called {
		t.Fatalf("expected connect function to be called")
	}
	if gotHost != "ssh://pi@orangepi" {
		t.Fatalf("expected aliased host, got %q", gotHost)
	}
}

func TestExecuteCommandSSHPrefixesBareHost(t *testing.T) {
	origConnect := ConnectDockerHostFunc
	origResolver := sshConfigHostEndpointResolver
	t.Cleanup(func() { ConnectDockerHostFunc = origConnect })
	t.Cleanup(func() { sshConfigHostEndpointResolver = origResolver })

	var gotHost string
	ConnectDockerHostFunc = func(_ *Model, host string) tea.Cmd {
		gotHost = host
		return nil
	}
	sshConfigHostEndpointResolver = func(string) (string, bool) { return "", false }

	m := &Model{CommandInput: "ssh orangepi.local"}
	_ = executeCommand(m)

	if gotHost != "ssh://orangepi.local" {
		t.Fatalf("expected ssh:// prefix, got %q", gotHost)
	}
}

func TestExecuteCommandSSHLocalUsesEmptyHost(t *testing.T) {
	origConnect := ConnectDockerHostFunc
	origResolver := sshConfigHostEndpointResolver
	t.Cleanup(func() { ConnectDockerHostFunc = origConnect })
	t.Cleanup(func() { sshConfigHostEndpointResolver = origResolver })

	var gotHost string
	ConnectDockerHostFunc = func(_ *Model, host string) tea.Cmd {
		gotHost = host
		return nil
	}
	sshConfigHostEndpointResolver = func(string) (string, bool) { return "", false }

	m := &Model{CommandInput: "ssh local"}
	_ = executeCommand(m)

	if gotHost != "" {
		t.Fatalf("expected empty host for local, got %q", gotHost)
	}
}

func TestExecuteCommandSSHPrefersSSHConfigHostAlias(t *testing.T) {
	origConnect := ConnectDockerHostFunc
	origResolver := sshConfigHostEndpointResolver
	t.Cleanup(func() { ConnectDockerHostFunc = origConnect })
	t.Cleanup(func() { sshConfigHostEndpointResolver = origResolver })

	var gotHost string
	ConnectDockerHostFunc = func(_ *Model, host string) tea.Cmd {
		gotHost = host
		return nil
	}
	sshConfigHostEndpointResolver = func(alias string) (string, bool) {
		if alias != "orangepi" {
			return "", false
		}
		return "ssh://orangepi", true
	}

	m := &Model{
		HostAliases:  map[string]string{"orangepi": "ssh://override@other"},
		CommandInput: "ssh orangepi",
	}
	_ = executeCommand(m)

	if gotHost != "ssh://orangepi" {
		t.Fatalf("expected ssh config alias to win, got %q", gotHost)
	}
}

func TestUpdateDockerConnectedMsgRebuildsCurrentNavAndSetsStatus(t *testing.T) {
	origGroup := GroupByProjectFunc
	origRebuildVolumes := RebuildVolumeItemsFunc
	t.Cleanup(func() {
		GroupByProjectFunc = origGroup
		RebuildVolumeItemsFunc = origRebuildVolumes
	})

	GroupByProjectFunc = func(cs []Container) ([]Container, []ComposeGroup) {
		return cs, nil
	}

	rebuiltVolumes := false
	RebuildVolumeItemsFunc = func(m *Model) {
		rebuiltVolumes = true
		m.Items = []ListItem{{IsVolume: true}}
	}

	m := Model{
		NavMode:       NavVolumes,
		ViewMode:      ViewLogs,
		StatusMessage: "old",
	}

	gotModel, _ := m.Update(DockerConnectedMsg{
		Client:     nil,
		Host:       "ssh://pi@orangepi",
		Containers: []Container{{ID: "c1"}},
		Volumes:    []Volume{{Name: "vol1"}},
		Images:     []Image{{ID: "img1"}},
		Networks:   []Network{{ID: "net1"}},
	})
	got := gotModel.(Model)

	if !rebuiltVolumes {
		t.Fatalf("expected volume rebuild for nav mode volumes")
	}
	if got.ViewMode != ViewDetails {
		t.Fatalf("expected view mode details, got %v", got.ViewMode)
	}
	if got.DockerHost != "ssh://pi@orangepi" {
		t.Fatalf("unexpected docker host: %q", got.DockerHost)
	}
	if got.StatusMessage != "Connected to ssh://pi@orangepi" {
		t.Fatalf("unexpected status message: %q", got.StatusMessage)
	}
}

func TestCommandModeTabCompletesSSHAndCyclesMatches(t *testing.T) {
	origListProvider := sshConfigHostAliasListProvider
	t.Cleanup(func() { sshConfigHostAliasListProvider = origListProvider })

	sshConfigHostAliasListProvider = func() []string {
		return []string{"orangepi", "office"}
	}

	m := Model{
		CommandMode:  true,
		CommandInput: "ssh o",
		HostAliases: map[string]string{
			"orbit": "ssh://orbit",
		},
	}

	gotModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := gotModel.(Model)
	if got.CommandInput != "ssh office" {
		t.Fatalf("expected first completion ssh office, got %q", got.CommandInput)
	}

	gotModel, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = gotModel.(Model)
	if got.CommandInput != "ssh orangepi" {
		t.Fatalf("expected second completion ssh orangepi, got %q", got.CommandInput)
	}

	gotModel, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = gotModel.(Model)
	if got.CommandInput != "ssh orbit" {
		t.Fatalf("expected third completion ssh orbit, got %q", got.CommandInput)
	}
}

func TestCommandModeTabResetsCompletionAfterTyping(t *testing.T) {
	origListProvider := sshConfigHostAliasListProvider
	t.Cleanup(func() { sshConfigHostAliasListProvider = origListProvider })

	sshConfigHostAliasListProvider = func() []string {
		return []string{"orangepi", "office"}
	}

	m := Model{
		CommandMode:  true,
		CommandInput: "ssh o",
	}

	gotModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := gotModel.(Model)
	if got.CommandInput != "ssh office" {
		t.Fatalf("expected first completion ssh office, got %q", got.CommandInput)
	}

	gotModel, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got = gotModel.(Model)
	if got.CommandInput != "ssh officex" {
		t.Fatalf("expected typed suffix to append, got %q", got.CommandInput)
	}

	// Now tab should use the new prefix and not cycle old matches.
	gotModel, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = gotModel.(Model)
	if got.CommandInput != "ssh officex" {
		t.Fatalf("expected no completion for unmatched prefix, got %q", got.CommandInput)
	}
}
