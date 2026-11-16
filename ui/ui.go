package ui

import (
	"fmt"
	"gdocker/config"
	"gdocker/models"
	"gdocker/ui/logviewer"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type helpEntry struct {
	key  string
	desc string
}

func init() {
	// Set function pointers to avoid circular imports
	models.RenderViewFunc = RenderView
	models.HelpScrollRangeFunc = helpScrollRange
}

const (
	minWidth  = 50
	minHeight = 12
)

var mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted))

// clip cuts content to a width x height box so that a long name or line can
// never wrap and push the layout out of shape.
func clip(content string, width, height int) string {
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(lines) > height {
		lines = lines[:max(height, 0)]
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "…")
	}
	return strings.Join(lines, "\n")
}

func RenderView(m *models.Model) string {
	if m.Width == 0 {
		return "Loading..."
	}
	if m.Width < minWidth || m.Height < minHeight {
		return lipgloss.Place(m.Width, m.Height, lipgloss.Center, lipgloss.Center,
			mutedStyle.Render(fmt.Sprintf("Terminal too small (need %dx%d)", minWidth, minHeight)))
	}

	// Like s3reader: a status bar with the mode, and the key hints below it.
	footer := renderStatusBar(m) + "\n" + renderHints(m)

	// If in help mode, show help overlay
	if m.HelpMode {
		return lipgloss.JoinVertical(lipgloss.Left, RenderHelp(m, m.Width, m.Height-2), footer)
	}

	// Panels are drawn with a border; their content gets the inner size.
	leftOuter := models.ListPaneWidth(m.Width)
	rightOuter := m.Width - leftOuter
	leftWidth, rightWidth := leftOuter-2, rightOuter-2
	innerHeight := m.Height - 5 // header, status bar, hints and borders

	// Render header with stats
	header := RenderHeader(m, m.Width)

	// Render left panel (container list)
	left := RenderList(m, leftWidth, innerHeight)

	// Render right panel based on view mode
	var right string
	switch m.ViewMode {
	case models.ViewLogs:
		right = m.LogViewer.View(rightWidth, m.Height-3)
	case models.ViewPorts:
		right = RenderPorts(m, rightWidth, m.Height-3)
	case models.ViewEnv:
		right = RenderEnv(m, rightWidth, m.Height-3)
	case models.ViewStats:
		right = RenderStats(m, rightWidth, m.Height-3)
	case models.ViewInspect:
		right = RenderInspect(m, rightWidth, m.Height-3)
	default:
		right = RenderDetails(m, rightWidth, m.Height-3)
	}

	// The panel that receives key presses gets the bright border.
	leftBorder, rightBorder := ColorPrimary, ColorBorder
	if m.ViewMode != models.ViewDetails {
		leftBorder, rightBorder = ColorBorder, ColorPrimary
	}
	panel := func(content string, outerWidth int, border string) string {
		return lipgloss.NewStyle().
			Width(outerWidth).
			Height(innerHeight + 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(border)).
			Render(clip(content, outerWidth-2, innerHeight))
	}
	panels := lipgloss.JoinHorizontal(lipgloss.Top,
		panel(left, leftOuter, leftBorder),
		panel(right, rightOuter, rightBorder))

	return lipgloss.JoinVertical(lipgloss.Left, header, panels, footer)
}

