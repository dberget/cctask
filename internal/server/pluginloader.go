package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidberget/cctask-go/internal/store"
)

const (
	pluginDir      = "plugins"
	pluginBinDir   = "bin"
	pluginBinName  = "plugin"
	pluginTimeout  = 10 * time.Second
)

// pluginsPath returns the path to .cctask/plugins/.
func pluginsPath(projectRoot string) string {
	return filepath.Join(store.CctaskDir(projectRoot), pluginDir)
}

// LoadPlugins discovers, compiles, and registers all plugins.
func (s *Server) LoadPlugins() error {
	dir := pluginsPath(s.projectRoot)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no plugins directory is fine
		}
		return fmt.Errorf("read plugins dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginPath := filepath.Join(dir, entry.Name())
		mainGo := filepath.Join(pluginPath, "main.go")
		if _, err := os.Stat(mainGo); err != nil {
			continue // skip directories without main.go
		}

		info, err := s.loadPlugin(pluginPath)
		if err != nil {
			log.Printf("plugin %s: skip: %v", entry.Name(), err)
			continue
		}

		s.plugins = append(s.plugins, *info)
		for _, route := range info.Routes {
			s.registerPluginHandler(*info, route)
		}
		log.Printf("plugin %s: loaded (%d routes)", info.Name, len(info.Routes))
	}
	return nil
}

func (s *Server) loadPlugin(pluginPath string) (*PluginInfo, error) {
	binDir := filepath.Join(pluginPath, pluginBinDir)
	binPath := filepath.Join(binDir, pluginBinName)

	// Compile if needed (binary missing or older than source)
	if needsCompile(pluginPath, binPath) {
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			return nil, fmt.Errorf("create bin dir: %w", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
		cmd.Dir = pluginPath
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("compile: %s: %w", strings.TrimSpace(string(out)), err)
		}
	}

	// Get plugin info
	ctx, cancel := context.WithTimeout(context.Background(), pluginTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, "info")
	cmd.Dir = pluginPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("info: %w", err)
	}

	var info PluginInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("parse info: %w", err)
	}
	info.BinaryPath = binPath
	return &info, nil
}

func needsCompile(pluginPath, binPath string) bool {
	binStat, err := os.Stat(binPath)
	if err != nil {
		return true // binary doesn't exist
	}

	// Check if any .go file is newer than the binary
	entries, err := os.ReadDir(pluginPath)
	if err != nil {
		return true
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(binStat.ModTime()) {
				return true
			}
		}
	}
	return false
}

func (s *Server) registerPluginHandler(info PluginInfo, route PluginRoute) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
		if err != nil {
			http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
			return
		}

		headers := make(map[string]string)
		for k := range r.Header {
			headers[k] = r.Header.Get(k)
		}

		pluginReq := PluginRequest{
			Body:    string(body),
			Headers: headers,
		}
		reqJSON, _ := json.Marshal(pluginReq)

		ctx, cancel := context.WithTimeout(r.Context(), pluginTimeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, info.BinaryPath, "handle", "--route", route.Path)
		cmd.Stdin = bytes.NewReader(reqJSON)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			log.Printf("plugin %s: handle error: %v: %s", info.Name, err, stderr.String())
			http.Error(w, `{"error":"plugin execution failed"}`, http.StatusInternalServerError)
			return
		}

		var resp PluginResponse
		if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
			log.Printf("plugin %s: parse response: %v", info.Name, err)
			http.Error(w, `{"error":"plugin returned invalid response"}`, http.StatusInternalServerError)
			return
		}

		// Create tasks from plugin response
		var created []string
		for _, pt := range resp.Tasks {
			task, err := store.AddTask(s.projectRoot, pt.Title, pt.Description, pt.Tags, pt.Group)
			if err != nil {
				log.Printf("plugin %s: create task: %v", info.Name, err)
				continue
			}
			created = append(created, task.ID)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"created": len(created),
			"taskIds": created,
		})
	}

	s.RegisterPluginRoute(route.Method, route.Path, handler)
}
