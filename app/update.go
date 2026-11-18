package app

import (
	"fmt"
	"gdocker/config"
	"gdocker/ui/logviewer"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"gdocker/docker"
	"gdocker/models"
	tk "github.com/Gandalfalex/tuikit"
	"github.com/Gandalfalex/vim-kit/keys"
	"github.com/Gandalfalex/vim-kit/motion"
)

const (
	statusTimeout     = 5 * time.Second
	statsRefreshEvery = 2 * time.Second
)

// startupMsg shows a status message once the program runs, so that it
// expires like any other.
type startupMsg struct {
	text  string
	isErr bool
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{autoRefreshTickCmd(m.AutoRefreshSecs)}
	if m.startup != "" {
		cmds = append(cmds, func() tea.Msg { return startupMsg{m.startup, true} })
	}
	return tea.Batch(cmds...)
}

// Update wraps update so every new status message expires after a while
// instead of hiding the key hints until the next action.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.ensure()
	if cmd, ok := m.Shell.Update(msg); ok {
		return m, cmd
	}
	prev, prevErr := m.Status, m.StatusErr
	m.StatusErr = false // set again by update when it reports a failure
	next, cmd := m.update(msg)
	if next.Status == prev {
		next.StatusErr = prevErr
	}
	return next, tea.Batch(cmd, next.Expire(prev))
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case startupMsg:
		m.Status, m.StatusErr = msg.text, msg.isErr
		return m, nil

	case models.AutoRefreshTickMsg:
		return m, tea.Batch(autoRefreshTickCmd(m.AutoRefreshSecs), refreshCurrent(&m))

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.LogViewer.SetSize(m.DetailWidth(), m.Height-3) // as passed to LogViewer.View
		return m, nil

	case models.ContainersRefreshedMsg:
		expandedProjects := make(map[string]bool)
		for _, p := range m.Projects {
			if p.Expanded {
				expandedProjects[p.Name] = true
			}
		}

		key := selectionKey(&m)
		m.Containers = msg.Containers
		m.Standalone, m.Projects = groupByProject(msg.Containers)

		for i := range m.Projects {
			if expandedProjects[m.Projects[i].Name] {
				m.Projects[i].Expanded = true
			}
		}

		if m.NavMode == models.NavContainers {
			rebuildItems(&m, key)
		}
		if len(msg.Recreated) > 0 {
			if len(msg.Recreated) == 1 {
				r := msg.Recreated[0]
				m.Status = fmt.Sprintf("Container recreated: %s (%s -> %s)", r.Name, shortID(r.OldID), shortID(r.NewID))
			} else {
				m.Status = fmt.Sprintf("%d containers recreated", len(msg.Recreated))
			}
		}
		if c := m.SelectedContainer(); m.ViewMode == models.ViewEnv && c != nil && !c.EnvLoaded {
			m.Status = "Loading environment variables..."
			return m, m.docker().LoadEnv(c)
		}
		return m, nil

	case models.LogsLoadedMsg:
		if m.ViewMode != models.ViewLogs {
			return m, nil // left the log view before the logs arrived
		}
		containerName := "container"
		if c := m.SelectedContainer(); c != nil {
			containerName = c.Name
		}
		m.LogViewer.SetLines(msg.Lines, containerName)
		m.LogSince = msg.Since
		m.Status = ""
		m.LogViewer.FollowingLogs = msg.Follow
		if m.LogViewer.FollowingLogs {
			return m, m.followLogs()
		}
		return m, nil

	case models.LogsFailedMsg:
		if m.ViewMode == models.ViewLogs {
			m.ViewMode = models.ViewDetails
		}
		m.Status = fmt.Sprintf("Failed to load logs: %v", msg.Err)
		m.StatusErr = true
		return m, nil

	case models.LogsFollowedMsg:
		if len(msg.Lines) > 0 {
			m.LogViewer.AppendLines(msg.Lines)
		}
		if !msg.Since.IsZero() {
			m.LogSince = msg.Since
		}
		if m.ViewMode == models.ViewLogs && m.LogViewer.FollowingLogs {
			return m, m.followLogs()
		}
		return m, nil

	case models.LogFollowStoppedMsg:
		return m, nil

	case models.StatsLoadedMsg:
		if m.ViewMode != models.ViewStats || msg.Seq != m.StatsSeq {
			return m, nil // stale: view was left or restarted
		}
		m.Stats = msg.Stats
		return m, statsTickCmd(msg.Seq)

	case models.StatsTickMsg:
		if m.ViewMode == models.ViewStats && msg.Seq == m.StatsSeq {
			return m, m.docker().LoadStats(m.SelectedContainer(), m.StatsSeq)
		}
		return m, nil

	case models.EnvLoadedMsg:
		applyContainerEnv(&m, msg.ContainerID, msg.Env)
		if m.ViewMode == models.ViewEnv {
			m.Status = ""
		}
		return m, nil

	case docker.ConnectedMsg:
		m.docker().Close()
		m.Backend = nil
		if msg.Client != nil {
			m.Backend = msg.Client
		}
		m.DockerHost = msg.Host
		m.Containers = msg.Containers
		m.Volumes = msg.Volumes
		m.Images = msg.Images
		m.Networks = msg.Networks
		m.Standalone, m.Projects = groupByProject(m.Containers)
		m.Cursor, m.ListTop = 0, 0
		rebuildItems(&m, "")
		m.ViewMode = models.ViewDetails
		m.LogSince = time.Time{}
		if m.DockerHost == "" {
			m.Status = "Connected to local Docker"
		} else {
			m.Status = "Connected to " + m.DockerHost
		}
		return m, nil

	case models.ActionResultMsg:
		m.Status = msg.Message
		m.StatusErr = !msg.Success
		if msg.Success {
			return m, refreshCurrent(&m)
		}
		return m, nil

	case models.VolumesLoadedMsg:
		key := selectionKey(&m)
		m.Volumes = msg.Volumes
		if m.NavMode == models.NavVolumes {
			rebuildItems(&m, key)
		}
		if msg.Status != "" {
			m.Status = msg.Status
		}
		return m, nil

	case models.ImagesLoadedMsg:
		key := selectionKey(&m)
		m.Images = msg.Images
		if m.NavMode == models.NavImages {
			rebuildItems(&m, key)
		}
		if msg.Status != "" {
			m.Status = msg.Status
		}
		return m, nil

	case models.NetworksLoadedMsg:
		key := selectionKey(&m)
		m.Networks = msg.Networks
		if m.NavMode == models.NavNetworks {
			rebuildItems(&m, key)
		}
		return m, nil

	case models.InspectLoadedMsg:
		if m.ViewMode != models.ViewInspect {
			return m, nil
		}
		m.InspectData = msg.Data
		m.InspectScroll, m.InspectTop = 0, 0
		m.Status = ""
		return m, nil

	case tea.PasteMsg:
		text := singleLine(msg.Content)
		switch {
		case m.commandOpen():
			m.cmd.Line.Close()
			return m, m.prompt.Update(tea.PasteMsg{Content: text})
		case m.ViewMode == models.ViewLogs && m.LogViewer.IsSearchMode():
			m.LogViewer, _ = m.LogViewer.Update(tea.PasteMsg{Content: text})
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	key := msg.String()
	kb := m.bindings()

	// Quitting works everywhere, including prompts and the log viewer.
	if config.Contains(kb.General.ForceQuit, key) {
		return handleForceQuit(&m)
	}

	if m.Confirm != nil {
		confirm := m.Confirm
		m.Confirm = nil
		if key == "y" || key == "Y" {
			m.Status = "Working..."
			return m, confirm.Cmd
		}
		m.Status = "Cancelled"
		return m, nil
	}

	if m.commandOpen() {
		return m.handleCommandKey(msg)
	}

	if m.HelpMode {
		return m.handleHelpKey(key), nil
	}

	logSearch := m.ViewMode == models.ViewLogs && m.LogViewer.IsSearchMode()

	// Allow entering command mode from any view (including logs), but not
	// while typing a search query.
	if !logSearch && config.Contains(kb.Commands.Enter, key) {
		return handleCommandMode(&m)
	}

	// Delegate to log viewer when in log view
	if m.ViewMode == models.ViewLogs {
		wasFollowing := m.LogViewer.FollowingLogs
		updated, cmd := m.LogViewer.Update(msg)
		m.LogViewer = updated
		if wasFollowing && !m.LogViewer.FollowingLogs {
			m.docker().StopFollowLogs()
		}
		// Process output messages from the component
		if cmd != nil {
			outMsg := cmd()
			switch outMsg := outMsg.(type) {
			case logviewer.StatusMsg:
				m.Status = outMsg.Text
				return m, nil
			case logviewer.FollowRequestMsg:
				m.Status = "Log follow enabled"
				return m, m.followLogs()
			case logviewer.ExitMsg:
				m.docker().StopFollowLogs()
				m.ViewMode = models.ViewDetails
				m.LogSince = time.Time{}
				m.Status = ""
				return m, nil
			default:
				return m, cmd
			}
		}
		return m, nil
	}

	ev, ok := m.Keys.Feed(m.keymap(), key)
	if !ok {
		return m, nil // count or pending sequence
	}
	if applyMotion(&m, ev) {
		return m, nil
	}
	if handler, exists := keyHandlers[ev.Action]; exists {
		return handler(&m)
	}
	return m, nil
}

