package models

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// KeyHandler is a function that handles a key press
type KeyHandler func(*Model) (Model, tea.Cmd)

// buildKeyHandlerMap creates a map of key -> handler function based on keybindings config
func buildKeyHandlerMap(m *Model) map[string]KeyHandler {
	handlers := make(map[string]KeyHandler)
	kb := m.KeyBindings

	// Navigation handlers
	for _, key := range kb.Navigation.Up {
		handlers[key] = handleNavigationUp
	}
	for _, key := range kb.Navigation.Down {
		handlers[key] = handleNavigationDown
	}
	for _, key := range kb.Navigation.Top {
		handlers[key] = handleNavigationTop
	}
	for _, key := range kb.Navigation.Bottom {
		handlers[key] = handleNavigationBottom
	}
	for _, key := range kb.Navigation.ToggleExpand {
		handlers[key] = handleToggleExpand
	}
	for _, key := range kb.Navigation.SwitchContainer {
		handlers[key] = handleSwitchContainer
	}
	for _, key := range kb.Navigation.SwitchVolume {
		handlers[key] = handleSwitchVolume
	}
	for _, key := range kb.Navigation.SwitchImage {
		handlers[key] = handleSwitchImage
	}
	for _, key := range kb.Navigation.SwitchNetwork {
		handlers[key] = handleSwitchNetwork
	}

	// Container action handlers
	for _, key := range kb.Container.Restart {
		handlers[key] = handleRestart
	}
	for _, key := range kb.Container.Delete {
		handlers[key] = handleDelete
	}
	for _, key := range kb.Container.Logs {
		handlers[key] = handleLogs
	}
	for _, key := range kb.Container.Exec {
		handlers[key] = handleExec
	}
	for _, key := range kb.Container.Ports {
		handlers[key] = handlePorts
	}
	for _, key := range kb.Container.Env {
		handlers[key] = handleEnv
	}
	for _, key := range kb.Container.Stats {
		handlers[key] = handleStats
	}
	for _, key := range kb.Container.RefreshStats {
		handlers[key] = handleStats
	}
	for _, key := range kb.Container.Inspect {
		handlers[key] = handleInspect
	}
	for _, key := range kb.Container.OpenPort {
		handlers[key] = handleOpenPort
	}

	// Command handler
	for _, key := range kb.Commands.Enter {
		handlers[key] = handleCommandMode
	}

	// View handlers
	for _, key := range kb.Views.Back {
		handlers[key] = handleBack
	}

	// General handlers
	for _, key := range kb.General.ForceQuit {
		handlers[key] = handleForceQuit
	}

	return handlers
}

// Navigation handlers

func handleNavigationUp(m *Model) (Model, tea.Cmd) {
	switch m.ViewMode {
	case ViewInspect:
		if m.InspectScroll > 0 {
			m.InspectScroll--
		}
	case ViewPorts:
		if m.SelectedPort > 0 {
			m.SelectedPort--
		}
	default:
		if m.Cursor > 0 {
			m.Cursor--
		}
	}
	return *m, nil
}

func handleNavigationDown(m *Model) (Model, tea.Cmd) {
	switch m.ViewMode {
	case ViewInspect:
		maxLines := len(strings.Split(m.InspectData, "\n"))
		if m.InspectScroll < maxLines-1 {
			m.InspectScroll++
		}
	case ViewPorts:
		if m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
			if m.SelectedPort < len(m.Items[m.Cursor].Container.Ports)-1 {
				m.SelectedPort++
			}
		}
	default:
		if m.Cursor < len(m.Items)-1 {
			m.Cursor++
		}
	}
	return *m, nil
}

func handleNavigationTop(m *Model) (Model, tea.Cmd) {
	if m.ViewMode == ViewInspect {
		m.InspectScroll = 0
	} else {
		m.Cursor = 0
	}
	return *m, nil
}