// renderStatusBar renders the bar above the hints: a badge with the input
// mode, then either the line being typed or the state of the current view.
func renderStatusBar(m *models.Model) string {
	bar := lipgloss.NewStyle().Background(lipgloss.Color(ColorBackground))
	seg := func(color, text string) string {
		return bar.Foreground(lipgloss.Color(color)).Render(text)
	}
	badge := func(color, name string) string {
		return lipgloss.NewStyle().Bold(true).Padding(0, 1).
			Foreground(lipgloss.Color(ColorOnAccent)).Background(lipgloss.Color(color)).Render(name)
	}
	logs := m.ViewMode == models.ViewLogs && !m.HelpMode

	var line string
	switch {
	case m.Confirm != nil:
		line = badge(ColorError, "CONFIRM") + seg(ColorForeground, " "+m.Confirm.Prompt)
	case m.CommandMode:
		line = badge(ColorInfo, "COMMAND") + seg(ColorForeground, " :"+m.CommandInput+IconInputCursor)
	case logs && m.LogViewer.IsSearchMode():
		line = badge(ColorWarning, "SEARCH") + seg(ColorForeground, " "+m.LogViewer.SearchPrompt()+IconInputCursor)
	default:
		mode, color := "NORMAL", ColorPrimary
		switch {
		case m.HelpMode:
			mode, color = "HELP", ColorSuccess
		case logs && m.LogViewer.Sel.Mode == logviewer.VisualLine:
			mode, color = "V-LINE", ColorNetwork
		case logs && m.LogViewer.Sel.Mode == logviewer.VisualChar:
			mode, color = "VISUAL", ColorNetwork
		}

		host := "local"
		if m.DockerHost != "" {
			host = m.DockerHost
		}
		parts := []string{seg(ColorForeground, host)}
		if !m.HelpMode {
			parts = append(parts, seg(ColorForeground, navModeLabel(m.NavMode)))
			if n := len(m.Items); n > 0 {
				parts = append(parts, seg(ColorForeground, fmt.Sprintf("%d/%d", m.Cursor+1, n)))
			}
		}
		if logs && m.LogViewer.FollowingLogs {
			parts = append(parts, seg(ColorSuccess, "following"))
		}
		partial := m.Keys.Partial()
		if logs {
			partial = m.LogViewer.Partial()
		}
		if partial != "" {
			parts = append(parts, seg(ColorWarning, partial))
		}
		if m.StatusMessage != "" {
			color := ColorForeground
			if m.StatusError {
				color = ColorError
			}
			parts = append(parts, seg(color, strings.ReplaceAll(m.StatusMessage, "\n", " ")))
		}
		line = badge(color, mode) + seg("", " ") + strings.Join(parts, seg(ColorMuted, " │ "))
	}

	line = ansi.Truncate(line, m.Width, "…")
	return line + bar.Render(strings.Repeat(" ", max(m.Width-lipgloss.Width(line), 0)))
}

// renderHints renders the last line: the keys that do something right now.
func renderHints(m *models.Model) string {
	var text string
	switch {
	case m.Confirm != nil:
		text = "y confirm · any other key cancel"
	case m.CommandMode:
		text = commandHint(m)
	case m.HelpMode:
		text = "j/k scroll · gg/G top/bottom · esc/q/? close help"
	default:
		text = statusHints(m)
	}
	return mutedStyle.Render(ansi.Truncate(" "+text, m.Width, "…"))
}

// commandHint shows the completion candidates while cycling through them
// (the selected one in brackets), else the keys of the command line.
func commandHint(m *models.Model) string {
	if !m.CmdLine.Cycling() {
		return "tab/S-tab complete · ↑↓ history · enter run · esc cancel"
	}
	// Start at the selection so it stays visible when the line is cut off.
	items := m.CmdLine.Items
	var parts []string
	for i := m.CmdLine.Index; i < len(items); i++ {
		label := items[i].Label
		if i == m.CmdLine.Index {
			label = "[" + label + "]"
		}
		parts = append(parts, label)
	}
	return fmt.Sprintf("%d/%d  %s", m.CmdLine.Index+1, len(items), strings.Join(parts, "  "))
}

// statusHints lists the keys that do something in the current context.
func statusHints(m *models.Model) string {
	switch m.ViewMode {
	case models.ViewLogs:
		return m.LogViewer.StatusBarHint()
	case models.ViewPorts:
		return "j/k select port · o/enter open in browser · esc back · ? help"
	case models.ViewEnv, models.ViewInspect:
		return "j/k gg/G ^d/^u scroll · esc back · : command · ? help"
	case models.ViewStats:
		return "refreshes every 2s · t refresh now · esc back · ? help"
	}

	item, ok := m.SelectedItem()
	switch {
	case !ok:
		return "1-4 switch resource · : command · ? help · :q quit"
	case item.IsProject:
		return "space expand/collapse · j/k move · 1-4 switch resource · : command · ? help · :q quit"
	case item.IsContainer:
		return "l logs · e shell · p ports · v env · t stats · i inspect · r restart · d delete · :s/:S start/stop · ? help · :q quit"
	case item.IsVolume, item.IsImage:
		return "j/k move · d delete · 1-4 switch resource · : command · ? help · :q quit"
	}
	return "j/k move · 1-4 switch resource · : command · ? help · :q quit"
}

