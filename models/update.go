package models

import (
	"fmt"
	"gdocker/config"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// These will be set by the docker and ui packages to avoid circular imports
var (
	GroupByProjectFunc      func([]Container) ([]Container, []ComposeGroup)
	RebuildItemsFunc        func(*Model)
	RebuildVolumeItemsFunc  func(*Model)
	RebuildImageItemsFunc   func(*Model)
	RebuildNetworkItemsFunc func(*Model)
	RefreshContainersFunc   func(*Model) tea.Cmd
	StartContainerFunc      func(*Model) tea.Cmd
	StopContainerFunc       func(*Model) tea.Cmd
	RestartContainerFunc    func(*Model) tea.Cmd
	DeleteContainerFunc     func(*Model) tea.Cmd
	DeleteVolumeFunc        func(*Model) tea.Cmd
	DeleteImageFunc         func(*Model) tea.Cmd
	LoadLogsFunc            func(*Model) tea.Cmd
	LoadInspectFunc         func(*Model) tea.Cmd
	LoadStatsFunc           func(*Model) tea.Cmd
	ExecShellFunc           func(*Model) tea.Cmd
	OpenPortInBrowserFunc   func(*Model) tea.Cmd
	QuitFunc                func(*Model)
	RenderViewFunc          func(*Model) string
)

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case ContainersRefreshedMsg:
		// Save expanded state
		expandedProjects := make(map[string]bool)
		for _, p := range m.Projects {
			if p.Expanded {
				expandedProjects[p.Name] = true
			}
		}

		// Refresh containers
		m.Containers = msg.Containers
		m.Standalone, m.Projects = GroupByProjectFunc(msg.Containers)

		// Restore expanded state
		for i := range m.Projects {
			if expandedProjects[m.Projects[i].Name] {
				m.Projects[i].Expanded = true
			}
		}

		RebuildItemsFunc(&m)
		return m, nil

	case LogsLoadedMsg:
		m.Logs = msg.Lines
		m.LogScroll = len(msg.Lines) - 1 // Scroll to bottom
		m.ViewMode = ViewLogs
		return m, nil

	case StatsLoadedMsg:
		m.Stats = msg.Stats
		m.ViewMode = ViewStats
		return m, nil

	case ActionResultMsg:
		m.StatusMessage = msg.Message
		if msg.Success {
			// Refresh based on navigation mode
			switch m.NavMode {
			case NavContainers:
				return m, RefreshContainersFunc(&m)
			case NavVolumes:
				RebuildVolumeItemsFunc(&m)
			case NavImages:
				RebuildImageItemsFunc(&m)
			}
		}
		return m, nil

	case VolumesLoadedMsg:
		m.Volumes = msg.Volumes
		RebuildVolumeItemsFunc(&m)
		m.StatusMessage = "Volume deleted"
		return m, nil

	case ImagesLoadedMsg:
		m.Images = msg.Images
		RebuildImageItemsFunc(&m)
		m.StatusMessage = "Image deleted"
		return m, nil

	case InspectLoadedMsg:
		m.InspectData = msg.Data
		m.ViewMode = ViewInspect
		m.LogScroll = 0
		return m, nil

	case tea.KeyMsg:
		// Handle command mode input
		if m.CommandMode {
			switch msg.String() {
			case "enter":
				// Execute command
				m.CommandMode = false
				return m, executeCommand(&m)
			case "esc":
				// Cancel command
				m.CommandMode = false
				m.CommandInput = ""
				m.StatusMessage = ""
				return m, nil
			case "backspace":
				if len(m.CommandInput) > 0 {
					m.CommandInput = m.CommandInput[:len(m.CommandInput)-1]
				}
				return m, nil
			default:
				// Add character to command input
				if len(msg.String()) == 1 {
					m.CommandInput += msg.String()
				}
				return m, nil
			}
		}

		// Handle search mode input
		if m.SearchMode {
			switch msg.String() {
			case "enter":
				// Execute search
				m.SearchMode = false
				performSearch(&m)
				return m, nil
			case "esc":
				// Cancel search
				m.SearchMode = false
				m.SearchQuery = ""
				m.StatusMessage = ""
				return m, nil
			case "backspace":
				if len(m.SearchQuery) > 0 {
					m.SearchQuery = m.SearchQuery[:len(m.SearchQuery)-1]
				}
				return m, nil
			default:
				// Add character to search query
				if len(msg.String()) == 1 {
					m.SearchQuery += msg.String()
				}
				return m, nil
			}
		}

		key := msg.String()
		kb := m.KeyBindings

		// Handle general keys
		if config.Contains(kb.General.ForceQuit, key) {
			QuitFunc(&m)
			return m, tea.Quit
		}

		// Handle back/escape
		if config.Contains(kb.Views.Back, key) {
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
			return m, nil
		}

		// Handle navigation - down
		if config.Contains(kb.Navigation.Down, key) {
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
			return m, nil
		}

		// Handle navigation - up
		if config.Contains(kb.Navigation.Up, key) {
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
			return m, nil
		}

		// Handle navigation - top
		if config.Contains(kb.Navigation.Top, key) {
			if m.ViewMode == ViewLogs || m.ViewMode == ViewInspect {
				m.LogScroll = 0
			} else {
				m.Cursor = 0
			}
			return m, nil
		}

		// Handle navigation - bottom
		if config.Contains(kb.Navigation.Bottom, key) {
			if m.ViewMode == ViewLogs || m.ViewMode == ViewInspect {
				maxLines := len(m.Logs) - 1
				if m.ViewMode == ViewInspect {
					maxLines = len(strings.Split(m.InspectData, "\n")) - 1
				}
				m.LogScroll = maxLines
			} else {
				m.Cursor = len(m.Items) - 1
			}
			return m, nil
		}

		// Handle toggle expand
		if config.Contains(kb.Navigation.ToggleExpand, key) {
			// Toggle project expansion
			if m.ViewMode == ViewDetails && m.Cursor < len(m.Items) && m.Items[m.Cursor].IsProject {
				idx := m.Items[m.Cursor].Index
				m.Projects[idx].Expanded = !m.Projects[idx].Expanded
				RebuildItemsFunc(&m)
			}
			// In ports view, "enter" opens the port
			if key == "enter" && m.ViewMode == ViewPorts && m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
				return m, OpenPortInBrowserFunc(&m)
			}
			return m, nil
		}

		// Handle container actions
		if config.Contains(kb.Container.Restart, key) {
			return m, RestartContainerFunc(&m)
		}

		if config.Contains(kb.Container.Delete, key) {
			switch m.NavMode {
			case NavContainers:
				return m, DeleteContainerFunc(&m)
			case NavVolumes:
				return m, DeleteVolumeFunc(&m)
			case NavImages:
				return m, DeleteImageFunc(&m)
			}
			return m, nil
		}

		if config.Contains(kb.Container.Logs, key) {
			return m, LoadLogsFunc(&m)
		}

		if config.Contains(kb.Container.Exec, key) {
			if m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
				return m, ExecShellFunc(&m)
			}
			return m, nil
		}

		if config.Contains(kb.Container.Ports, key) {
			if m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
				m.ViewMode = ViewPorts
				m.SelectedPort = 0
			}
			return m, nil
		}

		if config.Contains(kb.Container.Env, key) {
			if m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
				m.ViewMode = ViewEnv
			}
			return m, nil
		}

		if config.Contains(kb.Container.Stats, key) || config.Contains(kb.Container.RefreshStats, key) {
			if m.ViewMode == ViewStats {
				return m, LoadStatsFunc(&m)
			} else if m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
				return m, LoadStatsFunc(&m)
			}
			return m, nil
		}

		if config.Contains(kb.Container.Inspect, key) {
			if m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
				return m, LoadInspectFunc(&m)
			}
			return m, nil
		}

		if config.Contains(kb.Container.OpenPort, key) {
			if m.ViewMode == ViewPorts && m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
				return m, OpenPortInBrowserFunc(&m)
			}
			return m, nil
		}

		// Handle command mode
		if config.Contains(kb.Commands.Enter, key) {
			m.CommandMode = true
			m.CommandInput = ""
			m.StatusMessage = ""
			return m, nil
		}

		// Handle search
		if config.Contains(kb.Logs.Search, key) {
			if m.ViewMode == ViewLogs {
				m.SearchMode = true
				m.SearchQuery = ""
				m.StatusMessage = ""
			}
			return m, nil
		}

		if config.Contains(kb.Logs.NextResult, key) {
			if m.ViewMode == ViewLogs && len(m.SearchResults) > 0 {
				m.SearchResultIdx++
				if m.SearchResultIdx >= len(m.SearchResults) {
					m.SearchResultIdx = 0
				}
				m.LogScroll = m.SearchResults[m.SearchResultIdx]
				m.StatusMessage = formatSearchStatus(&m)
			}
			return m, nil
		}

		if config.Contains(kb.Logs.PrevResult, key) {
			if m.ViewMode == ViewLogs && len(m.SearchResults) > 0 {
				m.SearchResultIdx--
				if m.SearchResultIdx < 0 {
					m.SearchResultIdx = len(m.SearchResults) - 1
				}
				m.LogScroll = m.SearchResults[m.SearchResultIdx]
				m.StatusMessage = formatSearchStatus(&m)
			}
			return m, nil
		}

		// Handle view switching
		if config.Contains(kb.Navigation.SwitchContainer, key) {
			if m.NavMode != NavContainers {
				m.NavMode = NavContainers
				m.ViewMode = ViewDetails
				m.Cursor = 0
				RebuildItemsFunc(&m)
			}
			return m, nil
		}

		if config.Contains(kb.Navigation.SwitchVolume, key) {
			if m.NavMode != NavVolumes {
				m.NavMode = NavVolumes
				m.ViewMode = ViewDetails
				m.Cursor = 0
				RebuildVolumeItemsFunc(&m)
			}
			return m, nil
		}

		if config.Contains(kb.Navigation.SwitchImage, key) {
			if m.NavMode != NavImages {
				m.NavMode = NavImages
				m.ViewMode = ViewDetails
				m.Cursor = 0
				RebuildImageItemsFunc(&m)
			}
			return m, nil
		}

		if config.Contains(kb.Navigation.SwitchNetwork, key) {
			if m.NavMode != NavNetworks {
				m.NavMode = NavNetworks
				m.ViewMode = ViewDetails
				m.Cursor = 0
				RebuildNetworkItemsFunc(&m)
			}
			return m, nil
		}
	}

	return m, nil
}

