package ui

import (
	"fmt"
	"gdocker/config"
	"gdocker/models"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type helpEntry struct {
	key  string
	desc string
}

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
		BorderForeground(lipgloss.Color(ColorPrimary)).
		Render(left)

	rightPanel := lipgloss.NewStyle().
		Width(rightWidth).
		Height(m.Height - 4). // -4 for header and status
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorBorder)).
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
			follow := "off"
			if m.FollowingLogs {
				follow = "on"
			}
			statusText = "j/k: scroll • g/G: top/bottom • ?: search • n/N: next/prev • f: follow(" + follow + ") • :noh: clear • esc: back • :: cmd"
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
			if len(m.Items) == 0 {
				statusText = "1-4: switch resource • :help: shortcuts • :q: quit"
			} else if m.NavMode == models.NavContainers && m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
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
		Foreground(lipgloss.Color(ColorMuted)).
		Render(statusText)

	return lipgloss.JoinVertical(lipgloss.Left, header, panels, statusBar)
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
		emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted))
		s.WriteString(emptyStyle.Render(emptyText) + "\n\n")
		s.WriteString(emptyStyle.Render("Try:\n"))
		s.WriteString(emptyStyle.Render("• start Docker daemon\n"))
		s.WriteString(emptyStyle.Render("• switch resources with 1-4\n"))
		s.WriteString(emptyStyle.Render("• open :help for all commands"))
		return s.String()
	}

	// Calculate visible window for scrolling
	maxVisible := height - 4 // Account for title and padding
	start := 0
	end := len(m.Items)

	if len(m.Items) > maxVisible {
		// Center cursor in viewport
		start = max(m.Cursor-maxVisible/2, 0)
		end = start + maxVisible
		if end > len(m.Items) {
			end = len(m.Items)
			start = max(end-maxVisible, 0)
		}
	}

	for i := start; i < end; i++ {
		item := m.Items[i]
		cursor := "  "
		if i == m.Cursor {
			cursor = "> "
		}

		if item.IsProject {
			icon := IconCollapsed + " "
			if item.Project.Expanded {
				icon = IconExpanded + " "
			}

			// Count running containers
			running := 0
			for _, c := range item.Project.Containers {
				if c.State == "running" {
					running++
				}
			}

			projectStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorImage)).
				Bold(true)

			line := fmt.Sprintf("%s%s%s ", cursor, icon, projectStyle.Render(item.Project.Name))

			// Add count indicator
			countStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorMuted))
			line += countStyle.Render(fmt.Sprintf("(%d/%d)", running, len(item.Project.Containers)))

			if i == m.Cursor {
				line = lipgloss.NewStyle().
					Background(lipgloss.Color(ColorHighlight)).
					Render(line)
			}
			s.WriteString(line + "\n")

		} else if item.IsContainer {
			// Determine status icon and color
			statusIcon := GetContainerStatusIcon(item.Container.State)
			statusColor := lipgloss.Color(GetContainerStatusColor(item.Container.State))

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
				nameStyle = nameStyle.Foreground(lipgloss.Color(ColorMuted))
			}

			line := fmt.Sprintf("%s%s%s %s", cursor, indent, statusStyled, nameStyle.Render(item.Container.Name))

			if i == m.Cursor {
				line = lipgloss.NewStyle().
					Background(lipgloss.Color(ColorHighlight)).
					Render(line)
			}
			s.WriteString(line + "\n")

		} else if item.IsVolume {
			volumeIcon := lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorVolume)).
				Render(IconVolume)

			line := fmt.Sprintf("%s%s %s", cursor, volumeIcon, item.Volume.Name)

			if i == m.Cursor {
				line = lipgloss.NewStyle().
					Background(lipgloss.Color(ColorHighlight)).
					Bold(true).
					Render(line)
			}
			s.WriteString(line + "\n")

		} else if item.IsImage {
			imageIcon := lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorImage)).
				Render(IconImage)

			// Get the first repo tag or show <none>
			name := "<none>"
			if len(item.Image.RepoTags) > 0 && item.Image.RepoTags[0] != "<none>:<none>" {
				name = item.Image.RepoTags[0]
			}

			line := fmt.Sprintf("%s%s %s", cursor, imageIcon, name)

			if i == m.Cursor {
				line = lipgloss.NewStyle().
					Background(lipgloss.Color(ColorHighlight)).
					Bold(true).
					Render(line)
			}
			s.WriteString(line + "\n")

		} else if item.IsNetwork {
			networkIcon := lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorNetwork)).
				Render(IconNetwork)

			line := fmt.Sprintf("%s%s %s", cursor, networkIcon, item.Network.Name)

			if i == m.Cursor {
				line = lipgloss.NewStyle().
					Background(lipgloss.Color(ColorHighlight)).
					Bold(true).
					Render(line)
			}
			s.WriteString(line + "\n")
		}
	}

	if cfg.ShowListHelpHint {
		s.WriteString("\n")
		s.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMuted)).
			Render(":help for shortcuts"))
	}

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
		s.WriteString(emptyStyle.Render("• 1-4 to switch resources\n"))
		s.WriteString(emptyStyle.Render("• j/k to move cursor\n"))
		s.WriteString(emptyStyle.Render("• :help to open full help"))
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
			Render("Actions: l logs • e exec • p ports • v env • t stats • i inspect") + "\n\n")

		s.WriteString(renderLabel("Name") + c.Name + "\n")

		statusColor := lipgloss.Color(ColorLink)
		statusText := "Stopped"
		if c.State == "running" {
			statusColor = lipgloss.Color(ColorSuccess)
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
	s.WriteString(renderPaneHeader("Port Mappings", fmt.Sprintf("%s • %d published", c.Name, len(c.Ports))))
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

	if m.Cursor < 0 || m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		s.WriteString(renderPaneHeader("Environment Variables", "No container selected"))
		s.WriteString("No container selected")
		return s.String()
	}

	c := m.Items[m.Cursor].Container
	s.WriteString(renderPaneHeader("Environment Variables", fmt.Sprintf("%s • %d vars", c.Name, len(c.Env))))
	if len(c.Env) == 0 {
		s.WriteString("No environment variables")
		return s.String()
	}

	maxVisible := height - 5
	for i, envVar := range c.Env {
		if i >= maxVisible {
			remaining := len(c.Env) - maxVisible
			fmt.Fprintf(&s, "\n... %d more", remaining)
			break
		}

		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) == 2 {
			key := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render(parts[0])
			value := parts[1]
			// Truncate long values
			truncateAt := width - len(parts[0]) - 10
			if truncateAt < 8 {
				truncateAt = 8
			}
			if len(value) > truncateAt {
				value = value[:truncateAt-3] + "..."
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
	cfg := uiConfig(m)

	containerName := "container"
	if m.Cursor >= 0 && m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
		containerName = m.Items[m.Cursor].Container.Name
	}
	mode := "paused"
	if m.FollowingLogs {
		mode = "following"
	}
	s.WriteString(renderPaneHeader("Logs", fmt.Sprintf("%s • %d lines • %s", containerName, len(m.Logs), mode)))

	if len(m.Logs) == 0 {
		s.WriteString("No logs available")
		return s.String()
	}

	if m.SearchQuery != "" {
		searchStatus := fmt.Sprintf("Search: %q (%d matches)", m.SearchQuery, len(m.SearchResults))
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render(searchStatus) + "\n\n")
	}

	// Calculate visible window
	maxVisible := max(height-7, 1)

	// Ensure scroll position is valid
	scrollPos := m.LogScroll
	if scrollPos >= len(m.Logs) {
		scrollPos = len(m.Logs) - 1
	}
	if scrollPos < 0 {
		scrollPos = 0
	}

	// Calculate start position to keep cursor centered
	start := max(scrollPos-maxVisible/2, 0)

	end := start + maxVisible
	if end > len(m.Logs) {
		end = len(m.Logs)
		// Adjust start if we're at the end
		start = max(end-maxVisible, 0)
	}

	// Create a map of search result lines for quick lookup
	searchResultMap := make(map[int]bool)
	for _, idx := range m.SearchResults {
		searchResultMap[idx] = true
	}

	// Show logs in window
	for i := start; i < end; i++ {
		line := m.Logs[i]
		gutter := ""
		if cfg.ShowLineNumbers {
			gutter = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorMuted)).
				Render(fmt.Sprintf("%4d ", i+1))
		}

		// Truncate long lines to fit width
		maxLineWidth := width - 4
		if cfg.ShowLineNumbers {
			maxLineWidth = width - 9
		}
		if maxLineWidth < 12 {
			maxLineWidth = 12
		}
		if len(line) > maxLineWidth {
			line = line[:maxLineWidth-3] + "..."
		}

		// Highlight search matches in the line
		if m.SearchQuery != "" && searchResultMap[i] {
			line = highlightSearchTerm(line, m.SearchQuery)
		}

		if i == scrollPos {
			// Highlight current line
			line = lipgloss.NewStyle().
				Background(lipgloss.Color(ColorHighlight)).
				Render(gutter + line)
		} else {
			line = gutter + line
		}
		s.WriteString(line + "\n")
	}

	// Show position indicator
	s.WriteString("\n")
	progress := int(float64(scrollPos+1) / float64(len(m.Logs)) * 100)
	indicator := fmt.Sprintf("Line %d/%d (%d-%d visible, %d%%)", scrollPos+1, len(m.Logs), start+1, end, progress)
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render(indicator))

	return s.String()
}

