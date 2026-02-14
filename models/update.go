package models

import (
	"fmt"
	"gdocker/config"
	"gdocker/ui/logviewer"
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
		if m.ViewMode == ViewLogs && m.LogViewer.FollowingLogs {
			return m, tea.Batch(logFollowTickCmd(), FollowLogsFunc(&m))
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case ContainersRefreshedMsg:
		expandedProjects := make(map[string]bool)
		for _, p := range m.Projects {
			if p.Expanded {
				expandedProjects[p.Name] = true
			}
		}

		m.Containers = msg.Containers
		m.Standalone, m.Projects = GroupByProjectFunc(msg.Containers)

		for i := range m.Projects {
			if expandedProjects[m.Projects[i].Name] {
				m.Projects[i].Expanded = true
			}
		}

		RebuildItemsFunc(&m)
		return m, nil

	case LogsLoadedMsg:
		containerName := "container"
		if m.Cursor >= 0 && m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
			containerName = m.Items[m.Cursor].Container.Name
		}
		m.LogViewer.SetLines(msg.Lines, containerName)
		m.LogSince = msg.Since
		m.ViewMode = ViewLogs
		m.LogViewer.FollowingLogs = msg.Follow
		if m.LogViewer.FollowingLogs {
			return m, logFollowTickCmd()
		}
		return m, nil

	case LogsFollowedMsg:
		if len(msg.Lines) > 0 {
			m.LogViewer.AppendLines(msg.Lines)
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
		m.InspectScroll = 0
		return m, nil

	case tea.KeyMsg:
		// Command mode input
		if m.CommandMode {
			switch msg.String() {
			case "enter":
				m.CommandMode = false
				return m, executeCommand(&m)
			case "esc":
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
				if len(msg.String()) == 1 {
					m.CommandInput += msg.String()
				}
				return m, nil
			}
		}

		// Allow entering command mode from any view (including logs).
		if config.Contains(m.KeyBindings.Commands.Enter, msg.String()) {
			m.CommandMode = true
			m.CommandInput = ""
			m.StatusMessage = ""
			return m, nil
		}

		// Delegate to log viewer when in log view
		if m.ViewMode == ViewLogs {
			updated, cmd := m.LogViewer.Update(msg)
			m.LogViewer = updated
			// Process output messages from the component
			if cmd != nil {
				outMsg := cmd()
				switch outMsg := outMsg.(type) {
				case logviewer.StatusMsg:
					m.StatusMessage = outMsg.Text
					return m, nil
				case logviewer.FollowRequestMsg:
					m.StatusMessage = "Log follow enabled"
					return m, FollowLogsFunc(&m)
				case logviewer.ExitMsg:
					m.ViewMode = ViewDetails
					m.LogSince = time.Time{}
					m.StatusMessage = ""
					return m, nil
				default:
					return m, cmd
				}
			}
			return m, nil
		}

		// Build key handler map for non-log views
		keyHandlers := buildKeyHandlerMap(&m)
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

func executeCommand(m *Model) tea.Cmd {
	cmd := strings.TrimSpace(m.CommandInput)
	m.CommandInput = ""

	commandHandlers := buildCommandHandlerMap()
	if handler, exists := commandHandlers[cmd]; exists {
		return handler(m)
	}

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