func RenderList(m *models.Model, width, height int) string {
	var s strings.Builder
	cfg := uiConfig(m)

	// Show appropriate title based on navigation mode
	var titleText string
	var emptyText string
	switch m.NavMode {
	case models.NavContainers:
		titleText = "Containers [1]"
		emptyText = "No containers found"
	case models.NavVolumes:
		titleText = "Volumes [2]"
		emptyText = "No volumes found"
	case models.NavImages:
		titleText = "Images [3]"
		emptyText = "No images found"
	case models.NavNetworks:
		titleText = "Networks [4]"
		emptyText = "No networks found"
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorTitle)).
		Render(titleText)
	s.WriteString(title + "\n\n")

	if len(m.Items) == 0 {
		s.WriteString(mutedStyle.Render(emptyText) + "\n\n")
		s.WriteString(mutedStyle.Render("· switch resources with 1-4") + "\n")
		s.WriteString(mutedStyle.Render("· press ? for all commands"))
		return s.String()
	}

	maxVisible := max(height-4, 1) // Account for title and footer
	start, end := models.Visible(m.Cursor, m.ListTop, len(m.Items), maxVisible)

	for i := start; i < end; i++ {
		item := m.Items[i]
		selected := i == m.Cursor

		// Every segment carries the row background itself: a nested style
		// would reset it and leave the highlight in patches.
		row := lipgloss.NewStyle()
		cursor := IconNoCursor + " "
		if selected {
			row = row.Background(lipgloss.Color(ColorHighlight)).Bold(true)
			cursor = IconCursor + " "
		}
		seg := func(color, text string) string {
			if color == "" {
				return row.Render(text)
			}
			return row.Foreground(lipgloss.Color(color)).Render(text)
		}

		line := seg("", cursor)
		switch {
		case item.IsProject:
			icon := IconCollapsed
			if item.Project.Expanded {
				icon = IconExpanded
			}
			running := 0
			for _, c := range item.Project.Containers {
				if c.State == "running" {
					running++
				}
			}
			line += seg("", icon+" ") + seg(ColorImage, item.Project.Name) +
				seg(ColorMuted, fmt.Sprintf(" (%d/%d)", running, len(item.Project.Containers)))

		case item.IsContainer:
			if item.Container.Project != "" {
				line += seg("", "  ") // indent members of a project
			}
			nameColor := ""
			if item.Container.State != "running" && !selected {
				nameColor = ColorMuted
			}
			line += seg(GetContainerStatusColor(item.Container.State), GetContainerStatusIcon(item.Container.State)) +
				seg(nameColor, " "+item.Container.Name)

		case item.IsVolume:
			line += seg(ColorVolume, IconVolume) + seg("", " "+item.Volume.Name)

		case item.IsImage:
			// Get the first repo tag or show <none>
			name := "<none>"
			if len(item.Image.RepoTags) > 0 && item.Image.RepoTags[0] != "<none>:<none>" {
				name = item.Image.RepoTags[0]
			}
			line += seg(ColorImage, IconImage) + seg("", " "+name)

		case item.IsNetwork:
			line += seg(ColorNetwork, IconNetwork) + seg("", " "+item.Network.Name)
		}

		line = ansi.Truncate(line, width, "…")
		if pad := width - lipgloss.Width(line); selected && pad > 0 {
			line += seg("", strings.Repeat(" ", pad))
		}
		s.WriteString(line + "\n")
	}

	// Pin the footer to the bottom of the panel.
	s.WriteString(strings.Repeat("\n", maxVisible-(end-start)))
	footer := fmt.Sprintf("%d/%d", m.Cursor+1, len(m.Items))
	if cfg.ShowListHelpHint {
		footer += " · ? for help"
	}
	s.WriteString("\n" + mutedStyle.Render(footer))

	return s.String()
}

