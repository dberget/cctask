package tui

import (
	"github.com/davidberget/cctask-go/internal/store"
)

func renderContextView(projectRoot string) string {
	content := store.LoadContext(projectRoot)

	var lines string
	lines = styleCyanBold.Render("Project Context") + "\n"
	lines += styleGray.Render(".cctask/context.md") + "\n\n"

	if content == "" {
		lines += styleGray.Render("No context defined yet.") + "\n\n"
		lines += "Press " + styleCyanBold.Render("e") + " to add global context about your project.\n"
		lines += styleGray.Render("This context is prepended to all prompts sent to Claude.")
	} else {
		lines += content
	}

	return lines
}
