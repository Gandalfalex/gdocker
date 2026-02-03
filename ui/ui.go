package ui

import (
	"fmt"
	"gdocker/models"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func init() {
	// Set function pointer to avoid circular imports
	models.RenderViewFunc = RenderView
}

func RenderView(m *models.Model) string {
	if m.Width == 0 {
		return "Loading..."
	}

	// If in help mode, show help overlay
	if m.HelpMode {
		return RenderHelp(m, m.Width, m.Height)
	}

	leftWidth := m.Width / 3
	rightWidth := m.Width - leftWidth - 2

	// Render header with stats
	header := RenderHeader(m, m.Width)

	// Render left panel (container list)
	left := RenderList(m, leftWidth, m.Height-4) // -4 for header and status

	// Render right panel based on view mode
	var right string
	switch m.ViewMode {
	case models.ViewLogs:
		right = RenderLogs(m, rightWidth, m.Height-2)
	case models.ViewPorts:
		right = RenderPorts(m, rightWidth, m.Height-2)
	case models.ViewEnv:
		right = RenderEnv(m, rightWidth, m.Height-2)
	case models.ViewStats:
		right = RenderStats(m, rightWidth, m.Height-2)
	case models.ViewInspect:
		right = RenderInspect(m, rightWidth, m.Height-2)
	default:
		right = RenderDetails(m, rightWidth, m.Height-2)
	}

	// Combine panels
	leftPanel := lipgloss.NewStyle().
		Width(leftWidth).
		Height(m.Height - 4). // -4 for header and status
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#007acc")).
		Render(left)

	rightPanel := lipgloss.NewStyle().
		Width(rightWidth).
		Height(m.Height - 4). // -4 for header and status
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#3e3e3e")).
		Render(right)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)

	// Status bar
	var statusText string

	// Priority 1: Command/search mode (highest priority)
	if m.CommandMode {
		statusText = ":" + m.CommandInput + "█ • enter: execute • esc: cancel"
	} else if m.SearchMode {
		statusText = "?" + m.SearchQuery + "█ • enter: search • esc: cancel"
	} else if m.StatusMessage != "" {
		// Priority 2: Status messages (but not while in command/search mode)
		statusText = m.StatusMessage
	} else {
		// Priority 3: Context-specific shortcuts
		switch m.ViewMode {
		case models.ViewLogs:
			statusText = "j/k: scroll • g/G: top/bottom • ?: search • n/N: next/prev • :noh: clear • esc: back • :: cmd"
		case models.ViewPorts:
			statusText = "j/k: select port • o/enter: open in browser • esc: back • :: cmd"
		case models.ViewEnv:
			statusText = "esc: back • :: cmd"
		case models.ViewStats:
			statusText = "t: refresh • esc: back • :: cmd"
		case models.ViewInspect:
			statusText = "j/k: scroll • g/G: top/bottom • esc: back • :: cmd"
		case models.ViewDetails:
			// Show detailed instructions for the details view
			if m.NavMode == models.NavContainers && m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
				statusText = ":s: start • :S: stop • r: restart • d: delete • l: logs • e: exec • p: ports • v: env • t: stats • i: inspect • :: cmd • :help"
			} else if m.NavMode == models.NavContainers {
				statusText = "1-4: nav • j/k: move • space: expand • :s: start • :S: stop • r: restart • d: del • l: logs • e: exec • :: cmd • :help"
			} else if m.NavMode == models.NavVolumes {
				statusText = "1-4: nav • j/k: move • d: delete • :: cmd • :help"
			} else if m.NavMode == models.NavImages {
				statusText = "1-4: nav • j/k: move • d: delete • :: cmd • :help"
			} else if m.NavMode == models.NavNetworks {
				statusText = "1-4: nav • j/k: move • :: cmd • :help"
			}
		default:
			statusText = "1: containers • 2: volumes • 3: images • 4: networks • j/k: nav • :: cmd • :help • :q: quit"
		}
	}

	statusBar := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#858585")).
		Render(statusText)

	return lipgloss.JoinVertical(lipgloss.Left, header, panels, statusBar)
}

