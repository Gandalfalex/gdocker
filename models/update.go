package models

import (
	"fmt"
	"gdocker/config"
	"gdocker/ui/logviewer"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/Gandalfalex/vim-kit/cmdline"
	"github.com/Gandalfalex/vim-kit/keys"
	"github.com/Gandalfalex/vim-kit/motion"
)

// These will be set by the docker and ui packages to avoid circular imports
var (
	GroupByProjectFunc      func([]Container) ([]Container, []ComposeGroup)
	RebuildItemsFunc        func(*Model)
	RebuildVolumeItemsFunc  func(*Model)
	RebuildImageItemsFunc   func(*Model)
	RebuildNetworkItemsFunc func(*Model)
	RefreshContainersFunc   func(*Model) tea.Cmd
	RefreshVolumesFunc      func(*Model) tea.Cmd
	RefreshImagesFunc       func(*Model) tea.Cmd
	RefreshNetworksFunc     func(*Model) tea.Cmd
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
	// HelpScrollRangeFunc returns how many scroll positions the help view has.
	HelpScrollRangeFunc func(*Model) int
)

const (
	statusTimeout     = 5 * time.Second
	statsRefreshEvery = 2 * time.Second
)

type statusClearMsg struct{ seq int }

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{autoRefreshTickCmd(m.AutoRefreshSecs)}
	if m.StatusMessage != "" {
		cmds = append(cmds, statusClearCmd(m.statusSeq, 2*statusTimeout))
	}
	return tea.Batch(cmds...)
}

