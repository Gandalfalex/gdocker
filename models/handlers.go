package models

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gdocker/config"

	tea "charm.land/bubbletea/v2"
	"github.com/Gandalfalex/vim-kit/cmdline"
	"github.com/Gandalfalex/vim-kit/keys"
	"github.com/Gandalfalex/vim-kit/motion"
)

// KeyHandler is a function that handles a key press
type KeyHandler func(*Model) (Model, tea.Cmd)

// keyHandlers maps actions (the yaml names in config.KeyBindings) to
// handlers. Motions (up, down, top, ...) are handled by applyMotion.
var keyHandlers = map[keys.Action]KeyHandler{
	"toggle_expand":    handleToggleExpand,
	"switch_container": handleSwitchContainer,
	"switch_volume":    handleSwitchVolume,
	"switch_image":     handleSwitchImage,
	"switch_network":   handleSwitchNetwork,
	"restart":          handleRestart,
	"delete":           handleDelete,
	"logs":             handleLogs,
	"exec":             handleExec,
	"ports":            handlePorts,
	"env":              handleEnv,
	"stats":            handleStats,
	"inspect":          handleInspect,
	"open_port":        handleOpenPort,
	"enter":            handleCommandMode,
	"help":             handleHelp,
	"back":             handleBack,
	"force_quit":       handleForceQuit,
}

// keymap returns the keymap for the current view, building it on first use.
// Config is validated on load; should it still be invalid, defaults are used.
func (m *Model) keymap() *keys.Keymap {
	if m.keymaps == nil {
		kb := m.KeyBindings
		if kb == nil {
			kb = config.Default()
		}
		m.keymaps = &keymaps{}
		var err error
		if m.keymaps.main, err = kb.MainKeymap(); err != nil {
			m.keymaps.main, _ = config.Default().MainKeymap()
		}
		if m.keymaps.ports, err = kb.PortsKeymap(); err != nil {
			m.keymaps.ports, _ = config.Default().PortsKeymap()
		}
	}
	if m.ViewMode == ViewPorts {
		return m.keymaps.ports
	}
	return m.keymaps.main
}

// applyMotion moves the cursor of the current view and reports whether ev
// was a motion. Detail views scroll their own content; the list cursor only
// moves in the details view, so a view never shows data of another item.
func applyMotion(m *Model, ev keys.Event) bool {
	var cursor, top *int
	var n, h int
	unused := 0
	switch m.ViewMode {
	case ViewInspect:
		cursor, top = &m.InspectScroll, &m.InspectTop
		n, h = strings.Count(m.InspectData, "\n")+1, m.PaneRows()
	case ViewEnv:
		cursor, top, h = &m.EnvScroll, &m.EnvTop, m.PaneRows()
		if c := m.SelectedContainer(); c != nil {
			n = len(c.Env)
		}
	case ViewPorts:
		cursor, top = &m.SelectedPort, &unused
		if c := m.SelectedContainer(); c != nil {
			n = len(c.Ports)
		}
		h = n
	case ViewStats:
		cursor, top, n, h = &unused, &unused, 1, 1
	default:
		cursor, top, n, h = &m.Cursor, &m.ListTop, len(m.Items), m.ListRows()
	}
	n, h = max(n, 1), max(h, 1)
	w := motion.Window{Cursor: *cursor, Top: *top}
	w.Scroll(n, h) // sync with what is on screen before screen-relative motions
	if !w.Apply(ev, n, h) {
		return false
	}
	w.Scroll(n, h)
	*cursor, *top = w.Cursor, w.Top
	return true
}

// Navigation handlers

func handleToggleExpand(m *Model) (Model, tea.Cmd) {
	if m.ViewMode == ViewDetails && m.Cursor < len(m.Items) && m.Items[m.Cursor].IsProject {
		idx := m.Items[m.Cursor].Index
		m.Projects[idx].Expanded = !m.Projects[idx].Expanded
		RebuildItemsFunc(m)
	}
	return *m, nil
}

