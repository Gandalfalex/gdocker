package models

import (
	"fmt"
	"gdocker/config"
	"gdocker/ui/logviewer"
	"sort"
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
	LoadEnvFunc             func(*Model) tea.Cmd
	ConnectDockerHostFunc   func(*Model, string) tea.Cmd
	StartContainerFunc      func(*Model) tea.Cmd
	StopContainerFunc       func(*Model) tea.Cmd
	RestartContainerFunc    func(*Model) tea.Cmd
	DeleteContainerFunc     func(*Model) tea.Cmd
	DeleteVolumeFunc        func(*Model) tea.Cmd
	DeleteImageFunc         func(*Model) tea.Cmd
	LoadLogsFunc            func(*Model) tea.Cmd
	FollowLogsFunc          func(*Model) tea.Cmd
	StopFollowLogsFunc      func(*Model)
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
		if len(msg.Recreated) > 0 {
			if len(msg.Recreated) == 1 {
				r := msg.Recreated[0]
				m.StatusMessage = fmt.Sprintf("Container recreated: %s (%s -> %s)", r.Name, shortID(r.OldID), shortID(r.NewID))
			} else {
				m.StatusMessage = fmt.Sprintf("%d containers recreated", len(msg.Recreated))
			}
		}
		if LoadEnvFunc != nil && m.ViewMode == ViewEnv && m.Cursor >= 0 && m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer && !m.Items[m.Cursor].Container.EnvLoaded {
			m.StatusMessage = "Loading environment variables..."
			return m, LoadEnvFunc(&m)
		}
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
			return m, FollowLogsFunc(&m)
		}
		return m, nil

	case LogsFollowedMsg:
		if len(msg.Lines) > 0 {
			m.LogViewer.AppendLines(msg.Lines)
		}
		if !msg.Since.IsZero() {
			m.LogSince = msg.Since
		}
		if m.ViewMode == ViewLogs && m.LogViewer.FollowingLogs {
			return m, FollowLogsFunc(&m)
		}
		return m, nil

	case LogFollowStoppedMsg:
		return m, nil

	case StatsLoadedMsg:
		m.Stats = msg.Stats
		m.ViewMode = ViewStats
		return m, nil

	case EnvLoadedMsg:
		applyContainerEnv(&m, msg.ContainerID, msg.Env)
		if m.ViewMode == ViewEnv {
			m.StatusMessage = ""
		}
		return m, nil

	case DockerConnectedMsg:
		if StopFollowLogsFunc != nil {
			StopFollowLogsFunc(&m)
		}
		if m.DockerClient != nil {
			m.DockerClient.Close()
		}
		m.DockerClient = msg.Client
		m.DockerHost = msg.Host
		m.Containers = msg.Containers
		m.Volumes = msg.Volumes
		m.Images = msg.Images
		m.Networks = msg.Networks
		m.Standalone, m.Projects = GroupByProjectFunc(m.Containers)
		switch m.NavMode {
		case NavContainers:
			RebuildItemsFunc(&m)
		case NavVolumes:
			RebuildVolumeItemsFunc(&m)
		case NavImages:
			RebuildImageItemsFunc(&m)
		case NavNetworks:
			RebuildNetworkItemsFunc(&m)
		}
		m.ViewMode = ViewDetails
		m.LogSince = time.Time{}
		if msg.Host == "" {
			m.StatusMessage = "Connected to local Docker"
		} else {
			m.StatusMessage = "Connected to " + msg.Host
		}
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
				resetCommandCompletions(&m)
				return m, executeCommand(&m)
			case "esc":
				m.CommandMode = false
				m.CommandInput = ""
				resetCommandCompletions(&m)
				m.StatusMessage = ""
				return m, nil
			case "backspace":
				if len(m.CommandInput) > 0 {
					m.CommandInput = m.CommandInput[:len(m.CommandInput)-1]
				}
				resetCommandCompletions(&m)
				return m, nil
			case "tab":
				completeSSHCommandInput(&m)
				return m, nil
			default:
				if len(msg.String()) == 1 {
					m.CommandInput += msg.String()
				}
				resetCommandCompletions(&m)
				return m, nil
			}
		}

		// Allow entering command mode from any view (including logs).
		if config.Contains(m.KeyBindings.Commands.Enter, msg.String()) {
			m.CommandMode = true
			m.CommandInput = ""
			resetCommandCompletions(&m)
			m.StatusMessage = ""
			return m, nil
		}

		// Delegate to log viewer when in log view
		if m.ViewMode == ViewLogs {
			wasFollowing := m.LogViewer.FollowingLogs
			updated, cmd := m.LogViewer.Update(msg)
			m.LogViewer = updated
			if wasFollowing && !m.LogViewer.FollowingLogs && StopFollowLogsFunc != nil {
				StopFollowLogsFunc(&m)
			}
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
					if StopFollowLogsFunc != nil {
						StopFollowLogsFunc(&m)
					}
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
	resetCommandCompletions(m)
	if cmd == "" {
		return nil
	}

	parts := strings.Fields(cmd)
	base := parts[0]
	args := parts[1:]

	if base == "ssh" {
		return cmdSSH(m, args)
	}

	commandHandlers := buildCommandHandlerMap()
	if handler, exists := commandHandlers[base]; exists {
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

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func runtimeID(c *Container) string {
	if c == nil {
		return ""
	}
	if c.FullID != "" {
		return c.FullID
	}
	return c.ID
}

func applyContainerEnv(m *Model, containerID string, env []string) {
	for i := range m.Containers {
		if runtimeID(&m.Containers[i]) == containerID {
			m.Containers[i].Env = env
			m.Containers[i].EnvLoaded = true
		}
	}
	for i := range m.Standalone {
		if runtimeID(&m.Standalone[i]) == containerID {
			m.Standalone[i].Env = env
			m.Standalone[i].EnvLoaded = true
		}
	}
	for i := range m.Projects {
		for j := range m.Projects[i].Containers {
			if runtimeID(&m.Projects[i].Containers[j]) == containerID {
				m.Projects[i].Containers[j].Env = env
				m.Projects[i].Containers[j].EnvLoaded = true
			}
		}
	}
}

func resetCommandCompletions(m *Model) {
	m.CommandCompletions = nil
	m.CommandCompletionIdx = 0
	m.CommandCompletionBase = ""
}

func completeSSHCommandInput(m *Model) {
	raw := strings.TrimSpace(m.CommandInput)
	if raw == "" {
		return
	}

	if m.CommandCompletionBase != "" &&
		len(m.CommandCompletions) > 0 &&
		m.CommandCompletionIdx >= 0 &&
		m.CommandCompletionIdx < len(m.CommandCompletions) &&
		m.CommandInput == "ssh "+m.CommandCompletions[m.CommandCompletionIdx] {
		m.CommandCompletionIdx = (m.CommandCompletionIdx + 1) % len(m.CommandCompletions)
		m.CommandInput = "ssh " + m.CommandCompletions[m.CommandCompletionIdx]
		return
	}

	parts := strings.Fields(raw)
	if len(parts) == 0 || parts[0] != "ssh" || len(parts) > 2 {
		return
	}

	if len(parts) == 1 {
		if !strings.HasSuffix(m.CommandInput, " ") {
			m.CommandInput = "ssh "
			return
		}
	}

	prefix := ""
	if len(parts) == 2 {
		prefix = parts[1]
	}

	candidates := getSSHCompletionCandidates(m)
	if len(candidates) == 0 {
		return
	}

	matches := make([]string, 0, len(candidates))
	lowerPrefix := strings.ToLower(prefix)
	for _, c := range candidates {
		if strings.HasPrefix(strings.ToLower(c), lowerPrefix) {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		return
	}
	sort.Strings(matches)

	base := "ssh " + prefix
	if m.CommandCompletionBase == base && len(m.CommandCompletions) > 0 {
		m.CommandCompletionIdx = (m.CommandCompletionIdx + 1) % len(m.CommandCompletions)
	} else {
		m.CommandCompletions = matches
		m.CommandCompletionIdx = 0
		m.CommandCompletionBase = base
	}

	m.CommandInput = "ssh " + m.CommandCompletions[m.CommandCompletionIdx]
}