func handleNavigationBottom(m *Model) (Model, tea.Cmd) {
	if m.ViewMode == ViewInspect {
		m.InspectScroll = len(strings.Split(m.InspectData, "\n")) - 1
	} else {
		m.Cursor = len(m.Items) - 1
	}
	return *m, nil
}

func handleToggleExpand(m *Model) (Model, tea.Cmd) {
	if m.ViewMode == ViewDetails && m.Cursor < len(m.Items) && m.Items[m.Cursor].IsProject {
		idx := m.Items[m.Cursor].Index
		m.Projects[idx].Expanded = !m.Projects[idx].Expanded
		RebuildItemsFunc(m)
	}
	return *m, nil
}

func handleSwitchContainer(m *Model) (Model, tea.Cmd) {
	if m.NavMode != NavContainers {
		m.NavMode = NavContainers
		m.ViewMode = ViewDetails
		m.Cursor = 0
		RebuildItemsFunc(m)
	}
	return *m, nil
}

func handleSwitchVolume(m *Model) (Model, tea.Cmd) {
	if m.NavMode != NavVolumes {
		m.NavMode = NavVolumes
		m.ViewMode = ViewDetails
		m.Cursor = 0
		RebuildVolumeItemsFunc(m)
	}
	return *m, nil
}

func handleSwitchImage(m *Model) (Model, tea.Cmd) {
	if m.NavMode != NavImages {
		m.NavMode = NavImages
		m.ViewMode = ViewDetails
		m.Cursor = 0
		RebuildImageItemsFunc(m)
	}
	return *m, nil
}

func handleSwitchNetwork(m *Model) (Model, tea.Cmd) {
	if m.NavMode != NavNetworks {
		m.NavMode = NavNetworks
		m.ViewMode = ViewDetails
		m.Cursor = 0
		RebuildNetworkItemsFunc(m)
	}
	return *m, nil
}

// Container action handlers

func handleRestart(m *Model) (Model, tea.Cmd) {
	return *m, RestartContainerFunc(m)
}

func handleDelete(m *Model) (Model, tea.Cmd) {
	switch m.NavMode {
	case NavContainers:
		return *m, DeleteContainerFunc(m)
	case NavVolumes:
		return *m, DeleteVolumeFunc(m)
	case NavImages:
		return *m, DeleteImageFunc(m)
	}
	return *m, nil
}

func handleLogs(m *Model) (Model, tea.Cmd) {
	return *m, LoadLogsFunc(m)
}

func handleExec(m *Model) (Model, tea.Cmd) {
	if m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
		return *m, ExecShellFunc(m)
	}
	return *m, nil
}

func handlePorts(m *Model) (Model, tea.Cmd) {
	if m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
		m.ViewMode = ViewPorts
		m.SelectedPort = 0
	}
	return *m, nil
}

func handleEnv(m *Model) (Model, tea.Cmd) {
	if m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
		m.ViewMode = ViewEnv
		if !m.Items[m.Cursor].Container.EnvLoaded && LoadEnvFunc != nil {
			m.StatusMessage = "Loading environment variables..."
			return *m, LoadEnvFunc(m)
		}
	}
	return *m, nil
}

func handleStats(m *Model) (Model, tea.Cmd) {
	if m.ViewMode == ViewStats {
		return *m, LoadStatsFunc(m)
	} else if m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
		return *m, LoadStatsFunc(m)
	}
	return *m, nil
}

func handleInspect(m *Model) (Model, tea.Cmd) {
	if m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
		return *m, LoadInspectFunc(m)
	}
	return *m, nil
}

func handleOpenPort(m *Model) (Model, tea.Cmd) {
	if m.ViewMode == ViewPorts && m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
		return *m, OpenPortInBrowserFunc(m)
	}
	return *m, nil
}

// Command and view handlers

func handleCommandMode(m *Model) (Model, tea.Cmd) {
	m.CommandMode = true
	m.CommandInput = ""
	m.StatusMessage = ""
	return *m, nil
}