// switchNav shows another resource list and reloads it in the background.
func switchNav(m *Model, mode NavigationMode) (Model, tea.Cmd) {
	if m.NavMode == mode && m.ViewMode == ViewDetails {
		return *m, nil
	}
	if m.NavMode != mode {
		m.Cursor, m.ListTop = 0, 0
	}
	m.NavMode = mode
	m.ViewMode = ViewDetails
	rebuildItems(m, "")
	return *m, refreshCurrent(m)
}

func handleSwitchContainer(m *Model) (Model, tea.Cmd) { return switchNav(m, NavContainers) }
func handleSwitchVolume(m *Model) (Model, tea.Cmd)    { return switchNav(m, NavVolumes) }
func handleSwitchImage(m *Model) (Model, tea.Cmd)     { return switchNav(m, NavImages) }
func handleSwitchNetwork(m *Model) (Model, tea.Cmd)   { return switchNav(m, NavNetworks) }

// Container action handlers

// requireContainer returns the selected container, or sets a hint when the
// cursor is on something else (and, with running set, when it is stopped).
func requireContainer(m *Model, running bool) *Container {
	c := m.SelectedContainer()
	switch {
	case c == nil:
		m.StatusMessage = "Select a container first"
		return nil
	case running && c.State != "running":
		m.StatusMessage = c.Name + " is not running (:s to start)"
		return nil
	}
	return c
}

func handleRestart(m *Model) (Model, tea.Cmd) {
	c := requireContainer(m, false)
	if c == nil {
		return *m, nil
	}
	m.StatusMessage = "Restarting " + c.Name + "..."
	return *m, RestartContainerFunc(m)
}

// handleDelete asks for confirmation; the command is built now so it keeps
// targeting this item even if the list changes while the prompt is open.
func handleDelete(m *Model) (Model, tea.Cmd) {
	item, ok := m.SelectedItem()
	if !ok {
		return *m, nil
	}
	var cmd tea.Cmd
	var what string
	switch {
	case item.IsContainer:
		what = "container " + item.Container.Name
		if item.Container.State == "running" {
			what = "RUNNING " + what
		}
		cmd = DeleteContainerFunc(m)
	case item.IsVolume:
		what = "volume " + item.Volume.Name
		cmd = DeleteVolumeFunc(m)
	case item.IsImage:
		what = "image " + item.Image.ID
		if len(item.Image.RepoTags) > 0 {
			what = "image " + item.Image.RepoTags[0]
		}
		cmd = DeleteImageFunc(m)
	default:
		m.StatusMessage = "Nothing to delete here"
		return *m, nil
	}
	if cmd != nil {
		m.Confirm = &Confirm{Prompt: "Delete " + what + "?", Cmd: cmd}
		m.StatusMessage = ""
	}
	return *m, nil
}

func handleLogs(m *Model) (Model, tea.Cmd) {
	c := requireContainer(m, false)
	if c == nil {
		return *m, nil
	}
	m.ViewMode = ViewLogs
	m.LogViewer.SetLoading(c.Name)
	return *m, LoadLogsFunc(m)
}

func handleExec(m *Model) (Model, tea.Cmd) {
	if requireContainer(m, true) == nil {
		return *m, nil
	}
	return *m, ExecShellFunc(m)
}

func handlePorts(m *Model) (Model, tea.Cmd) {
	if requireContainer(m, false) != nil {
		m.ViewMode = ViewPorts
		m.SelectedPort = 0
	}
	return *m, nil
}

func handleEnv(m *Model) (Model, tea.Cmd) {
	if c := requireContainer(m, false); c != nil {
		m.ViewMode = ViewEnv
		m.EnvScroll, m.EnvTop = 0, 0
		if !c.EnvLoaded && LoadEnvFunc != nil {
			m.StatusMessage = "Loading environment variables..."
			return *m, LoadEnvFunc(m)
		}
	}
	return *m, nil
}