func renderLabel(label string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorMuted)).
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
	var result strings.Builder
	lastIdx := 0

	for idx != -1 {
		// Add text before match
		result.WriteString(line[lastIdx:idx])

		// Add highlighted match
		match := line[idx : idx+len(query)]
		highlighted := lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSearchHighlightFG)).
			Background(lipgloss.Color(ColorSearchHighlightBG)).
			Render(match)
		result.WriteString(highlighted)

		lastIdx = idx + len(query)
		idx = strings.Index(lowerLine[lastIdx:], lowerQuery)
		if idx != -1 {
			idx += lastIdx
		}
	}

	// Add remaining text
	result.WriteString(line[lastIdx:])

	return result.String()
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

	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Bold(true).Render("Runtime") + "\n")
	s.WriteString(renderMetricRow("CPU", m.Stats.CPUPerc))
	s.WriteString(renderMetricRow("PIDs", m.Stats.PIDs))
	s.WriteString("\n")

	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Bold(true).Render("Memory") + "\n")
	s.WriteString(renderMetricRow("Usage", m.Stats.MemUsage))
	s.WriteString(renderMetricRow("Percent", m.Stats.MemPerc))
	s.WriteString("\n")

	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Bold(true).Render("I/O") + "\n")
	s.WriteString(renderMetricRow("Network", m.Stats.NetIO))
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("  rx/tx") + "\n")
	s.WriteString(renderMetricRow("Block", m.Stats.BlockIO))
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("  read/write") + "\n")

	s.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render("Press 't' to refresh stats"))

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
		s.WriteString(renderPaneHeader("Container Inspect (JSON)", "No inspect data loaded"))
		s.WriteString("No inspect data loaded")
		return s.String()
	}

	// Split JSON into lines
	lines := strings.Split(m.InspectData, "\n")
	containerName := "container"
	if m.Cursor >= 0 && m.Cursor < len(m.Items) && m.Items[m.Cursor].IsContainer {
		containerName = m.Items[m.Cursor].Container.Name
	}
	s.WriteString(renderPaneHeader("Container Inspect (JSON)", fmt.Sprintf("%s • %d lines", containerName, len(lines))))

	// Calculate visible window
	maxVisible := max(height-7, 1)

	// Ensure scroll position is valid
	scrollPos := m.LogScroll
	if scrollPos >= len(lines) {
		scrollPos = len(lines) - 1
	}
	if scrollPos < 0 {
		scrollPos = 0
	}

	// Calculate start position to keep cursor centered
	start := max(scrollPos-maxVisible/2, 0)

	end := start + maxVisible
	if end > len(lines) {
		end = len(lines)
		start = max(end-maxVisible, 0)
	}

	// Show JSON lines
	for i := start; i < end; i++ {
		line := lines[i]
		gutter := ""
		if cfg.ShowLineNumbers {
			gutter = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorMuted)).
				Render(fmt.Sprintf("%4d ", i+1))
		}
		// Truncate long lines to fit width
		maxLineWidth := width - 4
		if cfg.ShowLineNumbers {
			maxLineWidth = width - 9
		}
		if maxLineWidth < 12 {
			maxLineWidth = 12
		}
		if len(line) > maxLineWidth {
			line = line[:maxLineWidth-3] + "..."
		}

		if i == scrollPos {
			// Highlight current line
			line = lipgloss.NewStyle().
				Background(lipgloss.Color(ColorHighlight)).
				Render(gutter + line)
		} else {
			line = gutter + line
		}
		s.WriteString(line + "\n")
	}

	// Show position indicator
	s.WriteString("\n")
	progress := int(float64(scrollPos+1) / float64(len(lines)) * 100)
	indicator := fmt.Sprintf("Line %d/%d (%d-%d visible, %d%%)", scrollPos+1, len(lines), start+1, end, progress)
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render(indicator))

	return s.String()
}

