package skill

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skill represents a custom skill definition loaded from a SKILL.md file.
type Skill struct {
	Name         string
	Description  string
	SystemPrompt string // markdown body after frontmatter
	FilePath     string
}

// LoadSkills scans for SKILL.md files in three locations (highest priority first):
// 1. <projectRoot>/.claude/skills/*/SKILL.md (project-level)
// 2. ~/.claude/skills/*/SKILL.md (user-level)
// 3. ~/.claude/plugins/cache/*/VERSION/skills/*/SKILL.md (enabled plugins only)
// De-duplicates by name (project > user > plugin). Sorted alphabetically.
func LoadSkills(projectRoot string) []Skill {
	seen := map[string]bool{}
	var skills []Skill

	// Project-level first (highest priority)
	projectDir := filepath.Join(projectRoot, ".claude", "skills")
	for _, s := range scanSkillDir(projectDir) {
		seen[s.Name] = true
		skills = append(skills, s)
	}

	home, err := os.UserHomeDir()
	if err == nil {
		// User-level
		userDir := filepath.Join(home, ".claude", "skills")
		for _, s := range scanSkillDir(userDir) {
			if !seen[s.Name] {
				seen[s.Name] = true
				skills = append(skills, s)
			}
		}

		// Plugin-level (enabled plugins only)
		enabled := loadEnabledPlugins(home)
		pluginCacheDir := filepath.Join(home, ".claude", "plugins", "cache")
		pluginDirs, err := os.ReadDir(pluginCacheDir)
		if err == nil {
			for _, pd := range pluginDirs {
				if !pd.IsDir() || !enabled[pd.Name()] {
					continue
				}
				// Scan version subdirectories
				versionDir := filepath.Join(pluginCacheDir, pd.Name())
				versionEntries, err := os.ReadDir(versionDir)
				if err != nil {
					continue
				}
				for _, ve := range versionEntries {
					if !ve.IsDir() {
						continue
					}
					skillsDir := filepath.Join(versionDir, ve.Name(), "skills")
					for _, s := range scanSkillDir(skillsDir) {
						if !seen[s.Name] {
							seen[s.Name] = true
							skills = append(skills, s)
						}
					}
				}
			}
		}
	}

	sort.Slice(skills, func(i, j int) bool {
		return skills[i].Name < skills[j].Name
	})
	return skills
}

// scanSkillDir scans a directory for subdirectories containing SKILL.md files.
// Follows symlinks when reading directories.
func scanSkillDir(dir string) []Skill {
	entries, err := readDirFollowSymlinks(dir)
	if err != nil {
		return nil
	}
	var skills []Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name(), "SKILL.md")
		s, err := ParseSkillFile(path)
		if err != nil {
			continue
		}
		skills = append(skills, *s)
	}
	return skills
}

// readDirFollowSymlinks reads a directory, resolving symlinked directories
// so their entries appear as directories.
func readDirFollowSymlinks(dir string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var result []os.DirEntry
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 {
			// Resolve symlink to check if it points to a directory
			target := filepath.Join(dir, e.Name())
			info, err := os.Stat(target) // Stat follows symlinks
			if err != nil {
				continue
			}
			if info.IsDir() {
				result = append(result, symlinkDirEntry{entry: e, info: info})
			}
		} else {
			result = append(result, e)
		}
	}
	return result, nil
}

// symlinkDirEntry wraps a DirEntry to report a symlink target as a directory.
type symlinkDirEntry struct {
	entry os.DirEntry
	info  os.FileInfo
}

func (s symlinkDirEntry) Name() string               { return s.entry.Name() }
func (s symlinkDirEntry) IsDir() bool                 { return s.info.IsDir() }
func (s symlinkDirEntry) Type() os.FileMode           { return s.info.Mode().Type() }
func (s symlinkDirEntry) Info() (os.FileInfo, error)   { return s.info, nil }

// ParseSkillFile reads a SKILL.md file with optional YAML frontmatter delimited by ---.
// The name defaults to the parent directory name if not set in frontmatter.
func ParseSkillFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	s := &Skill{
		FilePath: path,
	}

	// Default name from parent directory name
	s.Name = filepath.Base(filepath.Dir(path))

	// Parse frontmatter
	if strings.HasPrefix(strings.TrimSpace(content), "---") {
		trimmed := strings.TrimSpace(content)
		// Find closing ---
		rest := trimmed[3:] // skip opening ---
		rest = strings.TrimLeft(rest, " \t")
		if len(rest) > 0 && rest[0] == '\n' {
			rest = rest[1:]
		} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
			rest = rest[2:]
		}

		idx := strings.Index(rest, "\n---")
		if idx >= 0 {
			frontmatter := rest[:idx]
			body := rest[idx+4:] // skip \n---
			body = strings.TrimLeft(body, "\r\n")

			parseFrontmatter(s, frontmatter)
			s.SystemPrompt = strings.TrimSpace(body)
		} else {
			// No closing ---, treat entire content as system prompt
			s.SystemPrompt = strings.TrimSpace(content)
		}
	} else {
		s.SystemPrompt = strings.TrimSpace(content)
	}

	return s, nil
}

func parseFrontmatter(s *Skill, fm string) {
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
			s.Name = val
		case "description":
			s.Description = val
		}
	}
}

// loadEnabledPlugins reads ~/.claude/settings.json and returns a map of
// plugin names that are explicitly enabled (value is true).
func loadEnabledPlugins(home string) map[string]bool {
	enabled := map[string]bool{}
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return enabled
	}

	var settings struct {
		EnabledPlugins map[string]bool `json:"enabledPlugins"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return enabled
	}

	for name, on := range settings.EnabledPlugins {
		if on {
			enabled[name] = true
		}
	}
	return enabled
}
