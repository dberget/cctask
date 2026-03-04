package server

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidberget/cctask-go/internal/store"
)

//go:embed scaffold/*.tmpl
var scaffoldFS embed.FS

// ScaffoldPlugin creates a new plugin directory from the scaffold templates.
// Returns the path to the created plugin directory.
func ScaffoldPlugin(projectRoot, name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return "", fmt.Errorf("plugin name is required")
	}

	pluginDir := filepath.Join(store.CctaskDir(projectRoot), "plugins", name)
	if _, err := os.Stat(pluginDir); err == nil {
		return "", fmt.Errorf("plugin %q already exists", name)
	}

	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return "", fmt.Errorf("create plugin dir: %w", err)
	}

	// Read and render main.go template
	mainTmpl, err := scaffoldFS.ReadFile("scaffold/main.go.tmpl")
	if err != nil {
		return "", fmt.Errorf("read main.go template: %w", err)
	}
	mainContent := strings.ReplaceAll(string(mainTmpl), "{{.Name}}", name)
	mainContent = strings.ReplaceAll(mainContent, "{{.Description}}", name+" webhook integration")
	if err := os.WriteFile(filepath.Join(pluginDir, "main.go"), []byte(mainContent), 0o644); err != nil {
		return "", fmt.Errorf("write main.go: %w", err)
	}

	// Read and render go.mod template
	modTmpl, err := scaffoldFS.ReadFile("scaffold/go.mod.tmpl")
	if err != nil {
		return "", fmt.Errorf("read go.mod template: %w", err)
	}
	modContent := strings.ReplaceAll(string(modTmpl), "{{.Name}}", name)
	if err := os.WriteFile(filepath.Join(pluginDir, "go.mod"), []byte(modContent), 0o644); err != nil {
		return "", fmt.Errorf("write go.mod: %w", err)
	}

	return pluginDir, nil
}
