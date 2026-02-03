package models

import (
	"fmt"
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

		switch msg.String() {
		case "ctrl+c":
			QuitFunc(&m)
			return m, tea.Quit

		case "esc":
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

		case "j", "down":
			if m.ViewMode == ViewLogs || m.ViewMode == ViewInspect {
				maxLines := len(m.Logs)
				if m.ViewMode == ViewInspect {
					maxLines = len(strings.Split(m.InspectData, "\n"))
				}
				if m.LogScroll < maxLines-1 {
					m.LogScroll++
				}
			} else if m.ViewMode == ViewPorts {
				if m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
					if m.SelectedPort < len(m.Items[m.Cursor].Container.Ports)-1 {
						m.SelectedPort++
					}
				}
			} else {
				if m.Cursor < len(m.Items)-1 {
					m.Cursor++
				}
			}

		case "k", "up":
			if m.ViewMode == ViewLogs || m.ViewMode == ViewInspect {
				if m.LogScroll > 0 {
					m.LogScroll--
				}
			} else if m.ViewMode == ViewPorts {
				if m.SelectedPort > 0 {
					m.SelectedPort--
				}
			} else {
				if m.Cursor > 0 {
					m.Cursor--
				}
			}

		case "g":
			if m.ViewMode == ViewLogs || m.ViewMode == ViewInspect {
				m.LogScroll = 0
			} else {
				m.Cursor = 0
			}

		case "G":
			if m.ViewMode == ViewLogs || m.ViewMode == ViewInspect {
				maxLines := len(m.Logs) - 1
				if m.ViewMode == ViewInspect {
					maxLines = len(strings.Split(m.InspectData, "\n")) - 1
				}
				m.LogScroll = maxLines
			} else {
				m.Cursor = len(m.Items) - 1
			}

		case " ":
			// Toggle project expansion
			if m.ViewMode == ViewDetails && m.Cursor < len(m.Items) && m.Items[m.Cursor].IsProject {
				idx := m.Items[m.Cursor].Index
				m.Projects[idx].Expanded = !m.Projects[idx].Expanded
				RebuildItemsFunc(&m)
			}

		case "r":
			// Restart container
			return m, RestartContainerFunc(&m)

		case "d":
			// Delete based on navigation mode
			switch m.NavMode {
			case NavContainers:
				return m, DeleteContainerFunc(&m)
			case NavVolumes:
				return m, DeleteVolumeFunc(&m)
			case NavImages:
				return m, DeleteImageFunc(&m)
			}

		case "l":
			// View logs
			return m, LoadLogsFunc(&m)

		case "e":
			// Execute shell
			if m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
				return m, ExecShellFunc(&m)
			}

		case "p":
			// View ports
			if m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
				m.ViewMode = ViewPorts
				m.SelectedPort = 0
			}

		case "v":
			// View environment variables
			if m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
				m.ViewMode = ViewEnv
			}

		case "t":
			// View/refresh stats
			if m.ViewMode == ViewStats {
				// Refresh stats if already in stats view
				return m, LoadStatsFunc(&m)
			} else if m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
				// Load stats for the first time
				return m, LoadStatsFunc(&m)
			}

		case "i":
			// View inspect
			if m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
				return m, LoadInspectFunc(&m)
			}

		case "o", "enter":
			// Open port in browser
			if m.ViewMode == ViewPorts && m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
				return m, OpenPortInBrowserFunc(&m)
			}
			// Toggle project expansion in details view
			if m.ViewMode == ViewDetails && m.Cursor < len(m.Items) && m.Items[m.Cursor].IsProject {
				idx := m.Items[m.Cursor].Index
				m.Projects[idx].Expanded = !m.Projects[idx].Expanded
				RebuildItemsFunc(&m)
			}

		case ":":
			// Start command mode
			m.CommandMode = true
			m.CommandInput = ""
			m.StatusMessage = ""

		case "?":
			// Start search in logs view
			if m.ViewMode == ViewLogs {
				m.SearchMode = true
				m.SearchQuery = ""
				m.StatusMessage = ""
			}

		case "n":
			// Next search result
			if m.ViewMode == ViewLogs && len(m.SearchResults) > 0 {
				m.SearchResultIdx++
				if m.SearchResultIdx >= len(m.SearchResults) {
					m.SearchResultIdx = 0
				}
				m.LogScroll = m.SearchResults[m.SearchResultIdx]
				m.StatusMessage = formatSearchStatus(&m)
			}

		case "N":
			// Previous search result
			if m.ViewMode == ViewLogs && len(m.SearchResults) > 0 {
				m.SearchResultIdx--
				if m.SearchResultIdx < 0 {
					m.SearchResultIdx = len(m.SearchResults) - 1
				}
				m.LogScroll = m.SearchResults[m.SearchResultIdx]
				m.StatusMessage = formatSearchStatus(&m)
			}

		case "1":
			// Switch to containers view
			if m.NavMode != NavContainers {
				m.NavMode = NavContainers
				m.ViewMode = ViewDetails
				m.Cursor = 0
				RebuildItemsFunc(&m)
			}

		case "2":
			// Switch to volumes view
			if m.NavMode != NavVolumes {
				m.NavMode = NavVolumes
				m.ViewMode = ViewDetails
				m.Cursor = 0
				RebuildVolumeItemsFunc(&m)
			}

		case "3":
			// Switch to images view
			if m.NavMode != NavImages {
				m.NavMode = NavImages
				m.ViewMode = ViewDetails
				m.Cursor = 0
				RebuildImageItemsFunc(&m)
			}

		case "4":
			// Switch to networks view
			if m.NavMode != NavNetworks {
				m.NavMode = NavNetworks
				m.ViewMode = ViewDetails
				m.Cursor = 0
				RebuildNetworkItemsFunc(&m)
			}
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