func RenderList(m *models.Model, width, height int) string {
	var s strings.Builder

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
		Foreground(lipgloss.Color("#0e639c")).
		Render(titleText)
	s.WriteString(title + "\n\n")

	if len(m.Items) == 0 {
		s.WriteString(emptyText)
		return s.String()
	}

	// Calculate visible window for scrolling
	maxVisible := height - 4 // Account for title and padding
	start := 0
	end := len(m.Items)

	if len(m.Items) > maxVisible {
		// Center cursor in viewport
		start = m.Cursor - maxVisible/2
		if start < 0 {
			start = 0
		}
		end = start + maxVisible
		if end > len(m.Items) {
			end = len(m.Items)
			start = end - maxVisible
			if start < 0 {
				start = 0
			}
		}
	}

	for i := start; i < end; i++ {
		item := m.Items[i]
		cursor := "  "
		if i == m.Cursor {
			cursor = "> "
		}

		if item.IsProject {
			icon := "▶ "
			if item.Project.Expanded {
				icon = "▼ "
			}

			// Count running containers
			running := 0
			for _, c := range item.Project.Containers {
				if c.State == "running" {
					running++
				}
			}

			projectStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#569cd6")).
				Bold(true)

			line := fmt.Sprintf("%s%s%s ", cursor, icon, projectStyle.Render(item.Project.Name))

			// Add count indicator
			countStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#858585"))
			line += countStyle.Render(fmt.Sprintf("(%d/%d)", running, len(item.Project.Containers)))

			if i == m.Cursor {
				line = lipgloss.NewStyle().
					Background(lipgloss.Color("#094771")).
					Render(line)
			}
			s.WriteString(line + "\n")

		} else if item.IsContainer {
			// Determine status icon and color
			var statusIcon string
			var statusColor lipgloss.Color

			switch item.Container.State {
			case "running":
				statusIcon = "●"
				statusColor = lipgloss.Color("#4ec9b0") // Green
			case "paused":
				statusIcon = "◐"
				statusColor = lipgloss.Color("#dcdcaa") // Yellow
			case "restarting":
				statusIcon = "↻"
				statusColor = lipgloss.Color("#4ec9b0") // Green
			case "exited":
				statusIcon = "■"
				statusColor = lipgloss.Color("#858585") // Gray
			default:
				statusIcon = "●"
				statusColor = lipgloss.Color("#ce9178") // Orange
			}

			statusStyled := lipgloss.NewStyle().
				Foreground(statusColor).
				Render(statusIcon)

			// Indent if part of a project
			indent := ""
			if item.Container.Project != "" {
				indent = "  "
			}

			// Container name style
			nameStyle := lipgloss.NewStyle()
			if item.Container.State != "running" {
				nameStyle = nameStyle.Foreground(lipgloss.Color("#858585"))
			}

			line := fmt.Sprintf("%s%s%s %s", cursor, indent, statusStyled, nameStyle.Render(item.Container.Name))

			if i == m.Cursor {
				line = lipgloss.NewStyle().
					Background(lipgloss.Color("#094771")).
					Render(line)
			}
			s.WriteString(line + "\n")

		} else if item.IsVolume {
			volumeIcon := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#dcdcaa")).
				Render("◉")

			line := fmt.Sprintf("%s%s %s", cursor, volumeIcon, item.Volume.Name)

			if i == m.Cursor {
				line = lipgloss.NewStyle().
					Background(lipgloss.Color("#094771")).
					Bold(true).
					Render(line)
			}
			s.WriteString(line + "\n")

		} else if item.IsImage {
			imageIcon := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#569cd6")).
				Render("▢")

			// Get the first repo tag or show <none>
			name := "<none>"
			if len(item.Image.RepoTags) > 0 && item.Image.RepoTags[0] != "<none>:<none>" {
				name = item.Image.RepoTags[0]
			}

			line := fmt.Sprintf("%s%s %s", cursor, imageIcon, name)

			if i == m.Cursor {
				line = lipgloss.NewStyle().
					Background(lipgloss.Color("#094771")).
					Bold(true).
					Render(line)
			}
			s.WriteString(line + "\n")

		} else if item.IsNetwork {
			networkIcon := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#c586c0")).
				Render("⬡")

			line := fmt.Sprintf("%s%s %s", cursor, networkIcon, item.Network.Name)

			if i == m.Cursor {
				line = lipgloss.NewStyle().
					Background(lipgloss.Color("#094771")).
					Bold(true).
					Render(line)
			}
			s.WriteString(line + "\n")
		}
	}

	return s.String()
}