func RenderHeader(m *models.Model, width int) string {
	var running, stopped, total int
	cfg := uiConfig(m)

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
		Foreground(lipgloss.Color(ColorInfo)).
		Bold(true)

	title := titleStyle.Render("🐳 GDocker")

	// Stats
	runningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
	stoppedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted))

	stats := fmt.Sprintf("Containers: %s %s %s",
		runningStyle.Render(fmt.Sprintf("●%d", running)),
		stoppedStyle.Render(fmt.Sprintf("■%d", stopped)),
		lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render(fmt.Sprintf("(%d total)", total)),
	)

	// Resource counts
	resources := fmt.Sprintf("Volumes: %d  Images: %d  Networks: %d",
		len(m.Volumes), len(m.Images), len(m.Networks))
	resourceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted))

	// Combine all parts
	left := lipgloss.JoinHorizontal(lipgloss.Left, title, "  ", stats)
	if cfg.ShowHeaderContext {
		context := lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMuted)).
			Render(fmt.Sprintf("[%s • %s]", navModeLabel(m.NavMode), viewModeLabel(m.ViewMode)))
		left = lipgloss.JoinHorizontal(lipgloss.Left, title, " ", context, "  ", stats)
	}
	right := resourceStyle.Render(resources)

	// Calculate spacing
	spacerWidth := max(width-lipgloss.Width(left)-lipgloss.Width(right)-4, 0)
	spacer := strings.Repeat(" ", spacerWidth)

	header := lipgloss.JoinHorizontal(lipgloss.Left, " ", left, spacer, right, " ")

	return lipgloss.NewStyle().
		Background(lipgloss.Color(ColorBackground)).
		Foreground(lipgloss.Color(ColorForeground)).
		Width(width).
		Render(header)
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