func RenderDetails(m *models.Model, width, height int) string {
	var s strings.Builder
	cfg := uiConfig(m)

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorTitle)).
		Render("Details")
	s.WriteString(title + "\n\n")

	// Get selected item
	if m.Cursor < 0 || m.Cursor >= len(m.Items) {
		emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted))
		s.WriteString(emptyStyle.Render("No selection") + "\n\n")
		s.WriteString(emptyStyle.Render("Quick start:\n"))
		s.WriteString(emptyStyle.Render("· 1-4 to switch resources\n"))
		s.WriteString(emptyStyle.Render("· j/k to move cursor\n"))
		s.WriteString(emptyStyle.Render("· :help to open full help"))
		return s.String()
	}

	item := m.Items[m.Cursor]

	if item.IsProject {
		s.WriteString(renderLabel("Project") + item.Project.Name + "\n")
		s.WriteString(renderLabel("Containers") + fmt.Sprintf("%d", len(item.Project.Containers)) + "\n\n")

		running := 0
		for _, c := range item.Project.Containers {
			if c.State == "running" {
				running++
			}
		}
		s.WriteString(renderLabel("Running") + fmt.Sprintf("%d/%d", running, len(item.Project.Containers)) + "\n")
		s.WriteString("\n")
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("Containers in project") + "\n")
		for i, c := range item.Project.Containers {
			if i >= cfg.MaxProjectPreviewItems {
				s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("...more containers omitted") + "\n")
				break
			}
			status := lipgloss.NewStyle().
				Foreground(lipgloss.Color(GetContainerStatusColor(c.State))).
				Render(GetContainerStatusIcon(c.State))
			s.WriteString(fmt.Sprintf("  %s %s\n", status, c.Name))
		}

	} else if item.IsContainer {
		c := item.Container

		s.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMuted)).
			Render("l logs · e shell · p ports · v env · t stats · i inspect") + "\n\n")

		s.WriteString(renderLabel("Name") + c.Name + "\n")

		// Docker's own status line ("Up 2 hours", "Exited (1) 3 days ago")
		statusText := c.Status
		if statusText == "" {
			statusText = c.State
		}
		statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(GetContainerStatusColor(c.State)))
		s.WriteString(renderLabel("Status") + statusStyle.Render(statusText) + "\n")

		s.WriteString(renderLabel("Image") + c.Image + "\n")
		s.WriteString(renderLabel("ID") + c.ID + "\n")

		if c.Project != "" {
			s.WriteString(renderLabel("Project") + c.Project + "\n")
		}

		s.WriteString(renderLabel("Created") + formatTimeAgo(c.Created) + "\n")

		// Show port summary
		if len(c.Ports) > 0 {
			s.WriteString("\n" + renderLabel("Ports") + fmt.Sprintf("%d mapped", len(c.Ports)) + "\n")
			for i, p := range c.Ports {
				if i >= cfg.MaxContainerPortPreview {
					s.WriteString("  ...\n")
					break
				}
				if p.PublicPort > 0 {
					host := "localhost"
					if p.IP != "" && p.IP != "0.0.0.0" {
						host = p.IP
					}
					s.WriteString(fmt.Sprintf("  %s:%d -> %d/%s\n", host, p.PublicPort, p.PrivatePort, p.Type))
				} else {
					s.WriteString(fmt.Sprintf("  %d/%s\n", p.PrivatePort, p.Type))
				}
			}
		}

	} else if item.IsVolume {
		v := item.Volume

		s.WriteString(renderLabel("Name") + v.Name + "\n")
		s.WriteString(renderLabel("Driver") + v.Driver + "\n")
		s.WriteString(renderLabel("Mountpoint") + v.Mountpoint + "\n")
		s.WriteString(renderLabel("Scope") + v.Scope + "\n")
		s.WriteString(renderLabel("Created") + formatTimeAgo(v.Created) + "\n")

		if len(v.Labels) > 0 {
			s.WriteString("\n" + renderLabel("Labels") + "\n")
			for k, val := range v.Labels {
				fmt.Fprintf(&s, "  %s=%s\n", k, val)
			}
		}

	} else if item.IsImage {
		img := item.Image

		s.WriteString(renderLabel("ID") + img.ID + "\n")

		if len(img.RepoTags) > 0 {
			s.WriteString(renderLabel("Tags") + "\n")
			tagCount := 0
			for _, tag := range img.RepoTags {
				if tag != "<none>:<none>" {
					fmt.Fprintf(&s, "  %s\n", tag)
					tagCount++
					if tagCount >= cfg.MaxImageTagPreview {
						s.WriteString("  ...\n")
						break
					}
				}
			}
		}

		s.WriteString(renderLabel("Size") + formatBytes(uint64(img.Size)) + "\n")
		s.WriteString(renderLabel("Created") + formatTimeAgo(img.Created) + "\n")

	} else if item.IsNetwork {
		net := item.Network

		s.WriteString(renderLabel("Name") + net.Name + "\n")
		s.WriteString(renderLabel("ID") + net.ID + "\n")
		s.WriteString(renderLabel("Driver") + net.Driver + "\n")
		s.WriteString(renderLabel("Scope") + net.Scope + "\n")

		internalStr := "No"
		if net.Internal {
			internalStr = "Yes"
		}
		s.WriteString(renderLabel("Internal") + internalStr + "\n")
		s.WriteString(renderLabel("Created") + formatTimeAgo(net.Created) + "\n")

		if len(net.Labels) > 0 {
			s.WriteString("\n" + renderLabel("Labels") + "\n")
			for k, val := range net.Labels {
				fmt.Fprintf(&s, "  %s=%s\n", k, val)
			}
		}
	}

	return s.String()
}

