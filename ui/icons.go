package ui

// Container status icons
const (
	IconContainerRunning    = "●"
	IconContainerPaused     = "◐"
	IconContainerRestarting = "↻"
	IconContainerExited     = "■"
	IconContainerDefault    = "●"
)

// Resource type icons
const (
	IconVolume  = "◉"
	IconImage   = "▢"
	IconNetwork = "⬡"
	IconDocker  = "🐳"
)

// UI icons
const (
	IconExpanded    = "▼"
	IconCollapsed   = "▶"
	IconCursor      = "▌"
	IconNoCursor    = " "
	IconInputCursor = "█"
)

// Colors: the Tokyo Night palette, shared with s3reader.
const (
	ColorRunning    = "#9ece6a" // Green
	ColorPaused     = "#e0af68" // Yellow
	ColorRestarting = "#7dcfff" // Cyan
	ColorExited     = "#565f89" // Gray
	ColorDefault    = "#ff9e64" // Orange

	ColorVolume  = "#e0af68" // Yellow
	ColorImage   = "#7dcfff" // Cyan
	ColorNetwork = "#bb9af7" // Purple

	ColorPrimary    = "#7aa2f7" // Accent blue
	ColorSecondary  = "#3b4261" // Dark gray
	ColorBorder     = "#3b4261" // Dark gray
	ColorBackground = "#1f2335" // Bar background
	ColorForeground = "#c0caf5" // Light gray
	ColorMuted      = "#565f89" // Gray
	ColorSuccess    = "#9ece6a" // Green
	ColorWarning    = "#e0af68" // Yellow
	ColorError      = "#f7768e" // Red
	ColorInfo       = "#7dcfff" // Cyan
	ColorLabel      = "#7aa2f7" // Field labels
	ColorTitle      = "#7aa2f7" // Pane titles
	ColorSection    = "#e0af68" // Section headings
	ColorHighlight  = "#283457" // Selection/highlight background
	ColorLink       = "#ff9e64" // Link orange
	ColorOnAccent   = "#1a1b26" // Text on a colored badge

	ColorSearchHighlightFG = "#1a1b26" // Search highlight foreground
	ColorSearchHighlightBG = "#e0af68" // Search highlight background

	ColorVisualSelectionBG = "#3d59a1" // Visual selection background
)

// GetContainerStatusIcon returns the icon for a container status
func GetContainerStatusIcon(status string) string {
	switch status {
	case "running":
		return IconContainerRunning
	case "paused":
		return IconContainerPaused
	case "restarting":
		return IconContainerRestarting
	case "exited":
		return IconContainerExited
	default:
		return IconContainerDefault
	}
}

// GetContainerStatusColor returns the color for a container status
func GetContainerStatusColor(status string) string {
	switch status {
	case "running":
		return ColorRunning
	case "paused":
		return ColorPaused
	case "restarting":
		return ColorRestarting
	case "exited":
		return ColorExited
	default:
		return ColorDefault
	}
}