func RenderDetails(m *models.Model, width, height int) string {
	var s strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#0e639c")).
		Render("Details")
	s.WriteString(title + "\n\n")

	// Get selected item
	if m.Cursor < 0 || m.Cursor >= len(m.Items) {
		s.WriteString("No selection")
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

	} else if item.IsContainer {
		c := item.Container

		s.WriteString(renderLabel("Name") + c.Name + "\n")

		statusColor := lipgloss.Color("#ce9178")
		statusText := "Stopped"
		if c.State == "running" {
			statusColor = lipgloss.Color("#4ec9b0")
			statusText = "Running"
		}
		s.WriteString(renderLabel("Status") + lipgloss.NewStyle().Foreground(statusColor).Render(statusText) + "\n")

		s.WriteString(renderLabel("Image") + c.Image + "\n")
		s.WriteString(renderLabel("ID") + c.ID + "\n")

		if c.Project != "" {
			s.WriteString(renderLabel("Project") + c.Project + "\n")
		}

		s.WriteString(renderLabel("Created") + formatTimeAgo(c.Created) + "\n")

		// Show port summary
		if len(c.Ports) > 0 {
			s.WriteString("\n" + renderLabel("Ports") + fmt.Sprintf("%d mapped", len(c.Ports)) + "\n")
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
			for _, tag := range img.RepoTags {
				if tag != "<none>:<none>" {
					fmt.Fprintf(&s, "  %s\n", tag)
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

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#0e639c")).
		Render("Port Mappings")
	s.WriteString(title + "\n\n")

	if m.Cursor < 0 || m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		s.WriteString("No container selected")
		return s.String()
	}

	c := m.Items[m.Cursor].Container
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
			portStr = fmt.Sprintf("%s:%d -> %d/%s", port.IP, port.PublicPort, port.PrivatePort, port.Type)
		} else {
			portStr = fmt.Sprintf("%d/%s", port.PrivatePort, port.Type)
		}

		line := cursor + portStr
		if i == m.SelectedPort {
			line = lipgloss.NewStyle().
				Background(lipgloss.Color("#094771")).
				Bold(true).
				Render(line)
		}
		s.WriteString(line + "\n")

		// Show URL hint for HTTP ports
		if i == m.SelectedPort && port.PublicPort > 0 && (port.PrivatePort == 80 || port.PrivatePort == 8080 || port.PrivatePort == 3000 || port.PrivatePort == 5000) {
			url := fmt.Sprintf("http://localhost:%d", port.PublicPort)
			s.WriteString("  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#858585")).Render("→ "+url) + "\n")
		}
	}

	s.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#858585")).Render("Press 'o' or 'enter' to open in browser"))

	return s.String()
}

func RenderEnv(m *models.Model, width, height int) string {
	var s strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#0e639c")).
		Render("Environment Variables")
	s.WriteString(title + "\n\n")

	if m.Cursor < 0 || m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		s.WriteString("No container selected")
		return s.String()
	}

	c := m.Items[m.Cursor].Container
	if len(c.Env) == 0 {
		s.WriteString("No environment variables")
		return s.String()
	}

	maxVisible := height - 5
	for i, envVar := range c.Env {
		if i >= maxVisible {
			remaining := len(c.Env) - maxVisible
			s.WriteString(fmt.Sprintf("\n... %d more", remaining))
			break
		}

		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) == 2 {
			key := lipgloss.NewStyle().Foreground(lipgloss.Color("#4ec9b0")).Render(parts[0])
			value := parts[1]
			// Truncate long values
			if len(value) > width-len(parts[0])-10 {
				value = value[:width-len(parts[0])-13] + "..."
			}
			fmt.Fprintf(&s, "%s=%s\n", key, value)
		} else {
			s.WriteString(envVar + "\n")
		}
	}

	return s.String()
}

func RenderLogs(m *models.Model, width, height int) string {
	var s strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#0e639c")).
		Render("Logs")
	s.WriteString(title + "\n\n")

	if len(m.Logs) == 0 {
		s.WriteString("No logs available")
		return s.String()
	}

	// Calculate visible window
	maxVisible := height - 5
	if maxVisible < 1 {
		maxVisible = 1
	}

	// Ensure scroll position is valid
	scrollPos := m.LogScroll
	if scrollPos >= len(m.Logs) {
		scrollPos = len(m.Logs) - 1
	}
	if scrollPos < 0 {
		scrollPos = 0
	}

	// Calculate start position to keep cursor centered
	start := scrollPos - maxVisible/2
	if start < 0 {
		start = 0
	}

	end := start + maxVisible
	if end > len(m.Logs) {
		end = len(m.Logs)
		// Adjust start if we're at the end
		start = end - maxVisible
		if start < 0 {
			start = 0
		}
	}

	// Create a map of search result lines for quick lookup
	searchResultMap := make(map[int]bool)
	for _, idx := range m.SearchResults {
		searchResultMap[idx] = true
	}

	// Show logs in window
	for i := start; i < end; i++ {
		line := m.Logs[i]
		// Truncate long lines to fit width
		if len(line) > width-4 {
			line = line[:width-4] + "..."
		}

		// Highlight search matches in the line
		if m.SearchQuery != "" && searchResultMap[i] {
			line = highlightSearchTerm(line, m.SearchQuery)
		}

		if i == scrollPos {
			// Highlight current line
			line = lipgloss.NewStyle().
				Background(lipgloss.Color("#094771")).
				Render(line)
		}
		s.WriteString(line + "\n")
	}

	// Show position indicator
	s.WriteString("\n")
	indicator := fmt.Sprintf("Line %d/%d (%d-%d)", scrollPos+1, len(m.Logs), start+1, end)
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#858585")).Render(indicator))

	return s.String()
}