func RenderPorts(m *models.Model, width, height int) string {
	var s strings.Builder

	if m.Cursor < 0 || m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		s.WriteString(renderPaneHeader("Port Mappings", "No container selected"))
		s.WriteString("No container selected")
		return s.String()
	}

	c := m.Items[m.Cursor].Container
	s.WriteString(renderPaneHeader("Port Mappings", fmt.Sprintf("%s · %d published", c.Name, len(c.Ports))))
	if len(c.Ports) == 0 {
		s.WriteString("No ports mapped")
		return s.String()
	}

	for i, port := range c.Ports {
		cursor := "  "
		if i == m.SelectedPort {
			cursor = "> "
		}

		var portStr string
		if port.PublicPort > 0 {
			host := "localhost"
			if port.IP != "" && port.IP != "0.0.0.0" {
				host = port.IP
			}
			portStr = fmt.Sprintf("%s:%d -> %d/%s", host, port.PublicPort, port.PrivatePort, port.Type)
		} else {
			portStr = fmt.Sprintf("%d/%s", port.PrivatePort, port.Type)
		}

		line := cursor + portStr
		if i == m.SelectedPort {
			line = lipgloss.NewStyle().
				Background(lipgloss.Color(ColorHighlight)).
				Bold(true).
				Render(line)
		}
		s.WriteString(line + "\n")

		// Show URL hint for HTTP ports
		if i == m.SelectedPort && port.PublicPort > 0 && (port.PrivatePort == 80 || port.PrivatePort == 8080 || port.PrivatePort == 3000 || port.PrivatePort == 5000) {
			url := fmt.Sprintf("http://localhost:%d", port.PublicPort)
			s.WriteString("  " + lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("→ "+url) + "\n")
		}
	}

	s.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("Press 'o' or 'enter' to open in browser"))

	return s.String()
}

func RenderEnv(m *models.Model, width, height int) string {
	var s strings.Builder

	c := m.SelectedContainer()
	if c == nil {
		s.WriteString(renderPaneHeader("Environment Variables", "No container selected"))
		return s.String()
	}

	s.WriteString(renderPaneHeader("Environment Variables", fmt.Sprintf("%s · %d vars", c.Name, len(c.Env))))
	if !c.EnvLoaded {
		s.WriteString(mutedStyle.Render("Loading environment variables..."))
		return s.String()
	}
	if len(c.Env) == 0 {
		s.WriteString(mutedStyle.Render("No environment variables"))
		return s.String()
	}

	maxVisible := max(height-7, 1)
	scrollPos := min(max(m.EnvScroll, 0), len(c.Env)-1)
	start, end := models.Visible(scrollPos, m.EnvTop, len(c.Env), maxVisible)

	for i := start; i < end; i++ {
		row := lipgloss.NewStyle()
		if i == scrollPos {
			row = row.Background(lipgloss.Color(ColorHighlight))
		}
		key, value, hasValue := strings.Cut(c.Env[i], "=")
		line := row.Foreground(lipgloss.Color(ColorSuccess)).Render(key)
		if hasValue {
			line += row.Render("=" + value)
		}
		line = ansi.Truncate(line, width, "…")
		if pad := width - lipgloss.Width(line); i == scrollPos && pad > 0 {
			line += row.Render(strings.Repeat(" ", pad))
		}
		s.WriteString(line + "\n")
	}

	s.WriteString("\n" + mutedStyle.Render(fmt.Sprintf("Variable %d/%d", scrollPos+1, len(c.Env))))

	return s.String()
}