// handleCommandKey edits the ":" line. Candidates are looked up on tab
// only, since :ssh reads ~/.ssh/config for them.
func (m Model) handleCommandKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	esc := msg.String() == "esc"
	cmd, err := tk.HandleCommandKey(&m.cmd, &m, &m.prompt, msg)
	if err != nil {
		m.Status, m.StatusErr = err.Error(), true
	}
	if esc {
		m.Status = ""
	}
	return m, cmd
}

// handleHelpKey scrolls or closes the help view; no other key reaches the
// list while it is hidden behind the help.
func (m Model) handleHelpKey(key string) Model {
	kb := m.bindings()
	if key == "q" || config.Contains(kb.Views.Back, key) || config.Contains(kb.General.Help, key) {
		m.HelpMode = false
		m.Keys.Reset()
		return m
	}
	ev, ok := m.Keys.Feed(m.keymap(), key)
	if !ok {
		return m
	}
	n := max(helpScrollRange(&m), 1)
	w := motion.Window{Cursor: m.HelpScroll}
	if w.Apply(ev, n, max(m.Height/2, 1)) {
		m.HelpScroll = w.Cursor
	}
	return m
}

func (m *Model) bindings() *config.KeyBindings {
	if m.KeyBindings == nil {
		return config.Default()
	}
	return m.KeyBindings
}

