package models

import (
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

	// Command and search handlers
	for _, key := range kb.Commands.Enter {
		handlers[key] = handleCommandMode
	}
	for _, key := range kb.Logs.Search {
		handlers[key] = handleSearch
	}
	for _, key := range kb.Logs.NextResult {
		handlers[key] = handleNextSearchResult
	}
	for _, key := range kb.Logs.PrevResult {
		handlers[key] = handlePrevSearchResult
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
	case ViewLogs, ViewInspect:
		if m.LogScroll > 0 {
			m.LogScroll--
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
	case ViewLogs, ViewInspect:
		maxLines := len(m.Logs)
		if m.ViewMode == ViewInspect {
			maxLines = len(strings.Split(m.InspectData, "\n"))
		}
		if m.LogScroll < maxLines-1 {
			m.LogScroll++
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
	if m.ViewMode == ViewLogs || m.ViewMode == ViewInspect {
		m.LogScroll = 0
	} else {
		m.Cursor = 0
	}
	return *m, nil
}

func handleNavigationBottom(m *Model) (Model, tea.Cmd) {
	if m.ViewMode == ViewLogs || m.ViewMode == ViewInspect {
		maxLines := len(m.Logs) - 1
		if m.ViewMode == ViewInspect {
			maxLines = len(strings.Split(m.InspectData, "\n")) - 1
		}
		m.LogScroll = maxLines
	} else {
		m.Cursor = len(m.Items) - 1
	}
	return *m, nil
}

func handleToggleExpand(m *Model) (Model, tea.Cmd) {
	// Toggle project expansion
	if m.ViewMode == ViewDetails && m.Cursor < len(m.Items) && m.Items[m.Cursor].IsProject {
		idx := m.Items[m.Cursor].Index
		m.Projects[idx].Expanded = !m.Projects[idx].Expanded
		RebuildItemsFunc(m)
	}
	// Note: "enter" key in ports view is handled separately by handleOpenPort
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

// Command and search handlers

func handleCommandMode(m *Model) (Model, tea.Cmd) {
	m.CommandMode = true
	m.CommandInput = ""
	m.StatusMessage = ""
	return *m, nil
}

func handleSearch(m *Model) (Model, tea.Cmd) {
	if m.ViewMode == ViewLogs {
		m.SearchMode = true
		m.SearchQuery = ""
		m.StatusMessage = ""
	}
	return *m, nil
}

func handleNextSearchResult(m *Model) (Model, tea.Cmd) {
	if m.ViewMode == ViewLogs && len(m.SearchResults) > 0 {
		m.SearchResultIdx++
		if m.SearchResultIdx >= len(m.SearchResults) {
			m.SearchResultIdx = 0
		}
		m.LogScroll = m.SearchResults[m.SearchResultIdx]
		m.StatusMessage = formatSearchStatus(m)
	}
	return *m, nil
}

func handlePrevSearchResult(m *Model) (Model, tea.Cmd) {
	if m.ViewMode == ViewLogs && len(m.SearchResults) > 0 {
		m.SearchResultIdx--
		if m.SearchResultIdx < 0 {
			m.SearchResultIdx = len(m.SearchResults) - 1
		}
		m.LogScroll = m.SearchResults[m.SearchResultIdx]
		m.StatusMessage = formatSearchStatus(m)
	}
	return *m, nil
}

// View handlers

func handleBack(m *Model) (Model, tea.Cmd) {
	if m.HelpMode {
		m.HelpMode = false
		m.StatusMessage = ""
	} else if m.ViewMode == ViewLogs || m.ViewMode == ViewPorts || m.ViewMode == ViewEnv || m.ViewMode == ViewStats || m.ViewMode == ViewInspect {
		m.ViewMode = ViewDetails
		m.Logs = nil
		m.SelectedPort = 0
		m.SearchQuery = ""
		m.SearchResults = nil
		m.SearchResultIdx = 0
		m.Stats = nil
		m.InspectData = ""
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

// CommandHandler is a function that executes a command
type CommandHandler func(*Model) tea.Cmd

// buildCommandHandlerMap creates a map of command -> handler function
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
	m.SearchQuery = ""
	m.SearchResults = nil
	m.SearchResultIdx = 0
	m.StatusMessage = "Search cleared"
	return nil
}

func cmdHelp(m *Model) tea.Cmd {
	m.HelpMode = true
	m.StatusMessage = ""
	return nil
}
