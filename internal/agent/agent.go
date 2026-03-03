package agent

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Agent represents a custom agent definition loaded from a .md file.
type Agent struct {
	Name         string
	Description  string
	Model        string // "inherit", "sonnet", "opus", "haiku", or empty (inherit)
	SystemPrompt string // markdown body after frontmatter
	FilePath     string
}

// LoadAgents scans .claude/agents/*.md at project level then user level.
// Project-level agents take precedence over user-level agents with the same name.
func LoadAgents(projectRoot string) []Agent {
	seen := map[string]bool{}
	var agents []Agent

	// Project-level first (higher priority)
	projectDir := filepath.Join(projectRoot, ".claude", "agents")
	for _, a := range scanDir(projectDir) {
		seen[a.Name] = true
		agents = append(agents, a)
	}

	// User-level
	home, err := os.UserHomeDir()
	if err == nil {
		userDir := filepath.Join(home, ".claude", "agents")
		for _, a := range scanDir(userDir) {
			if !seen[a.Name] {
				agents = append(agents, a)
			}
		}
	}

	sort.Slice(agents, func(i, j int) bool {
		return agents[i].Name < agents[j].Name
	})
	return agents
}

func scanDir(dir string) []Agent {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var agents []Agent
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		a, err := ParseAgentFile(path)
		if err != nil {
			continue
		}
		agents = append(agents, *a)
	}
	return agents
}

// ParseAgentFile reads a .md file with optional YAML frontmatter delimited by ---.
func ParseAgentFile(path string) (*Agent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	a := &Agent{
		FilePath: path,
	}

	// Default name from filename
	base := filepath.Base(path)
	a.Name = strings.TrimSuffix(base, ".md")

	// Parse frontmatter
	if strings.HasPrefix(strings.TrimSpace(content), "---") {
		trimmed := strings.TrimSpace(content)
		// Find closing ---
		rest := trimmed[3:] // skip opening ---
		rest = strings.TrimLeft(rest, " \t")
		if rest[0] == '\n' {
			rest = rest[1:]
		} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
			rest = rest[2:]
		}

		idx := strings.Index(rest, "\n---")
		if idx >= 0 {
			frontmatter := rest[:idx]
			body := rest[idx+4:] // skip \n---
			body = strings.TrimLeft(body, "\r\n")

			parseFrontmatter(a, frontmatter)
			a.SystemPrompt = strings.TrimSpace(body)
		} else {
			// No closing ---, treat entire content as system prompt
			a.SystemPrompt = strings.TrimSpace(content)
		}
	} else {
		a.SystemPrompt = strings.TrimSpace(content)
	}

	return a, nil
}

func parseFrontmatter(a *Agent, fm string) {
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "name":
			a.Name = val
		case "description":
			a.Description = val
		case "model":
			a.Model = val
		}
	}
}