func RenderHelp(m *models.Model, width, height int) string {
	var s strings.Builder

	s.WriteString(renderPaneHeader("GDocker Help", "Keyboard and command reference"))

	s.WriteString(renderHelpSection("Navigation", []helpEntry{
		{key: "1-4", desc: "Switch between containers, volumes, images, networks"},
		{key: "j/k, ↓/↑", desc: "Move cursor up/down"},
		{key: "g/G", desc: "Jump to top/bottom"},
		{key: "space/enter", desc: "Toggle project expansion"},
	}))
	s.WriteString("\n")

	s.WriteString(renderHelpSection("Container Actions", []helpEntry{
		{key: ":s / :S", desc: "Start/stop container"},
		{key: "r", desc: "Restart container"},
		{key: "d", desc: "Delete container/volume/image"},
		{key: "l", desc: "View logs"},
		{key: "e", desc: "Execute shell (docker exec)"},
		{key: "p", desc: "View port mappings"},
		{key: "v", desc: "View environment variables"},
		{key: "t", desc: "View/refresh stats"},
		{key: "i", desc: "View inspect (JSON)"},
	}))
	s.WriteString("\n")

	s.WriteString(renderHelpSection("Logs View", []helpEntry{
		{key: "j/k", desc: "Scroll logs up/down"},
		{key: "g/G", desc: "Jump to top/bottom"},
		{key: "?", desc: "Search in logs"},
		{key: "n/N", desc: "Next/previous search result"},
		{key: "f", desc: "Toggle live log follow"},
		{key: ":noh", desc: "Clear search highlighting"},
	}))
	s.WriteString("\n")

	s.WriteString(renderHelpSection("Ports View", []helpEntry{
		{key: "j/k", desc: "Select port"},
		{key: "o/enter", desc: "Open selected port in browser"},
	}))
	s.WriteString("\n")

	s.WriteString(renderHelpSection("General", []helpEntry{
		{key: ":q", desc: "Quit application"},
		{key: ":help", desc: "Show this help"},
		{key: "esc", desc: "Go back/close view"},
		{key: "ctrl+c", desc: "Force quit"},
		{key: ":", desc: "Enter command mode"},
	}))
	s.WriteString("\n")
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Italic(true).Render("Press esc to close this help"))

	helpContent := s.String()

	// Create a centered box for the help content
	helpBox := lipgloss.NewStyle().
		Width(width-4).
		Height(height-2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorInfo)).
		Padding(1, 2).
		Render(helpContent)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, helpBox)
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
	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorMuted)).
		Bold(true)
	return fmt.Sprintf("%s%s\n", labelStyle.Render(fmt.Sprintf("  %-8s ", label)), value)
}

func renderHelpSection(title string, entries []helpEntry) string {
	var s strings.Builder
	header := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSuccess)).
		Bold(true).
		Render(title)
	s.WriteString(header + "\n")

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorInfo)).
		Bold(true)
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorForeground))

	for _, entry := range entries {
		key := keyStyle.Render(fmt.Sprintf("  %-12s", entry.key))
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
