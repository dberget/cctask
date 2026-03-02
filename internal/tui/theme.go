package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary   = lipgloss.Color("6")  // cyan
	colorSecondary = lipgloss.Color("8")  // gray
	colorAccent    = lipgloss.Color("3")  // yellow
	colorSuccess   = lipgloss.Color("2")  // green
	colorError     = lipgloss.Color("1")  // red
	colorMagenta   = lipgloss.Color("5")  // magenta
	colorWhite     = lipgloss.Color("15") // white

	styleBold = lipgloss.NewStyle().Bold(true)

	styleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite)

	styleCyan = lipgloss.NewStyle().
			Foreground(colorPrimary)

	styleCyanBold = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	styleGray = lipgloss.NewStyle().
			Foreground(colorSecondary)

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
			Background(lipgloss.Color("15")).
			Foreground(lipgloss.Color("0"))

	styleBorder = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorSecondary)
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
		return "○"
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
		return "○"
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
	return styleGray.Render("none")
}