func renderLabel(label string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#858585")).
		Bold(true).
		Render(label + ": ")
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

func highlightSearchTerm(line, query string) string {
	if query == "" {
		return line
	}

	lowerLine := strings.ToLower(line)
	lowerQuery := strings.ToLower(query)

	// Find the position of the query in the line
	idx := strings.Index(lowerLine, lowerQuery)
	if idx == -1 {
		return line
	}

	// Highlight all occurrences
	result := ""
	lastIdx := 0

	for idx != -1 {
		// Add text before match
		result += line[lastIdx:idx]

		// Add highlighted match
		match := line[idx : idx+len(query)]
		highlighted := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#ffff00")).
			Render(match)
		result += highlighted

		lastIdx = idx + len(query)
		idx = strings.Index(lowerLine[lastIdx:], lowerQuery)
		if idx != -1 {
			idx += lastIdx
		}
	}

	// Add remaining text
	result += line[lastIdx:]

	return result
}

func RenderStats(m *models.Model, width, height int) string {
	var s strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#0e639c")).
		Render("Container Stats")
	s.WriteString(title + "\n\n")

	if m.Stats == nil {
		s.WriteString("Loading stats...")
		return s.String()
	}

	// CPU Usage
	s.WriteString(renderLabel("CPU Usage") + m.Stats.CPUPerc + "\n")

	// Memory Usage
	s.WriteString(renderLabel("Memory Usage") + m.Stats.MemUsage + "\n")
	s.WriteString(renderLabel("Memory %") + m.Stats.MemPerc + "\n\n")

	// Network I/O
	s.WriteString(renderLabel("Network I/O") + m.Stats.NetIO + "\n")
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#858585")).Render("  (Received / Transmitted)") + "\n\n")

	// Block I/O
	s.WriteString(renderLabel("Block I/O") + m.Stats.BlockIO + "\n")
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#858585")).Render("  (Read / Write)") + "\n\n")

	// PIDs
	s.WriteString(renderLabel("PIDs") + m.Stats.PIDs + "\n")

	s.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#858585")).Render("Press 't' to refresh stats"))

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

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#0e639c")).
		Render("Container Inspect (JSON)")
	s.WriteString(title + "\n\n")

	if m.InspectData == "" {
		s.WriteString("No inspect data loaded")
		return s.String()
	}

	// Split JSON into lines
	lines := strings.Split(m.InspectData, "\n")

	// Calculate visible window
	maxVisible := height - 5
	if maxVisible < 1 {
		maxVisible = 1
	}

	// Ensure scroll position is valid
	scrollPos := m.LogScroll
	if scrollPos >= len(lines) {
		scrollPos = len(lines) - 1
	}
	if scrollPos < 0 {
		scrollPos = 0
	}

	// Calculate start position to keep cursor centered
	start := scrollPos - maxVisible/2
	if start < 0 {
		start = 0
	}

	end := start + maxVisible
	if end > len(lines) {
		end = len(lines)
		start = end - maxVisible
		if start < 0 {
			start = 0
		}
	}

	// Show JSON lines
	for i := start; i < end; i++ {
		line := lines[i]
		// Truncate long lines to fit width
		if len(line) > width-4 {
			line = line[:width-4] + "..."
		}

		if i == scrollPos {
			// Highlight current line
			line = lipgloss.NewStyle().
				Background(lipgloss.Color("#094771")).
				Render(line)
		}
		s.WriteString(line + "\n")
	}

	// Show position indicator
	s.WriteString("\n")
	indicator := fmt.Sprintf("Line %d/%d (%d-%d)", scrollPos+1, len(lines), start+1, end)
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#858585")).Render(indicator))

	return s.String()
}

func RenderHeader(m *models.Model, width int) string {
	var running, stopped, total int

	// Count containers by state
	for _, c := range m.Containers {
		total++
		if c.State == "running" {
			running++
		} else {
			stopped++
		}
	}

	// Create header sections
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#569cd6")).
		Bold(true)

	title := titleStyle.Render("🐳 GDocker")

	// Stats
	runningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4ec9b0"))
	stoppedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#858585"))

	stats := fmt.Sprintf("Containers: %s %s %s",
		runningStyle.Render(fmt.Sprintf("●%d", running)),
		stoppedStyle.Render(fmt.Sprintf("■%d", stopped)),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#858585")).Render(fmt.Sprintf("(%d total)", total)),
	)

	// Resource counts
	resources := fmt.Sprintf("Volumes: %d  Images: %d  Networks: %d",
		len(m.Volumes), len(m.Images), len(m.Networks))
	resourceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#858585"))

	// Combine all parts
	left := lipgloss.JoinHorizontal(lipgloss.Left, title, "  ", stats)
	right := resourceStyle.Render(resources)

	// Calculate spacing
	spacerWidth := width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if spacerWidth < 0 {
		spacerWidth = 0
	}
	spacer := strings.Repeat(" ", spacerWidth)

	header := lipgloss.JoinHorizontal(lipgloss.Left, " ", left, spacer, right, " ")

	return lipgloss.NewStyle().
		Background(lipgloss.Color("#1e1e1e")).
		Foreground(lipgloss.Color("#d4d4d4")).
		Width(width).
		Render(header)
}

func RenderHelp(m *models.Model, width, height int) string {
	var s strings.Builder

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#569cd6")).
		Render("GDocker - Help")

	s.WriteString(title + "\n\n")

	// Navigation section
	s.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4ec9b0")).
		Bold(true).
		Render("Navigation") + "\n")
	s.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#d4d4d4")).
		Render(
			"  1-4          Switch between containers, volumes, images, networks\n" +
				"  j/k, ↓/↑     Move cursor up/down\n" +
				"  g/G          Jump to top/bottom\n" +
				"  space/enter  Toggle project expansion\n",
		))

	s.WriteString("\n")

	// Container actions section
	s.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4ec9b0")).
		Bold(true).
		Render("Container Actions") + "\n")
	s.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#d4d4d4")).
		Render(
			"  :s           Start container\n" +
				"  :S           Stop container\n" +
				"  r            Restart container\n" +
				"  d            Delete container/volume/image\n" +
				"  l            View logs\n" +
				"  e            Execute shell (docker exec)\n" +
				"  p            View port mappings\n" +
				"  v            View environment variables\n" +
				"  t            View stats (refresh with 't')\n" +
				"  i            View inspect (JSON)\n",
		))

	s.WriteString("\n")

	// Logs section
	s.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4ec9b0")).
		Bold(true).
		Render("Logs View") + "\n")
	s.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#d4d4d4")).
		Render(
			"  j/k          Scroll logs up/down\n" +
				"  g/G          Jump to top/bottom\n" +
				"  ?            Search in logs\n" +
				"  n/N          Next/previous search result\n" +
				"  :noh         Clear search highlighting\n",
		))

	s.WriteString("\n")

	// Ports section
	s.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4ec9b0")).
		Bold(true).
		Render("Ports View") + "\n")
	s.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#d4d4d4")).
		Render(
			"  j/k          Select port\n" +
				"  o/enter      Open selected port in browser\n",
		))

	s.WriteString("\n")

	// Commands section
	s.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4ec9b0")).
		Bold(true).
		Render("Commands") + "\n")
	s.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#d4d4d4")).
		Render(
			"  :q           Quit application\n" +
				"  :help        Show this help\n" +
				"  :s           Start container\n" +
				"  :S           Stop container\n" +
				"  :noh         Clear search highlighting\n",
		))

	s.WriteString("\n")

	// General section
	s.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4ec9b0")).
		Bold(true).
		Render("General") + "\n")
	s.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#d4d4d4")).
		Render(
			"  esc          Go back/close view\n" +
				"  ctrl+c       Force quit\n" +
				"  :            Enter command mode\n",
		))

	s.WriteString("\n")
	s.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("#858585")).
		Italic(true).
		Render("Press esc to close this help"))

	helpContent := s.String()

	// Create a centered box for the help content
	helpBox := lipgloss.NewStyle().
		Width(width-4).
		Height(height-2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#569cd6")).
		Padding(1, 2).
		Render(helpContent)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, helpBox)
}
