package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/davidberget/cctask-go/internal/server"
)

// pluginViewInfo holds plugin display data for the TUI view.
type pluginViewInfo struct {
	Name        string
	Description string
	Routes      []server.PluginRoute
	Dir         string
	Compiled    bool
	Error       string // non-empty if info retrieval failed
}

// discoverPlugins scans .cctask/plugins/ and returns display info for each plugin.
func discoverPlugins(projectRoot string) []pluginViewInfo {
	dir := server.PluginsPath(projectRoot)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var plugins []pluginViewInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginPath := filepath.Join(dir, entry.Name())
		mainGo := filepath.Join(pluginPath, "main.go")
		if _, err := os.Stat(mainGo); err != nil {
			continue
		}

		info := pluginViewInfo{
			Name: entry.Name(),
			Dir:  pluginPath,
		}

		binPath := filepath.Join(pluginPath, "bin", "plugin")
		if _, err := os.Stat(binPath); err == nil {
			info.Compiled = true
			cmd := exec.Command(binPath, "info")
			cmd.Dir = pluginPath
			out, err := cmd.Output()
			if err == nil {
				var pi server.PluginInfo
				if json.Unmarshal(out, &pi) == nil {
					info.Name = pi.Name
					info.Description = pi.Description
					info.Routes = pi.Routes
				}
			} else {
				info.Error = "failed to get plugin info"
			}
		}

		plugins = append(plugins, info)
	}
	return plugins
}

// renderPluginList renders the fullscreen plugin browser view.
func renderPluginList(projectRoot string, width int) string {
	plugins := discoverPlugins(projectRoot)

	var b strings.Builder
	b.WriteString(styleCyanBold.Render("Installed Plugins"))
	b.WriteString("\n")
	b.WriteString(horizontalLine(width))
	b.WriteString("\n\n")

	if len(plugins) == 0 {
		b.WriteString(styleGray.Render("No plugins installed. Use :plugin new <name> to create one."))
		b.WriteString("\n")
		return b.String()
	}

	for i, p := range plugins {
		// Name
		b.WriteString(styleCyanBold.Render(p.Name))
		if !p.Compiled {
			b.WriteString("  " + styleYellow.Render("(not compiled)"))
		}
		b.WriteString("\n")

		// Description
		if p.Description != "" {
			b.WriteString("  " + p.Description)
			b.WriteString("\n")
		}

		// Routes
		if len(p.Routes) > 0 {
			b.WriteString("  " + styleDim.Render("Routes:"))
			b.WriteString("\n")
			for _, r := range p.Routes {
				b.WriteString(fmt.Sprintf("    %s %s", styleCyanBold.Render(r.Method), r.Path))
				b.WriteString("\n")
			}
		}

		// Directory
		b.WriteString("  " + styleDim.Render("Dir: "+p.Dir))
		b.WriteString("\n")

		// Error
		if p.Error != "" {
			b.WriteString("  " + styleYellow.Render(p.Error))
			b.WriteString("\n")
		}

		if i < len(plugins)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}
