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
	IconCursor      = ">"
	IconNoCursor    = " "
	IconInputCursor = "█"
)

// Colors
const (
	ColorRunning    = "#4ec9b0" // Green
	ColorPaused     = "#dcdcaa" // Yellow
	ColorRestarting = "#4ec9b0" // Green
	ColorExited     = "#858585" // Gray
	ColorDefault    = "#ce9178" // Orange

	ColorVolume  = "#dcdcaa" // Yellow
	ColorImage   = "#569cd6" // Blue
	ColorNetwork = "#c586c0" // Purple

	ColorPrimary    = "#007acc" // Blue
	ColorSecondary  = "#3e3e3e" // Dark gray
	ColorBorder     = "#3e3e3e" // Dark gray
	ColorBackground = "#1e1e1e" // Very dark gray
	ColorForeground = "#d4d4d4" // Light gray
	ColorMuted      = "#858585" // Gray
	ColorSuccess    = "#4ec9b0" // Green
	ColorWarning    = "#dcdcaa" // Yellow
	ColorError      = "#f48771" // Red
	ColorInfo       = "#569cd6" // Blue
	ColorLabel      = "#608b4e" // Olive green
	ColorTitle      = "#0e639c" // Title blue
	ColorHighlight  = "#094771" // Selection/highlight background
	ColorLink       = "#ce9178" // Link orange

	ColorSearchHighlightFG = "#000000" // Black (search highlight foreground)
	ColorSearchHighlightBG = "#ffff00" // Yellow (search highlight background)
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
