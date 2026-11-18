package app

import (
	"time"

	"gdocker/config"
	"gdocker/models"
	"gdocker/ui/logviewer"

	tea "charm.land/bubbletea/v2"
	tk "github.com/Gandalfalex/tuikit"
	"github.com/Gandalfalex/vim-kit/keys"
	"github.com/Gandalfalex/vim-kit/motion"
)

type Model struct {
	tk.Shell[models.ViewMode] // status line (expiring), key prefixes, theme

	KeyBindings     *config.KeyBindings
	UIConfig        *config.UIConfig
	Containers      []models.Container
	Standalone      []models.Container
	Projects        []models.ComposeGroup
	Items           []models.ListItem
	Cursor          int
	ListTop         int // First visible list row
	SelectedPort    int // For port selection to open in browser
	Width           int
	Height          int
	NavMode         models.NavigationMode
	ViewMode        models.ViewMode
	AutoRefreshSecs int
	LogTailLines    int
	LogViewer       logviewer.Model
	LogSince        time.Time
	DockerHost      string
	HostAliases     map[string]string
	Confirm         *Confirm               // pending y/n prompt
	Backend         Backend                // the Docker daemon
	prompt          tk.Prompt[int]         // the ":" line
	cmd             tk.CommandLine[*Model] // its commands, history and completion
	ready           bool                   // prompt and cmd are set up
	startup         string                 // status to show once running, e.g. a config error
	HelpMode        bool                   // Whether we're in help view
	HelpScroll      int                    // First visible help line
	Stats           *models.ContainerStats
	StatsSeq        int // Identifies the current stats refresh loop
	EnvScroll       int // Cursor line in env view
	EnvTop          int // First visible env line
	Volumes         []models.Volume
	Images          []models.Image
	Networks        []models.Network
	VolumeFiles     []string // Files in current volume directory
	VolumePath      string   // Current path in volume
	InspectData     string   // JSON inspect data
	InspectScroll   int      // Cursor line in inspect view
	InspectTop      int      // First visible inspect line
	keymaps         *keymaps
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
func (m *Model) SelectedItem() (models.ListItem, bool) {
	if m.Cursor < 0 || m.Cursor >= len(m.Items) {
		return models.ListItem{}, false
	}
	return m.Items[m.Cursor], true
}

// SelectedContainer returns the container under the cursor, or nil.
func (m *Model) SelectedContainer() *models.Container {
	if item, ok := m.SelectedItem(); ok && item.IsContainer {
		return item.Container
	}
	return nil
}

// keymaps caches the keymaps built from KeyBindings.
type keymaps struct{ main, ports *keys.Keymap }

// ensure sets up what the zero Model lacks; Update calls it first.
func (m *Model) ensure() {
	if m.ready {
		return
	}
	m.ready = true
	if m.Stack.Len() == 0 { // the zero Shell
		m.Shell = tk.NewShell(tk.TokyoNight, models.ViewDetails)
	}
	m.StatusTTL = statusTimeout
	m.prompt = tk.NewPrompt[int]()
	m.cmd = tk.CommandLine[*Model]{Commands: Commands, Goto: gotoLine, CompleteOnTab: true}
}

// commandOpen reports whether the ":" line is open.
func (m *Model) commandOpen() bool { return m.prompt.Active() }