func handleBack(m *Model) (Model, tea.Cmd) {
	if m.HelpMode {
		m.HelpMode = false
		m.StatusMessage = ""
	} else if m.ViewMode == ViewPorts || m.ViewMode == ViewEnv || m.ViewMode == ViewStats || m.ViewMode == ViewInspect {
		m.ViewMode = ViewDetails
		m.SelectedPort = 0
		m.Stats = nil
		m.InspectData = ""
		m.InspectScroll = 0
		m.StatusMessage = ""
	}
	return *m, nil
}

// General handlers

func handleForceQuit(m *Model) (Model, tea.Cmd) {
	QuitFunc(m)
	return *m, tea.Quit
}

// Command handlers

type CommandHandler func(*Model) tea.Cmd

func buildCommandHandlerMap() map[string]CommandHandler {
	return map[string]CommandHandler{
		"q":     cmdQuit,
		"quit":  cmdQuit,
		"s":     cmdStart,
		"start": cmdStart,
		"S":     cmdStop,
		"stop":  cmdStop,
		"noh":   cmdNoHighlight,
		"help":  cmdHelp,
		"h":     cmdHelp,
	}
}

func cmdQuit(m *Model) tea.Cmd {
	QuitFunc(m)
	return tea.Quit
}

func cmdStart(m *Model) tea.Cmd {
	m.StatusMessage = "Starting container..."
	return StartContainerFunc(m)
}

func cmdStop(m *Model) tea.Cmd {
	m.StatusMessage = "Stopping container..."
	return StopContainerFunc(m)
}

func cmdNoHighlight(m *Model) tea.Cmd {
	m.LogViewer.SearchQuery = ""
	m.LogViewer.SearchResults = nil
	m.LogViewer.SearchResultIdx = 0
	m.StatusMessage = "Search cleared"
	return nil
}

func cmdHelp(m *Model) tea.Cmd {
	m.HelpMode = true
	m.StatusMessage = ""
	return nil
}

func cmdSSH(m *Model, args []string) tea.Cmd {
	if len(args) < 1 {
		m.StatusMessage = "Usage: :ssh <alias|host|local>"
		return nil
	}
	if ConnectDockerHostFunc == nil {
		m.StatusMessage = "SSH connection command is not available"
		return nil
	}

	target := strings.TrimSpace(args[0])
	if target == "" {
		m.StatusMessage = "Usage: :ssh <alias|host|local>"
		return nil
	}

	host := ""
	label := "local Docker"
	if target != "local" {
		if !strings.Contains(target, "://") {
			// Prefer ~/.ssh/config host aliases first so SSH options
			// (HostName, User, Port) are respected.
			if endpoint, ok := sshConfigHostEndpointResolver(target); ok {
				target = endpoint
			} else if alias, ok := m.HostAliases[target]; ok {
				target = alias
			}
			if !strings.Contains(target, "://") {
				target = "ssh://" + target
			}
		}
		host = target
		label = host
	}

	m.StatusMessage = "Connecting to " + label + "..."
	return ConnectDockerHostFunc(m, host)
}

var sshConfigHostEndpointResolver = resolveSSHConfigHostEndpoint
var sshConfigHostAliasListProvider = listSSHConfigHostAliases

func resolveSSHConfigHostEndpoint(alias string) (string, bool) {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return "", false
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	configPath := filepath.Join(homeDir, ".ssh", "config")

	visited := make(map[string]bool)
	if !hasSSHConfigHostAliasInFile(alias, configPath, visited) {
		return "", false
	}

	// Keep alias verbatim so OpenSSH can apply full config
	// (IdentityFile, ProxyJump, HostName, User, Port, etc.).
	return "ssh://" + alias, true
}