// renderLabel renders a field name in a fixed-width column, so the values
// line up.
func renderLabel(label string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorLabel)).
		Width(12).
		Render(label)
}

func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)
	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		mins := int(duration.Minutes())
		return fmt.Sprintf("%d minutes ago", mins)
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	}
}

func RenderStats(m *models.Model, width, height int) string {
	var s strings.Builder

	containerName := "container"
	if m.Cursor >= 0 && m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
		containerName = m.Items[m.Cursor].Container.Name
	}
	s.WriteString(renderPaneHeader("Container Stats", containerName))

	if m.Stats == nil {
		s.WriteString("Loading stats...")
		return s.String()
	}

	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSection)).Bold(true).Render("Runtime") + "\n")
	s.WriteString(renderMetricRow("CPU", m.Stats.CPUPerc))
	s.WriteString(renderMetricRow("PIDs", m.Stats.PIDs))
	s.WriteString("\n")

	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSection)).Bold(true).Render("Memory") + "\n")
	s.WriteString(renderMetricRow("Usage", m.Stats.MemUsage))
	s.WriteString(renderMetricRow("Percent", m.Stats.MemPerc))
	s.WriteString("\n")

	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSection)).Bold(true).Render("I/O") + "\n")
	s.WriteString(renderMetricRow("Network", m.Stats.NetIO))
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("  rx/tx") + "\n")
	s.WriteString(renderMetricRow("Block", m.Stats.BlockIO))
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("  read/write") + "\n")

	s.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("Refreshes automatically · t to refresh now"))

	return s.String()
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func RenderInspect(m *models.Model, width, height int) string {
	var s strings.Builder
	cfg := uiConfig(m)

	if m.InspectData == "" {
		s.WriteString(renderPaneHeader("Container Inspect (JSON)", ""))
		s.WriteString(mutedStyle.Render("Loading inspect data..."))
		return s.String()
	}

	// Split JSON into lines
	lines := strings.Split(m.InspectData, "\n")
	containerName := "container"
	if m.Cursor >= 0 && m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
		containerName = m.Items[m.Cursor].Container.Name
	}
	s.WriteString(renderPaneHeader("Container Inspect (JSON)", fmt.Sprintf("%s · %d lines", containerName, len(lines))))

	// Calculate visible window
	maxVisible := max(height-7, 1)

	// Ensure scroll position is valid
	scrollPos := m.InspectScroll
	if scrollPos >= len(lines) {
		scrollPos = len(lines) - 1
	}
	if scrollPos < 0 {
		scrollPos = 0
	}

	start, end := models.Visible(scrollPos, m.InspectTop, len(lines), maxVisible)

	// Show JSON lines
	gutterWidth := 0
	if cfg.ShowLineNumbers {
		gutterWidth = 5
	}
	for i := start; i < end; i++ {
		row := lipgloss.NewStyle()
		if i == scrollPos {
			// Highlight current line
			row = row.Background(lipgloss.Color(ColorHighlight))
		}
		gutter := ""
		if cfg.ShowLineNumbers {
			gutter = row.Foreground(lipgloss.Color(ColorMuted)).Render(fmt.Sprintf("%4d ", i+1))
		}
		// Truncate long lines to fit width
		line := ansi.Truncate(strings.ReplaceAll(lines[i], "\t", "  "), max(width-gutterWidth, 12), "…")
		if pad := width - gutterWidth - lipgloss.Width(line); i == scrollPos && pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		s.WriteString(gutter + row.Render(line) + "\n")
	}

	// Show position indicator
	s.WriteString("\n")
	progress := int(float64(scrollPos+1) / float64(len(lines)) * 100)
	indicator := fmt.Sprintf("Line %d/%d (%d-%d visible, %d%%)", scrollPos+1, len(lines), start+1, end, progress)
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render(indicator))

	return s.String()
}