// handleStats opens the stats view (or restarts its refresh loop).
func handleStats(m *Model) (Model, tea.Cmd) {
	if requireContainer(m, true) == nil {
		return *m, nil
	}
	if m.ViewMode != ViewStats {
		m.Stats = nil
	}
	m.ViewMode = ViewStats
	m.StatsSeq++
	return *m, LoadStatsFunc(m)
}

func handleInspect(m *Model) (Model, tea.Cmd) {
	if requireContainer(m, false) == nil {
		return *m, nil
	}
	m.ViewMode = ViewInspect
	m.InspectData = ""
	m.InspectScroll, m.InspectTop = 0, 0
	return *m, LoadInspectFunc(m)
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
	m.CmdLine.Open(nil)
	m.StatusMessage = ""
	return *m, nil
}

func handleHelp(m *Model) (Model, tea.Cmd) {
	m.HelpMode = true
	m.HelpScroll = 0
	m.StatusMessage = ""
	return *m, nil
}

func handleBack(m *Model) (Model, tea.Cmd) {
	if m.ViewMode == ViewPorts || m.ViewMode == ViewEnv || m.ViewMode == ViewStats || m.ViewMode == ViewInspect {
		m.ViewMode = ViewDetails
		m.SelectedPort = 0
		m.Stats = nil
		m.InspectData = ""
		m.InspectScroll = 0
	}
	m.StatusMessage = ""
	return *m, nil
}

// General handlers

func handleForceQuit(m *Model) (Model, tea.Cmd) {
	QuitFunc(m)
	return *m, tea.Quit
}

// Command handlers

// Commands are the ":" ex-commands. Lookup, aliases, unique prefixes (:hel)
// and completion come from vimkit/cmdline.
var Commands cmdline.Set[*Model, tea.Cmd]

func init() {
	noArg := func(fn func(*Model) tea.Cmd) func(*Model, string) tea.Cmd {
		return func(m *Model, _ string) tea.Cmd { return fn(m) }
	}
	Commands = cmdline.Set[*Model, tea.Cmd]{
		{Name: "start", Aliases: []string{"s"}, Desc: "start selected container", Run: noArg(cmdStart)},
		{Name: "stop", Aliases: []string{"S"}, Desc: "stop selected container", Run: noArg(cmdStop)},
		{Name: "ssh", Args: "<alias|host|local>", Desc: "connect to Docker on another host",
			Complete: completeSSHHosts,
			Run:      func(m *Model, arg string) tea.Cmd { return cmdSSH(m, strings.Fields(arg)) }},
		{Name: "nohlsearch", Aliases: []string{"noh"}, Desc: "clear log search", Run: noArg(cmdNoHighlight)},
		{Name: "help", Aliases: []string{"h"}, Desc: "key and command reference", Run: noArg(cmdHelp)},
		{Name: "quit", Aliases: []string{"q", "q!", "qa", "x"}, Desc: "quit", Run: noArg(cmdQuit)},
	}
}

// completeSSHHosts offers "local", configured aliases and ~/.ssh/config hosts.
func completeSSHHosts(m *Model, arg string) []cmdline.Item {
	hosts := getSSHCompletionCandidates(m)
	sort.Strings(hosts)
	return cmdline.MatchStrings(hosts, arg)
}

func cmdQuit(m *Model) tea.Cmd {
	QuitFunc(m)
	return tea.Quit
}

func cmdStart(m *Model) tea.Cmd {
	c := requireContainer(m, false)
	if c == nil {
		return nil
	}
	m.StatusMessage = "Starting " + c.Name + "..."
	return StartContainerFunc(m)
}

func cmdStop(m *Model) tea.Cmd {
	c := requireContainer(m, false)
	if c == nil {
		return nil
	}
	m.StatusMessage = "Stopping " + c.Name + "..."
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
	handleHelp(m)
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