func hasSSHConfigHostAliasInFile(alias, configPath string, visited map[string]bool) bool {
	configPath = filepath.Clean(configPath)
	if visited[configPath] {
		return false
	}
	visited[configPath] = true

	f, err := os.Open(configPath)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	baseDir := filepath.Dir(configPath)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		switch {
		case strings.EqualFold(fields[0], "Include"):
			for _, includePattern := range fields[1:] {
				includePattern = strings.Trim(includePattern, "\"'")
				if includePattern == "" {
					continue
				}
				if !filepath.IsAbs(includePattern) {
					includePattern = filepath.Join(baseDir, includePattern)
				}
				matches, err := filepath.Glob(includePattern)
				if err != nil {
					continue
				}
				for _, includePath := range matches {
					if hasSSHConfigHostAliasInFile(alias, includePath, visited) {
						return true
					}
				}
			}
		case strings.EqualFold(fields[0], "Host"):
			if hostPatternsMatchAlias(fields[1:], alias) {
				return true
			}
		}
	}

	return false
}

func hasSSHConfigHostAlias(alias string) bool {
	_, ok := resolveSSHConfigHostEndpoint(alias)
	return ok
}

func getSSHCompletionCandidates(m *Model) []string {
	seen := make(map[string]struct{})

	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		seen[v] = struct{}{}
	}

	add("local")
	for alias := range m.HostAliases {
		add(alias)
	}
	for _, alias := range sshConfigHostAliasListProvider() {
		add(alias)
	}

	candidates := make([]string, 0, len(seen))
	for alias := range seen {
		candidates = append(candidates, alias)
	}
	return candidates
}

func listSSHConfigHostAliases() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	configPath := filepath.Join(homeDir, ".ssh", "config")
	visited := make(map[string]bool)
	set := make(map[string]struct{})
	collectSSHConfigHostAliasesFromFile(configPath, visited, set)

	out := make([]string, 0, len(set))
	for alias := range set {
		out = append(out, alias)
	}
	return out
}

func collectSSHConfigHostAliasesFromFile(configPath string, visited map[string]bool, set map[string]struct{}) {
	configPath = filepath.Clean(configPath)
	if visited[configPath] {
		return
	}
	visited[configPath] = true

	f, err := os.Open(configPath)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	baseDir := filepath.Dir(configPath)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		switch {
		case strings.EqualFold(fields[0], "Include"):
			for _, includePattern := range fields[1:] {
				includePattern = strings.Trim(includePattern, "\"'")
				if includePattern == "" {
					continue
				}
				if !filepath.IsAbs(includePattern) {
					includePattern = filepath.Join(baseDir, includePattern)
				}
				matches, err := filepath.Glob(includePattern)
				if err != nil {
					continue
				}
				for _, includePath := range matches {
					collectSSHConfigHostAliasesFromFile(includePath, visited, set)
				}
			}
		case strings.EqualFold(fields[0], "Host"):
			for _, raw := range fields[1:] {
				p := strings.Trim(strings.TrimSpace(raw), "\"'")
				if p == "" || strings.HasPrefix(p, "!") || strings.ContainsAny(p, "*?") {
					continue
				}
				set[p] = struct{}{}
			}
		}
	}
}

func hostPatternsMatchAlias(patterns []string, alias string) bool {
	aliasLower := strings.ToLower(alias)
	positiveMatch := false

	for _, raw := range patterns {
		p := strings.Trim(strings.TrimSpace(raw), "\"'")
		if p == "" {
			continue
		}
		negated := strings.HasPrefix(p, "!")
		if negated {
			p = p[1:]
		}
		if p == "" {
			continue
		}

		matched := false
		if strings.ContainsAny(p, "*?") {
			ok, err := path.Match(strings.ToLower(p), aliasLower)
			if err == nil {
				matched = ok
			}
		} else if strings.EqualFold(p, alias) {
			matched = true
		}

		if matched && negated {
			return false
		}
		if matched {
			positiveMatch = true
		}
	}

	return positiveMatch
}