// Update wraps update so every new status message expires after a while
// instead of hiding the key hints until the next action.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	prev, prevErr := m.StatusMessage, m.StatusError
	m.StatusError = false // set again by update when it reports a failure
	next, cmd := m.update(msg)
	if next.StatusMessage == prev {
		next.StatusError = prevErr
	} else if next.StatusMessage != "" {
		next.statusSeq++
		timeout := statusTimeout
		if next.StatusError {
			timeout *= 2
		}
		cmd = tea.Batch(cmd, statusClearCmd(next.statusSeq, timeout))
	}
	return next, cmd
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusClearMsg:
		if msg.seq == m.statusSeq {
			m.StatusMessage = ""
		}
		return m, nil

	case AutoRefreshTickMsg:
		return m, tea.Batch(autoRefreshTickCmd(m.AutoRefreshSecs), refreshCurrent(&m))

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.LogViewer.SetSize(m.DetailWidth(), m.Height-3) // as passed to LogViewer.View
		return m, nil

	case ContainersRefreshedMsg:
		expandedProjects := make(map[string]bool)
		for _, p := range m.Projects {
			if p.Expanded {
				expandedProjects[p.Name] = true
			}
		}

		key := selectionKey(&m)
		m.Containers = msg.Containers
		m.Standalone, m.Projects = GroupByProjectFunc(msg.Containers)

		for i := range m.Projects {
			if expandedProjects[m.Projects[i].Name] {
				m.Projects[i].Expanded = true
			}
		}

		if m.NavMode == NavContainers {
			rebuildItems(&m, key)
		}
		if len(msg.Recreated) > 0 {
			if len(msg.Recreated) == 1 {
				r := msg.Recreated[0]
				m.StatusMessage = fmt.Sprintf("Container recreated: %s (%s -> %s)", r.Name, shortID(r.OldID), shortID(r.NewID))
			} else {
				m.StatusMessage = fmt.Sprintf("%d containers recreated", len(msg.Recreated))
			}
		}
		if c := m.SelectedContainer(); LoadEnvFunc != nil && m.ViewMode == ViewEnv && c != nil && !c.EnvLoaded {
			m.StatusMessage = "Loading environment variables..."
			return m, LoadEnvFunc(&m)
		}
		return m, nil

	case LogsLoadedMsg:
		if m.ViewMode != ViewLogs {
			return m, nil // left the log view before the logs arrived
		}
		containerName := "container"
		if c := m.SelectedContainer(); c != nil {
			containerName = c.Name
		}
		m.LogViewer.SetLines(msg.Lines, containerName)
		m.LogSince = msg.Since
		m.StatusMessage = ""
		m.LogViewer.FollowingLogs = msg.Follow
		if m.LogViewer.FollowingLogs {
			return m, FollowLogsFunc(&m)
		}
		return m, nil

	case LogsFailedMsg:
		if m.ViewMode == ViewLogs {
			m.ViewMode = ViewDetails
		}
		m.StatusMessage = fmt.Sprintf("Failed to load logs: %v", msg.Err)
		m.StatusError = true
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
		if m.ViewMode != ViewStats || msg.Seq != m.StatsSeq {
			return m, nil // stale: view was left or restarted
		}
		m.Stats = msg.Stats
		return m, statsTickCmd(msg.Seq)

	case StatsTickMsg:
		if m.ViewMode == ViewStats && msg.Seq == m.StatsSeq && LoadStatsFunc != nil {
			return m, LoadStatsFunc(&m)
		}
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
		m.Cursor, m.ListTop = 0, 0
		rebuildItems(&m, "")
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
		m.StatusError = !msg.Success
		if msg.Success {
			return m, refreshCurrent(&m)
		}
		return m, nil

	case VolumesLoadedMsg:
		key := selectionKey(&m)
		m.Volumes = msg.Volumes
		if m.NavMode == NavVolumes {
			rebuildItems(&m, key)
		}
		if msg.Status != "" {
			m.StatusMessage = msg.Status
		}
		return m, nil

	case ImagesLoadedMsg:
		key := selectionKey(&m)
		m.Images = msg.Images
		if m.NavMode == NavImages {
			rebuildItems(&m, key)
		}
		if msg.Status != "" {
			m.StatusMessage = msg.Status
		}
		return m, nil

	case NetworksLoadedMsg:
		key := selectionKey(&m)
		m.Networks = msg.Networks
		if m.NavMode == NavNetworks {
			rebuildItems(&m, key)
		}
		return m, nil

	case InspectLoadedMsg:
		if m.ViewMode != ViewInspect {
			return m, nil
		}
		m.InspectData = msg.Data
		m.InspectScroll, m.InspectTop = 0, 0
		m.StatusMessage = ""
		return m, nil

	case tea.PasteMsg:
		text := singleLine(msg.Content)
		switch {
		case m.CommandMode:
			m.CommandInput += text
			m.CmdLine.Close()
		case m.ViewMode == ViewLogs && m.LogViewer.IsSearchMode():
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
			m.StatusMessage = "Working..."
			return m, confirm.Cmd
		}
		m.StatusMessage = "Cancelled"
		return m, nil
	}

	if m.CommandMode {
		return m.handleCommandKey(msg)
	}

	if m.HelpMode {
		return m.handleHelpKey(key), nil
	}

	logSearch := m.ViewMode == ViewLogs && m.LogViewer.IsSearchMode()

	// Allow entering command mode from any view (including logs), but not
	// while typing a search query.
	if !logSearch && config.Contains(kb.Commands.Enter, key) {
		return handleCommandMode(&m)
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

// handleCommandKey edits the ":" line. Completion candidates are looked up
// on tab rather than per key press, since :ssh reads ~/.ssh/config for them.
func (m Model) handleCommandKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	setInput := func(text string, ok bool) {
		if ok {
			m.CommandInput = text
		}
	}
	switch msg.String() {
	case "enter":
		m.CommandMode = false
		m.CmdLine.Commit(strings.TrimSpace(m.CommandInput))
		return m, executeCommand(&m)
	case "esc":
		m.CommandMode = false
		m.CommandInput = ""
		m.CmdLine.Close()
		m.StatusMessage = ""
	case "backspace":
		if m.CommandInput == "" {
			m.CommandMode = false // like vim: backspace on empty ":" leaves
		}
		m.CommandInput = trimLastRune(m.CommandInput)
		m.CmdLine.Close()
	case "tab", "ctrl+n":
		completeCommandInput(&m, 1)
	case "shift+tab", "ctrl+p":
		completeCommandInput(&m, -1)
	case "up":
		setInput(m.CmdLine.Recall(-1))
		m.CmdLine.Close()
	case "down":
		setInput(m.CmdLine.Recall(1))
		m.CmdLine.Close()
	default:
		if msg.Text != "" {
			m.CommandInput += msg.Text
			m.CmdLine.Close()
		}
	}
	return m, nil
}

