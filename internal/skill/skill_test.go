package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSkillFile_WithFrontmatter(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `---
name: custom-name
description: A test skill
---
You are a helpful assistant that does testing.

Use careful analysis.`

	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := ParseSkillFile(path)
	if err != nil {
		t.Fatalf("ParseSkillFile returned error: %v", err)
	}

	if s.Name != "custom-name" {
		t.Errorf("Name = %q, want %q", s.Name, "custom-name")
	}
	if s.Description != "A test skill" {
		t.Errorf("Description = %q, want %q", s.Description, "A test skill")
	}
	if s.SystemPrompt != "You are a helpful assistant that does testing.\n\nUse careful analysis." {
		t.Errorf("SystemPrompt = %q, want multiline body", s.SystemPrompt)
	}
	if s.FilePath != path {
		t.Errorf("FilePath = %q, want %q", s.FilePath, path)
	}
}

func TestParseSkillFile_WithoutFrontmatter(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "plain-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `You are a plain skill with no frontmatter.

Just markdown content.`

	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := ParseSkillFile(path)
	if err != nil {
		t.Fatalf("ParseSkillFile returned error: %v", err)
	}

	// Name defaults to parent directory name
	if s.Name != "plain-skill" {
		t.Errorf("Name = %q, want %q", s.Name, "plain-skill")
	}
	if s.Description != "" {
		t.Errorf("Description = %q, want empty", s.Description)
	}
	if s.SystemPrompt != "You are a plain skill with no frontmatter.\n\nJust markdown content." {
		t.Errorf("SystemPrompt = %q, want full content", s.SystemPrompt)
	}
}

func TestParseSkillFile_NameDefaultsToParentDir(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "dir-name-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Frontmatter without a name field
	content := `---
description: Only description set
---
Body content here.`

	path := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := ParseSkillFile(path)
	if err != nil {
		t.Fatalf("ParseSkillFile returned error: %v", err)
	}

	// Name should come from parent directory since not in frontmatter
	if s.Name != "dir-name-skill" {
		t.Errorf("Name = %q, want %q", s.Name, "dir-name-skill")
	}
	if s.Description != "Only description set" {
		t.Errorf("Description = %q, want %q", s.Description, "Only description set")
	}
}

func TestLoadSkills_ProjectLevel(t *testing.T) {
	root := t.TempDir()

	// Create two project-level skills
	for _, name := range []string{"beta-skill", "alpha-skill"} {
		skillDir := filepath.Join(root, ".claude", "skills", name)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatal(err)
		}
		content := "---\nname: " + name + "\ndescription: " + name + " desc\n---\nPrompt for " + name
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	skills := LoadSkills(root)

	if len(skills) < 2 {
		t.Fatalf("got %d skills, want at least 2", len(skills))
	}

	// Verify alphabetical sort
	foundAlpha := -1
	foundBeta := -1
	for i, s := range skills {
		if s.Name == "alpha-skill" {
			foundAlpha = i
		}
		if s.Name == "beta-skill" {
			foundBeta = i
		}
	}
	if foundAlpha == -1 || foundBeta == -1 {
		t.Fatal("did not find both alpha-skill and beta-skill")
	}
	if foundAlpha >= foundBeta {
		t.Errorf("alpha-skill at index %d should come before beta-skill at index %d", foundAlpha, foundBeta)
	}
}

func TestLoadSkills_Deduplication(t *testing.T) {
	root := t.TempDir()

	// Create a project-level skill
	projectSkillDir := filepath.Join(root, ".claude", "skills", "shared-skill")
	if err := os.MkdirAll(projectSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(projectSkillDir, "SKILL.md"),
		[]byte("---\nname: shared-skill\n---\nProject version"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	// LoadSkills will also scan user home; the project-level should win
	skills := LoadSkills(root)

	count := 0
	for _, s := range skills {
		if s.Name == "shared-skill" {
			count++
			if s.SystemPrompt != "Project version" {
				t.Errorf("expected project-level prompt, got %q", s.SystemPrompt)
			}
		}
	}
	if count != 1 {
		t.Errorf("expected 1 shared-skill, got %d", count)
	}
}

func TestLoadSkills_FollowsSymlinks(t *testing.T) {
	root := t.TempDir()

	// Create actual skill directory outside the standard location
	realDir := filepath.Join(root, "real-skills", "linked-skill")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(realDir, "SKILL.md"),
		[]byte("---\nname: linked-skill\n---\nFollowed symlink"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	// Create the skills dir and a symlink inside it
	skillsDir := filepath.Join(root, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realDir, filepath.Join(skillsDir, "linked-skill")); err != nil {
		t.Fatal(err)
	}

	skills := LoadSkills(root)

	found := false
	for _, s := range skills {
		if s.Name == "linked-skill" {
			found = true
			if s.SystemPrompt != "Followed symlink" {
				t.Errorf("SystemPrompt = %q, want %q", s.SystemPrompt, "Followed symlink")
			}
		}
	}
	if !found {
		t.Error("did not find linked-skill via symlink")
	}
}

func TestLoadEnabledPlugins(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	settings := `{"enabledPlugins": {"good-plugin": true, "bad-plugin": false, "another-good": true}}`
	if err := os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(settings), 0o644); err != nil {
		t.Fatal(err)
	}

	enabled := loadEnabledPlugins(home)

	if !enabled["good-plugin"] {
		t.Error("good-plugin should be enabled")
	}
	if !enabled["another-good"] {
		t.Error("another-good should be enabled")
	}
	if enabled["bad-plugin"] {
		t.Error("bad-plugin should not be enabled")
	}
	if enabled["nonexistent"] {
		t.Error("nonexistent should not be enabled")
	}
}

func TestLoadEnabledPlugins_MissingFile(t *testing.T) {
	home := t.TempDir()
	enabled := loadEnabledPlugins(home)
	if len(enabled) != 0 {
		t.Errorf("expected empty map, got %v", enabled)
	}
}