// RenderHeader renders the title line in s3reader's style: a badge, then a
// breadcrumb of where we are, with the resource counts on the right.
func RenderHeader(m *models.Model, width int) string {
	cfg := uiConfig(m)
	crumb := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary))

	parts := []string{lipgloss.NewStyle().Bold(true).Padding(0, 1).
		Foreground(lipgloss.Color(ColorOnAccent)).Background(lipgloss.Color(ColorPrimary)).Render("gdocker")}
	if cfg.ShowHeaderContext {
		parts = append(parts, crumb.Render(navModeLabel(m.NavMode)))
		if item, ok := m.SelectedItem(); ok {
			switch {
			case item.IsProject:
				parts = append(parts, crumb.Render(item.Project.Name))
			case item.IsContainer:
				if item.Container.Project != "" {
					parts = append(parts, crumb.Render(item.Container.Project))
				}
				parts = append(parts, crumb.Render(item.Container.Name))
			}
		}
		if m.ViewMode != models.ViewDetails {
			parts = append(parts, mutedStyle.Render(viewModeLabel(m.ViewMode)))
		}
	}
	left := strings.Join(parts, mutedStyle.Render(" › "))

	var running, stopped int
	for _, c := range m.Containers {
		if c.State == "running" {
			running++
		} else {
			stopped++
		}
	}
	right := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render(fmt.Sprintf("%s %d", IconContainerRunning, running)) +
		mutedStyle.Render(fmt.Sprintf("  %s %d  ·  %d volumes  ·  %d images  ·  %d networks",
			IconContainerExited, stopped, len(m.Volumes), len(m.Images), len(m.Networks)))

	// Drop the counts when the terminal is too narrow for both.
	spacer := width - lipgloss.Width(left) - lipgloss.Width(right) - 1
	if spacer < 2 {
		return ansi.Truncate(left, width, "…")
	}
	return left + strings.Repeat(" ", spacer) + right
}

func navModeLabel(mode models.NavigationMode) string {
	switch mode {
	case models.NavContainers:
		return "containers"
	case models.NavVolumes:
		return "volumes"
	case models.NavImages:
		return "images"
	case models.NavNetworks:
		return "networks"
	default:
		return "unknown"
	}
}

func viewModeLabel(mode models.ViewMode) string {
	switch mode {
	case models.ViewDetails:
		return "details"
	case models.ViewLogs:
		return "logs"
	case models.ViewPorts:
		return "ports"
	case models.ViewEnv:
		return "env"
	case models.ViewStats:
		return "stats"
	case models.ViewVolumeBrowse:
		return "volume"
	case models.ViewInspect:
		return "inspect"
	default:
		return "unknown"
	}
}

// keyList formats the keys bound to an action for the help view.
func keyList(bindings ...[]string) string {
	var parts []string
	for _, b := range bindings {
		if len(b) > 0 {
			parts = append(parts, strings.Join(b, ", "))
		}
	}
	if len(parts) == 0 {
		return "(unbound)"
	}
	return strings.Join(parts, " / ")
}

