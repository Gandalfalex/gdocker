package models

import (
	"gdocker/config"
	"gdocker/ui/logviewer"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/Gandalfalex/vim-kit/cmdline"
	"github.com/Gandalfalex/vim-kit/keys"
	"github.com/Gandalfalex/vim-kit/motion"
	"github.com/moby/moby/client"
)

type Model struct {
	KeyBindings     *config.KeyBindings
	UIConfig        *config.UIConfig
	Containers      []Container
	Standalone      []Container
	Projects        []ComposeGroup
	Items           []ListItem
	Cursor          int
	ListTop         int // First visible list row
	SelectedPort    int // For port selection to open in browser
	Width           int
	Height          int
	NavMode         NavigationMode
	ViewMode        ViewMode
	AutoRefreshSecs int
	LogTailLines    int
	LogViewer       logviewer.Model
	LogSince        time.Time
	DockerHost      string
	HostAliases     map[string]string
	StatusMessage   string
	StatusError     bool     // StatusMessage reports a failure
	Confirm         *Confirm // pending y/n prompt
	DockerClient    *client.Client
	CommandMode     bool         // Whether we're in command mode (:)
	CommandInput    string       // Current command input
	CmdLine         cmdline.Line // ":" history and completion state
	HelpMode        bool         // Whether we're in help view
	HelpScroll      int          // First visible help line
	Stats           *ContainerStats
	StatsSeq        int // Identifies the current stats refresh loop
	EnvScroll       int // Cursor line in env view
	EnvTop          int // First visible env line
	Volumes         []Volume
	Images          []Image
	Networks        []Network
	VolumeFiles     []string   // Files in current volume directory
	VolumePath      string     // Current path in volume
	InspectData     string     // JSON inspect data
	InspectScroll   int        // Cursor line in inspect view
	InspectTop      int        // First visible inspect line
	Keys            keys.Input // count prefix and pending key sequence
	keymaps         *keymaps
	statusSeq       int // bumped per status message, for timed clearing
}

// Confirm is a y/n prompt guarding a destructive command.
type Confirm struct {
	Prompt string
	Cmd    tea.Cmd
}

// ListRows is the number of list rows the left panel shows.
func (m *Model) ListRows() int { return max(m.Height-9, 1) }

// PaneRows is the number of content rows a scrolling detail view shows.
func (m *Model) PaneRows() int { return max(m.Height-10, 1) }

// ListPaneWidth is the outer width of the left panel for a terminal width.
func ListPaneWidth(total int) int { return min(max(total/3, 28), 56) }

// DetailWidth is the inner width of the right panel.
func (m *Model) DetailWidth() int { return m.Width - ListPaneWidth(m.Width) - 2 }

// Visible returns the rows [start, end) of n to draw in h rows so that the
// cursor is on screen. top is the stored scroll offset; it is only adjusted
// when the cursor moved out of view by other means than a motion (a refresh,
// a jump), so scrolling stays where the user left it.
func Visible(cursor, top, n, h int) (start, end int) {
	w := motion.Window{Cursor: cursor, Top: top}
	w.Clamp(n)
	w.Scroll(n, h)
	return w.Top, min(w.Top+h, n)
}

// SelectedItem returns the list item under the cursor.
func (m *Model) SelectedItem() (ListItem, bool) {
	if m.Cursor < 0 || m.Cursor >= len(m.Items) {
		return ListItem{}, false
	}
	return m.Items[m.Cursor], true
}

// SelectedContainer returns the container under the cursor, or nil.
func (m *Model) SelectedContainer() *Container {
	if item, ok := m.SelectedItem(); ok && item.IsContainer {
		return item.Container
	}
	return nil
}

// keymaps caches the keymaps built from KeyBindings.
type keymaps struct{ main, ports *keys.Keymap }

type PortMapping struct {
	PrivatePort uint16
	PublicPort  uint16
	Type        string
	IP          string
}

type ContainerStats struct {
	CPUPerc     string
	MemUsage    string
	MemPerc     string
	NetIO       string
	BlockIO     string
	PIDs        string
	ContainerID string
}

// Container holds container info
type Container struct {
	ID        string
	FullID    string
	Name      string
	Image     string
	State     string
	Status    string
	Project   string
	Created   time.Time
	Ports     []PortMapping
	Env       []string
	EnvLoaded bool
}

// Volume holds volume info
type Volume struct {
	Name       string
	Driver     string
	Mountpoint string
	Created    time.Time
	Labels     map[string]string
	Scope      string
}

// Image holds image info
type Image struct {
	ID       string
	RepoTags []string
	Size     int64
	Created  time.Time
}

// Network holds network info
type Network struct {
	ID       string
	Name     string
	Driver   string
	Scope    string
	Created  time.Time
	Internal bool
	Labels   map[string]string
}

// ComposeGroup groups containers by project
type ComposeGroup struct {
	Name       string
	Containers []Container
	Expanded   bool
}

// ListItem represents an item in the sidebar
type ListItem struct {
	IsProject   bool
	IsContainer bool
	IsVolume    bool
	IsImage     bool
	IsNetwork   bool
	Project     *ComposeGroup
	Container   *Container
	Volume      *Volume
	Image       *Image
	Network     *Network
	Index       int // Index in the projects/containers array
}

// NavigationMode represents what we're viewing in the left panel
type NavigationMode int

const (
	NavContainers NavigationMode = iota
	NavVolumes
	NavImages
	NavNetworks
)

// ViewMode represents what's shown in the right panel
type ViewMode int

const (
	ViewDetails ViewMode = iota
	ViewLogs
	ViewPorts
	ViewEnv
	ViewStats
	ViewVolumeBrowse
	ViewInspect
)

// Messages
type ContainersRefreshedMsg struct {
	Containers []Container
	Recreated  []ContainerRecreated
}

type ContainerRecreated struct {
	Name  string
	OldID string
	NewID string
}

type LogsLoadedMsg struct {
	Lines  []string
	Follow bool
	Since  time.Time
}

// LogsFailedMsg reports that the initial log fetch failed.
type LogsFailedMsg struct {
	Err error
}

type LogLineMsg struct {
	Line string
}

type LogsFollowedMsg struct {
	Lines []string
	Since time.Time
}

type ActionResultMsg struct {
	Message string
	Success bool
}

type StatsLoadedMsg struct {
	Stats *ContainerStats
	Seq   int // Model.StatsSeq when the request was issued
}

type StatsTickMsg struct {
	Seq int
}

// Status, when set, is shown in the status bar (e.g. after a delete).
type VolumesLoadedMsg struct {
	Volumes []Volume
	Status  string
}

type ImagesLoadedMsg struct {
	Images []Image
	Status string
}

type NetworksLoadedMsg struct {
	Networks []Network
}

type InspectLoadedMsg struct {
	Data string
}

type VolumeFilesMsg struct {
	Files []string
	Path  string
}

type EnvLoadedMsg struct {
	ContainerID string
	Env         []string
}

type DockerConnectedMsg struct {
	Client     *client.Client
	Host       string
	Containers []Container
	Volumes    []Volume
	Images     []Image
	Networks   []Network
}

type AutoRefreshTickMsg struct{}

type LogFollowTickMsg struct{}

type LogFollowStoppedMsg struct{}