func (m Model) View() string {
	return RenderViewFunc(&m)
}

func performSearch(m *Model) {
	if m.SearchQuery == "" {
		m.SearchResults = nil
		m.SearchResultIdx = 0
		return
	}

	var results []int
	query := strings.ToLower(m.SearchQuery)

	for i, line := range m.Logs {
		if strings.Contains(strings.ToLower(line), query) {
			results = append(results, i)
		}
	}

	m.SearchResults = results
	m.SearchResultIdx = 0

	if len(results) > 0 {
		m.LogScroll = results[0]
		m.StatusMessage = formatSearchStatus(m)
	} else {
		m.StatusMessage = "No matches found"
	}
}

func formatSearchStatus(m *Model) string {
	if len(m.SearchResults) == 0 {
		return "No matches"
	}
	return fmt.Sprintf("Match %d/%d", m.SearchResultIdx+1, len(m.SearchResults))
}

func executeCommand(m *Model) tea.Cmd {
	cmd := strings.TrimSpace(m.CommandInput)
	m.CommandInput = ""

	switch cmd {
	case "q", "quit":
		// Quit application
		QuitFunc(m)
		return tea.Quit
	case "s", "start":
		// Start container
		m.StatusMessage = "Starting container..."
		return StartContainerFunc(m)
	case "S", "stop":
		// Stop container
		m.StatusMessage = "Stopping container..."
		return StopContainerFunc(m)
	case "noh":
		// Clear search highlighting
		m.SearchQuery = ""
		m.SearchResults = nil
		m.SearchResultIdx = 0
		m.StatusMessage = "Search cleared"
	case "help", "h":
		// Show help window
		m.HelpMode = true
		m.StatusMessage = ""
	default:
		m.StatusMessage = fmt.Sprintf("Unknown command: %s", cmd)
	}
	return nil
}