// helpLines builds the help text from the active key bindings, so rebound
// keys show up correctly.
func helpLines(m *models.Model) []string {
	kb := m.KeyBindings
	if kb == nil {
		kb = config.Default()
	}

	var s strings.Builder
	s.WriteString(renderHelpSection("Navigation", []helpEntry{
		{key: keyList(kb.Navigation.SwitchContainer, kb.Navigation.SwitchVolume, kb.Navigation.SwitchImage, kb.Navigation.SwitchNetwork), desc: "Show containers / volumes / images / networks"},
		{key: keyList(kb.Navigation.Down, kb.Navigation.Up), desc: "Move down / up (with count: 5j)"},
		{key: keyList(kb.Navigation.Top, kb.Navigation.Bottom), desc: "Jump to top / bottom"},
		{key: "ctrl+d / ctrl+u", desc: "Half page down / up"},
		{key: "H / M / L", desc: "Top / middle / bottom of screen"},
		{key: "zz / zt / zb", desc: "Put cursor line at center / top / bottom"},
		{key: "ctrl+e / ctrl+y", desc: "Scroll view without moving the cursor"},
		{key: keyList(kb.Navigation.ToggleExpand), desc: "Expand or collapse compose project"},
	}))
	s.WriteString("\n")

	s.WriteString(renderHelpSection("Container Actions", []helpEntry{
		{key: ":s / :S", desc: "Start / stop container"},
		{key: keyList(kb.Container.Restart), desc: "Restart container"},
		{key: keyList(kb.Container.Delete), desc: "Delete container, volume or image (asks first)"},
		{key: keyList(kb.Container.Logs), desc: "View logs"},
		{key: keyList(kb.Container.Exec), desc: "Open a shell in the container"},
		{key: keyList(kb.Container.Ports), desc: "View port mappings"},
		{key: keyList(kb.Container.Env), desc: "View environment variables"},
		{key: keyList(kb.Container.Stats), desc: "View live stats"},
		{key: keyList(kb.Container.Inspect), desc: "View inspect (JSON)"},
	}))
	s.WriteString("\n")

	s.WriteString(renderHelpSection("Logs View", []helpEntry{
		{key: keyList(kb.Logs.Search), desc: "Search in logs"},
		{key: keyList(kb.Logs.NextResult, kb.Logs.PrevResult), desc: "Next / previous search result"},
		{key: keyList(kb.Logs.Follow), desc: "Toggle live log follow"},
		{key: keyList(kb.Logs.ToggleWrap), desc: "Toggle line wrapping"},
		{key: keyList(kb.Logs.VisualLine), desc: "Select lines"},
		{key: keyList(kb.Logs.CharVisual), desc: "Select characters (h/l, w/b/e, 0/$)"},
		{key: keyList(kb.Logs.Yank), desc: "Copy current line or selection"},
	}))
	s.WriteString("\n")

	s.WriteString(renderHelpSection("Ports View", []helpEntry{
		{key: keyList(kb.Container.OpenPort), desc: "Open selected port in browser"},
	}))
	s.WriteString("\n")

	s.WriteString(renderHelpSection("General", []helpEntry{
		{key: keyList(kb.Views.Back), desc: "Go back / close view"},
		{key: keyList(kb.General.Help), desc: "Show this help"},
		{key: keyList(kb.General.ForceQuit), desc: "Quit immediately"},
	}))
	s.WriteString("\n")

	// Generated from the command set, so it cannot go stale.
	commands := []helpEntry{{key: keyList(kb.Commands.Enter), desc: "Enter command mode (tab completes, ↑↓ history, :hel = :help)"}}
	for _, c := range models.Commands {
		commands = append(commands, helpEntry{key: ":" + c.Usage(), desc: c.Desc})
	}
	commands = append(commands, helpEntry{key: ":42", desc: "jump to line 42"})
	s.WriteString(renderHelpSection("Commands", commands))

	return strings.Split(strings.TrimRight(s.String(), "\n"), "\n")
}

// helpVisibleRows is the number of help lines that fit a help box of the
// given outer height (border and pane header take the rest).
func helpVisibleRows(height int) int {
	return max(height-5, 1)
}

// helpScrollRange reports how many scroll positions the help view has.
func helpScrollRange(m *models.Model) int {
	return max(len(helpLines(m))-helpVisibleRows(m.Height-2)+1, 1)
}

func RenderHelp(m *models.Model, width, height int) string {
	lines := helpLines(m)
	visible := helpVisibleRows(height)
	start := min(max(m.HelpScroll, 0), max(len(lines)-visible, 0))
	end := min(start+visible, len(lines))

	subtitle := "Keyboard and command reference"
	if len(lines) > visible {
		subtitle += fmt.Sprintf(" · lines %d-%d of %d", start+1, end, len(lines))
	}
	content := renderPaneHeader("GDocker Help", subtitle) + strings.Join(lines[start:end], "\n")

	// Border and padding count towards Width/Height.
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorPrimary)).
		Padding(0, 2).
		Render(clip(content, width-6, height-2))
}

func renderPaneHeader(title, subtitle string) string {
	titleStyled := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorTitle)).
		Render(title)

	if subtitle == "" {
		return titleStyled + "\n\n"
	}

	subtitleStyled := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorMuted)).
		Render(subtitle)

	return titleStyled + "\n" + subtitleStyled + "\n\n"
}

func renderMetricRow(label, value string) string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorLabel))
	return fmt.Sprintf("%s%s\n", labelStyle.Render(fmt.Sprintf("  %-10s", label)), value)
}

func renderHelpSection(title string, entries []helpEntry) string {
	var s strings.Builder
	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSection)).
		Bold(true).
		Render(title)
	s.WriteString(header + "\n")

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorPrimary))
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorForeground))

	for _, entry := range entries {
		key := keyStyle.Render(fmt.Sprintf("  %-26s", entry.key))
		s.WriteString(key + " " + textStyle.Render(entry.desc) + "\n")
	}

	return s.String()
}

func uiConfig(m *models.Model) config.UIConfig {
	if m != nil && m.UIConfig != nil {
		return *m.UIConfig
	}
	return *config.DefaultUI()
}
