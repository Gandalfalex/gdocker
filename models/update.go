package models

import (
	"fmt"
	"strings"
	"time"

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
	FollowLogsFunc          func(*Model) tea.Cmd
	LoadInspectFunc         func(*Model) tea.Cmd
	LoadStatsFunc           func(*Model) tea.Cmd
	ExecShellFunc           func(*Model) tea.Cmd
	OpenPortInBrowserFunc   func(*Model) tea.Cmd
	QuitFunc                func(*Model)
	RenderViewFunc          func(*Model) string
)

func (m Model) Init() tea.Cmd {
	return autoRefreshTickCmd(m.AutoRefreshSecs)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case AutoRefreshTickMsg:
		next := autoRefreshTickCmd(m.AutoRefreshSecs)
		if m.NavMode == NavContainers {
			return m, tea.Batch(next, RefreshContainersFunc(&m))
		}
		return m, next

	case LogFollowTickMsg:
		if m.ViewMode == ViewLogs && m.FollowingLogs {
			return m, tea.Batch(logFollowTickCmd(), FollowLogsFunc(&m))
		}
		return m, nil

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
		m.LogSince = msg.Since
		m.ViewMode = ViewLogs
		m.FollowingLogs = msg.Follow
		if m.FollowingLogs {
			return m, logFollowTickCmd()
		}
		return m, nil

	case LogsFollowedMsg:
		if len(msg.Lines) > 0 {
			for _, line := range msg.Lines {
				// Skip immediate duplicates caused by coarse timestamp "since" filters.
				if len(m.Logs) == 0 || m.Logs[len(m.Logs)-1] != line {
					m.Logs = append(m.Logs, line)
				}
			}
			m.LogScroll = len(m.Logs) - 1
			if m.SearchQuery != "" {
				performSearch(&m)
				if len(m.SearchResults) > 0 {
					m.LogScroll = len(m.Logs) - 1
				}
			}
		}
		if !msg.Since.IsZero() {
			m.LogSince = msg.Since
		}
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

		// Build key handler map
		keyHandlers := buildKeyHandlerMap(&m)

		// Look up and execute handler for this key
		key := msg.String()
		if handler, exists := keyHandlers[key]; exists {
			return handler(&m)
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

	// Build command handler map
	commandHandlers := buildCommandHandlerMap()

	// Look up and execute command handler
	if handler, exists := commandHandlers[cmd]; exists {
		return handler(m)
	}

	// Unknown command
	m.StatusMessage = fmt.Sprintf("Unknown command: %s", cmd)
	return nil
}

func autoRefreshTickCmd(intervalSec int) tea.Cmd {
	if intervalSec < 1 {
		intervalSec = 10
	}
	return tea.Tick(time.Duration(intervalSec)*time.Second, func(time.Time) tea.Msg {
		return AutoRefreshTickMsg{}
	})
}

func logFollowTickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(time.Time) tea.Msg {
		return LogFollowTickMsg{}
	})
}
