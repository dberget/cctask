package tui

import "github.com/charmbracelet/lipgloss"

var (
	// ── Color Palette ──────────────────────────────────
	colorPrimary   = lipgloss.Color("#818CF8") // indigo-400
	colorSecondary = lipgloss.Color("#9CA3AF") // gray-400
	colorAccent    = lipgloss.Color("#FBBF24") // amber-400
	colorSuccess   = lipgloss.Color("#34D399") // emerald-400
	colorError     = lipgloss.Color("#F87171") // red-400
	colorMagenta   = lipgloss.Color("#C084FC") // purple-400
	colorWhite     = lipgloss.Color("#E5E7EB") // gray-200
	colorBright    = lipgloss.Color("#F9FAFB") // gray-50
	colorDim       = lipgloss.Color("#4B5563") // gray-600
	colorBorder    = lipgloss.Color("#374151") // gray-700

	// ── Base Styles ────────────────────────────────────
	styleBold = lipgloss.NewStyle().Bold(true)

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBright)

	styleCyan = lipgloss.NewStyle().
			Foreground(colorPrimary)

	styleCyanBold = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	styleGray = lipgloss.NewStyle().
			Foreground(colorSecondary)

	styleDim = lipgloss.NewStyle().
			Foreground(colorDim)

	styleYellow = lipgloss.NewStyle().
			Foreground(colorAccent)

	styleGreen = lipgloss.NewStyle().
			Foreground(colorSuccess)

	styleRed = lipgloss.NewStyle().
			Foreground(colorError)

	styleMagenta = lipgloss.NewStyle().
			Foreground(colorMagenta)

	styleSelected = lipgloss.NewStyle().
			Foreground(colorPrimary)

	styleCursor = lipgloss.NewStyle().
			Background(colorBright).
			Foreground(lipgloss.Color("#111827"))

	styleBorder = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder)
)

func statusIcon(status string) string {
	switch status {
	case "pending":
		return styleYellow.Render("●")
	case "in-progress":
		return styleCyan.Render("◉")
	case "done":
		return styleGreen.Render("✓")
	default:
		return styleDim.Render("○")
	}
}

func statusLabel(status string) string {
	switch status {
	case "pending":
		return styleYellow.Render("pending")
	case "in-progress":
		return styleCyan.Render("in-progress")
	case "done":
		return styleGreen.Render("done")
	default:
		return status
	}
}

func processStatusSymbol(status string) string {
	switch status {
	case "running":
		return styleYellow.Render("◉")
	case "done":
		return styleGreen.Render("✓")
	case "error":
		return styleRed.Render("✗")
	default:
		return styleDim.Render("○")
	}
}

func processStatusColor(status string) lipgloss.Color {
	switch status {
	case "running":
		return colorAccent
	case "done":
		return colorSuccess
	case "error":
		return colorError
	default:
		return colorSecondary
	}
}

func planStatus(hasPlan bool) string {
	if hasPlan {
		return styleGreen.Render("✓ saved")
	}
	return styleDim.Render("none")
}

// ApplyTheme sets all color and style variables from a named theme preset.
func ApplyTheme(name string) {
	tc, ok := Themes[name]
	if !ok {
		tc = Themes["default"]
	}

	colorPrimary = lipgloss.Color(tc.Primary)
	colorSecondary = lipgloss.Color(tc.Secondary)
	colorAccent = lipgloss.Color(tc.Accent)
	colorSuccess = lipgloss.Color(tc.Success)
	colorError = lipgloss.Color(tc.Error)
	colorMagenta = lipgloss.Color(tc.Magenta)
	colorWhite = lipgloss.Color(tc.White)
	colorBright = lipgloss.Color(tc.Bright)
	colorDim = lipgloss.Color(tc.Dim)
	colorBorder = lipgloss.Color(tc.Border)

	// Rebuild derived styles
	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(colorBright)
	styleCyan = lipgloss.NewStyle().Foreground(colorPrimary)
	styleCyanBold = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	styleGray = lipgloss.NewStyle().Foreground(colorSecondary)
	styleDim = lipgloss.NewStyle().Foreground(colorDim)
	styleYellow = lipgloss.NewStyle().Foreground(colorAccent)
	styleGreen = lipgloss.NewStyle().Foreground(colorSuccess)
	styleRed = lipgloss.NewStyle().Foreground(colorError)
	styleMagenta = lipgloss.NewStyle().Foreground(colorMagenta)
	styleSelected = lipgloss.NewStyle().Foreground(colorPrimary)
	styleCursor = lipgloss.NewStyle().Background(colorBright).Foreground(lipgloss.Color("#111827"))
	styleBorder = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(colorBorder)
}