// completeCommandInput cycles through the candidates for the typed text.
func completeCommandInput(m *Model, dir int) {
	if !m.CmdLine.Cycling() {
		m.CmdLine.SetItems(Commands.Complete(m, m.CommandInput))
	}
	if items := m.CmdLine.Items; len(items) == 1 && items[0].Value == strings.TrimSpace(m.CommandInput) {
		// Nothing left to complete in the name: move on to the argument.
		if c, err := Commands.Resolve(items[0].Value); err == nil && c.Args != "" && !strings.HasSuffix(m.CommandInput, " ") {
			m.CommandInput = items[0].Value + " "
		}
		m.CmdLine.Close()
		return
	}
	if text, ok := m.CmdLine.Cycle(dir); ok {
		m.CommandInput = text
	}
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
	n := 1
	if HelpScrollRangeFunc != nil {
		n = max(HelpScrollRangeFunc(&m), 1)
	}
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
	v := tea.NewView(RenderViewFunc(&m))
	v.AltScreen = true
	v.WindowTitle = "gdocker"
	return v
}

// refreshCurrent reloads the resource list that is currently shown.
func refreshCurrent(m *Model) tea.Cmd {
	var refresh func(*Model) tea.Cmd
	switch m.NavMode {
	case NavContainers:
		refresh = RefreshContainersFunc
	case NavVolumes:
		refresh = RefreshVolumesFunc
	case NavImages:
		refresh = RefreshImagesFunc
	case NavNetworks:
		refresh = RefreshNetworksFunc
	}
	if refresh == nil {
		return nil
	}
	return refresh(m)
}

// selectionKey identifies the item under the cursor across list rebuilds.
func selectionKey(m *Model) string {
	item, ok := m.SelectedItem()
	if !ok {
		return ""
	}
	return itemKey(item)
}

func itemKey(item ListItem) string {
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
	rebuild := RebuildItemsFunc
	switch m.NavMode {
	case NavVolumes:
		rebuild = RebuildVolumeItemsFunc
	case NavImages:
		rebuild = RebuildImageItemsFunc
	case NavNetworks:
		rebuild = RebuildNetworkItemsFunc
	}
	if rebuild != nil {
		rebuild(m)
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

func statusClearCmd(seq int, after time.Duration) tea.Cmd {
	return tea.Tick(after, func(time.Time) tea.Msg { return statusClearMsg{seq: seq} })
}

func statsTickCmd(seq int) tea.Cmd {
	return tea.Tick(statsRefreshEvery, func(time.Time) tea.Msg { return StatsTickMsg{Seq: seq} })
}

func executeCommand(m *Model) tea.Cmd {
	name, arg := cmdline.Split(m.CommandInput)
	m.CommandInput = ""
	m.CmdLine.Close()
	if name == "" {
		return nil
	}
	if n, ok := cmdline.LineNumber(name); ok { // :42 jumps to line 42
		gotoLine(m, n)
		return nil
	}
	c, err := Commands.Resolve(name)
	if err != nil {
		m.StatusMessage = err.Error()
		m.StatusError = true
		return nil
	}
	return c.Run(m, arg)
}

// gotoLine moves the cursor of the current view to line n (1-based).
func gotoLine(m *Model, n int) {
	ev := keys.Event{Action: motion.Top, Count: max(n, 1), HasCount: true}
	if m.ViewMode == ViewLogs {
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
		return AutoRefreshTickMsg{}
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