func (m Model) View() tea.View {
	v := tea.NewView(RenderView(&m))
	v.AltScreen = true
	v.WindowTitle = "gdocker"
	return v
}

// refreshCurrent reloads the resource list that is currently shown.
func refreshCurrent(m *Model) tea.Cmd {
	switch m.NavMode {
	case models.NavContainers:
		return m.docker().RefreshContainers(m.Containers)
	case models.NavVolumes:
		return m.docker().RefreshVolumes()
	case models.NavImages:
		return m.docker().RefreshImages()
	case models.NavNetworks:
		return m.docker().RefreshNetworks()
	}
	return nil
}

// selectionKey identifies the item under the cursor across list rebuilds.
func selectionKey(m *Model) string {
	item, ok := m.SelectedItem()
	if !ok {
		return ""
	}
	return itemKey(item)
}

func itemKey(item models.ListItem) string {
	switch {
	case item.IsProject && item.Project != nil:
		return "project/" + item.Project.Name
	case item.IsContainer && item.Container != nil:
		return "container/" + item.Container.Name
	case item.IsVolume && item.Volume != nil:
		return "volume/" + item.Volume.Name
	case item.IsImage && item.Image != nil:
		return "image/" + item.Image.ID
	case item.IsNetwork && item.Network != nil:
		return "network/" + item.Network.ID
	}
	return ""
}

// rebuildItems rebuilds the list for the current nav mode and puts the
// cursor back on the item identified by key, if it still exists.
func rebuildItems(m *Model, key string) {
	switch m.NavMode {
	case models.NavVolumes:
		rebuildVolumeItems(m)
	case models.NavImages:
		rebuildImageItems(m)
	case models.NavNetworks:
		rebuildNetworkItems(m)
	default:
		rebuildContainerItems(m)
	}
	if key == "" {
		return
	}
	w := motion.Window{Cursor: m.Cursor}
	w.Select(len(m.Items), func(i int) bool { return itemKey(m.Items[i]) == key })
	m.Cursor = w.Cursor
}

// singleLine flattens pasted text for the one-line command and search inputs.
func singleLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func trimLastRune(s string) string {
	_, size := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-size]
}

func statsTickCmd(seq int) tea.Cmd {
	return tea.Tick(statsRefreshEvery, func(time.Time) tea.Msg { return models.StatsTickMsg{Seq: seq} })
}

// executeCommand runs a ":" line.
func executeCommand(m *Model, line string) tea.Cmd {
	m.ensure()
	cmd, err := m.cmd.Run(m, line)
	if err != nil {
		m.Status, m.StatusErr = err.Error(), true
	}
	return cmd
}

// gotoLine moves the cursor of the current view to line n (1-based).
func gotoLine(m *Model, n int) {
	ev := keys.Event{Action: motion.Top, Count: max(n, 1), HasCount: true}
	if m.ViewMode == models.ViewLogs {
		m.LogViewer.Goto(ev)
		return
	}
	applyMotion(m, ev)
}

func autoRefreshTickCmd(intervalSec int) tea.Cmd {
	if intervalSec < 1 {
		intervalSec = 10
	}
	return tea.Tick(time.Duration(intervalSec)*time.Second, func(time.Time) tea.Msg {
		return models.AutoRefreshTickMsg{}
	})
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func runtimeID(c *models.Container) string {
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

// followLogs waits for more log lines while the log view follows them, and
// stops the follower otherwise.
func (m *Model) followLogs() tea.Cmd {
	c := m.SelectedContainer()
	if m.ViewMode != models.ViewLogs || !m.LogViewer.FollowingLogs || c == nil {
		m.docker().StopFollowLogs()
		return nil
	}
	return m.docker().FollowLogs(c, m.LogSince)
}
