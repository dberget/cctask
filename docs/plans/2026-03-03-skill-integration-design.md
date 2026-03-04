# Skill Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Claude skill discovery, task-level skill selection on the form, config toggle, and slash-command autocomplete in description fields.

**Architecture:** New `internal/skill/` package mirrors `internal/agent/` for loading SKILL.md files. Skills are stored on Task structs and appended to system prompts at run time. The task form gets a 5th field (Skills) that opens a MultiCheckModel picker. Description field gets inline autocomplete when `/` is typed.

**Tech Stack:** Go, Bubble Tea, Lipgloss, YAML frontmatter parsing (reuse agent pattern)

---

### Task 1: Create `internal/skill/` package — Skill struct and loader

**Files:**
- Create: `internal/skill/skill.go`
- Create: `internal/skill/skill_test.go`

**Step 1: Write the failing test**

```go
package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSkillFile(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	os.MkdirAll(skillDir, 0o755)
	content := "---\nname: my-skill\ndescription: Does cool stuff\n---\n\n## Instructions\n\nDo the thing."
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644)

	s, err := ParseSkillFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "my-skill" {
		t.Errorf("name = %q, want %q", s.Name, "my-skill")
	}
	if s.Description != "Does cool stuff" {
		t.Errorf("description = %q, want %q", s.Description, "Does cool stuff")
	}
	if s.SystemPrompt != "## Instructions\n\nDo the thing." {
		t.Errorf("systemPrompt = %q", s.SystemPrompt)
	}
}

func TestParseSkillFile_NoFrontmatter(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "raw-skill")
	os.MkdirAll(skillDir, 0o755)
	content := "Just raw instructions here."
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644)

	s, err := ParseSkillFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "raw-skill" {
		t.Errorf("name = %q, want %q", s.Name, "raw-skill")
	}
	if s.SystemPrompt != "Just raw instructions here." {
		t.Errorf("systemPrompt = %q", s.SystemPrompt)
	}
}

func TestLoadSkills(t *testing.T) {
	// Create a temp project root with .claude/skills/
	root := t.TempDir()
	skillDir := filepath.Join(root, ".claude", "skills", "test-skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: test-skill\ndescription: A test\n---\nPrompt body"), 0o644)

	skills := LoadSkills(root)
	if len(skills) == 0 {
		t.Fatal("expected at least 1 skill")
	}
	found := false
	for _, s := range skills {
		if s.Name == "test-skill" {
			found = true
		}
	}
	if !found {
		t.Error("test-skill not found in loaded skills")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/davidberget/github/cctask-go && go test ./internal/skill/ -v`
Expected: FAIL — package does not exist

**Step 3: Write minimal implementation**

```go
package skill

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Skill represents a Claude skill loaded from a SKILL.md file.
type Skill struct {
	Name         string
	Description  string
	SystemPrompt string
	FilePath     string
}

// LoadSkills scans for SKILL.md files in project, user, and plugin directories.
// Project-level skills take precedence over user-level, which take precedence over plugins.
func LoadSkills(projectRoot string) []Skill {
	seen := map[string]bool{}
	var skills []Skill

	// 1. Project-level: <projectRoot>/.claude/skills/*/SKILL.md
	projectDir := filepath.Join(projectRoot, ".claude", "skills")
	for _, s := range scanSkillDir(projectDir) {
		seen[s.Name] = true
		skills = append(skills, s)
	}

	// 2. User-level: ~/.claude/skills/*/SKILL.md
	home, err := os.UserHomeDir()
	if err == nil {
		userDir := filepath.Join(home, ".claude", "skills")
		for _, s := range scanSkillDir(userDir) {
			if !seen[s.Name] {
				seen[s.Name] = true
				skills = append(skills, s)
			}
		}
	}

	// 3. Plugin skills: ~/.claude/plugins/cache/*/skills/*/SKILL.md (enabled only)
	if err == nil {
		enabled := loadEnabledPlugins(home)
		cacheDir := filepath.Join(home, ".claude", "plugins", "cache")
		entries, _ := os.ReadDir(cacheDir)
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			// Plugin dirs are named like "plugin-name" or contain version subdirs
			pluginName := e.Name()
			if !enabled[pluginName] {
				continue
			}
			// Scan all version subdirectories for skills
			pluginPath := filepath.Join(cacheDir, pluginName)
			versionEntries, _ := os.ReadDir(pluginPath)
			for _, ve := range versionEntries {
				if !ve.IsDir() {
					continue
				}
				skillsDir := filepath.Join(pluginPath, ve.Name(), "skills")
				for _, s := range scanSkillDir(skillsDir) {
					if !seen[s.Name] {
						seen[s.Name] = true
						skills = append(skills, s)
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

func scanSkillDir(dir string) []Skill {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var skills []Skill
	for _, e := range entries {
		// Each entry should be a directory (or symlink to directory) containing SKILL.md
		path := filepath.Join(dir, e.Name())

		// Resolve symlinks
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			continue
		}

		skillFile := filepath.Join(path, "SKILL.md")
		s, err := ParseSkillFile(skillFile)
		if err != nil {
			continue
		}
		skills = append(skills, *s)
	}
	return skills
}

// ParseSkillFile reads a SKILL.md file with optional YAML frontmatter.
func ParseSkillFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	content := string(data)
	s := &Skill{
		FilePath: path,
	}

	// Default name from parent directory
	s.Name = filepath.Base(filepath.Dir(path))

	// Parse frontmatter
	if strings.HasPrefix(strings.TrimSpace(content), "---") {
		trimmed := strings.TrimSpace(content)
		rest := trimmed[3:]
		rest = strings.TrimLeft(rest, " \t")
		if len(rest) > 0 && rest[0] == '\n' {
			rest = rest[1:]
		} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
			rest = rest[2:]
		}

		idx := strings.Index(rest, "\n---")
		if idx >= 0 {
			frontmatter := rest[:idx]
			body := rest[idx+4:]
			body = strings.TrimLeft(body, "\r\n")

			parseSkillFrontmatter(s, frontmatter)
			s.SystemPrompt = strings.TrimSpace(body)
		} else {
			s.SystemPrompt = strings.TrimSpace(content)
		}
	} else {
		s.SystemPrompt = strings.TrimSpace(content)
	}

	return s, nil
}

func parseSkillFrontmatter(s *Skill, fm string) {
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

// enabledPlugins reads ~/.claude/settings.json to find enabled plugins.
type settingsJSON struct {
	EnabledPlugins map[string]bool `json:"enabledPlugins"`
}

func loadEnabledPlugins(home string) map[string]bool {
	fp := filepath.Join(home, ".claude", "settings.json")
	data, err := os.ReadFile(fp)
	if err != nil {
		return map[string]bool{}
	}
	var s settingsJSON
	if err := json.Unmarshal(data, &s); err != nil {
		return map[string]bool{}
	}
	if s.EnabledPlugins == nil {
		return map[string]bool{}
	}
	return s.EnabledPlugins
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/davidberget/github/cctask-go && go test ./internal/skill/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/skill/skill.go internal/skill/skill_test.go
git commit -m "feat: add skill discovery package (internal/skill)"
```

---

### Task 2: Add `Skills` field to Task model and store

**Files:**
- Modify: `internal/model/types.go:33-47` (Task struct)
- Modify: `internal/store/store.go:160-188` (AddTask function)
- Modify: `internal/store/store.go:190-240` (UpdateTask switch)

**Step 1: Add Skills field to Task struct**

In `internal/model/types.go`, add after `WorkDir`:

```go
Skills      []string   `json:"skills,omitempty"`
```

**Step 2: Add skills case to UpdateTask**

In `internal/store/store.go`, add a new case in the `UpdateTask` switch block (after the `"workDir"` case around line 231):

```go
case "skills":
	if val, ok := v.([]string); ok {
		task.Skills = val
	}
```

**Step 3: Run tests to verify nothing broke**

Run: `cd /Users/davidberget/github/cctask-go && go test ./...`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/model/types.go internal/store/store.go
git commit -m "feat: add Skills field to Task model and store"
```

---

### Task 3: Add `DisableSkillPicker` to Config

**Files:**
- Modify: `internal/model/types.go:95-100` (Config struct)

**Step 1: Add field to Config struct**

In `internal/model/types.go`, add to Config struct:

```go
DisableSkillPicker bool `json:"disableSkillPicker,omitempty"`
```

**Step 2: Run tests**

Run: `cd /Users/davidberget/github/cctask-go && go test ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/model/types.go
git commit -m "feat: add DisableSkillPicker config option"
```

---

### Task 4: Add Skills field to TaskFormData and FormModel

**Files:**
- Modify: `internal/tui/form.go` (FormModel, TaskFormData, NewForm, Update, View)

**Step 1: Update TaskFormData and form fields**

Add `Skills` to `TaskFormData`:

```go
type TaskFormData struct {
	Title       string
	Description string
	Tags        string
	WorkDir     string
	Skills      []string
}
```

Add `fieldSkills` to the enum and update `fieldCount`:

```go
const (
	fieldTitle formField = iota
	fieldDescription
	fieldTags
	fieldWorkDir
	fieldSkills
	fieldCount
)
```

Add fields to `FormModel`:

```go
type FormModel struct {
	Heading string
	Active  formField
	Width   int

	title   textinput.Model
	desc    textarea.Model
	tags    textinput.Model
	workDir textinput.Model

	// Skills picker
	skills          []string // selected skill names
	availableSkills []string // all skill names for display
	skillPickerOpen bool     // true when MultiCheck overlay is showing
}
```

**Step 2: Update NewForm to accept available skills**

Change `NewForm` signature to accept skills:

```go
func NewForm(heading string, initial *TaskFormData, width int, availableSkills []string) FormModel
```

In the body, set `m.availableSkills = availableSkills`. If `initial != nil`, set `m.skills = initial.Skills`.

**Step 3: Update Data() to return skills**

```go
func (m FormModel) Data() TaskFormData {
	return TaskFormData{
		Title:       m.title.Value(),
		Description: m.desc.Value(),
		Tags:        m.tags.Value(),
		WorkDir:     m.workDir.Value(),
		Skills:      m.skills,
	}
}
```

**Step 4: Update Update() for Skills field**

When `m.Active == fieldSkills` and Enter is pressed, return a `FormSkillPickerMsg{}` message (the app.go will handle opening the MultiCheck picker). When on the last field (now `fieldSkills`) and Enter is pressed, also trigger save if title is non-empty. Change the submit-on-last-field check from `fieldWorkDir` to `fieldSkills`.

In the Update method, change:
```go
if msg.Type == tea.KeyEnter && m.Active != fieldDescription {
	if m.Active == fieldWorkDir {
```
to:
```go
if msg.Type == tea.KeyEnter && m.Active != fieldDescription {
	if m.Active == fieldSkills {
		// Enter on skills field opens the skill picker
		return m, func() tea.Msg { return FormSkillPickerMsg{} }
	}
	if m.Active == fieldWorkDir {
```

Also update the Ctrl+S/Ctrl+D submit to continue using `m.Data()` (which now includes Skills).

**Step 5: Update View() for Skills field**

Add the Skills field rendering in the View loop. Update `labels`:

```go
labels := [fieldCount]string{"Title", "Description", "Tags", "WorkDir", "Skills"}
```

Add the case in the switch:

```go
case fieldSkills:
	skillsDisplay := "(none)"
	if len(m.skills) > 0 {
		skillsDisplay = strings.Join(m.skills, ", ")
	}
	lines = append(lines, labelPadded+lipgloss.NewStyle().Foreground(colorWhite).Render(skillsDisplay))
```

**Step 6: Add FormSkillPickerMsg type**

In form.go (or wherever form messages are defined):

```go
type FormSkillPickerMsg struct{}
```

**Step 7: Add method to update selected skills**

```go
func (m *FormModel) SetSkills(skills []string) {
	m.skills = skills
}
```

**Step 8: Run build**

Run: `cd /Users/davidberget/github/cctask-go && go build ./...`
Expected: Will have compile errors — NewForm call sites need updating. That's Task 5.

**Step 9: Commit (partial — form changes only)**

```bash
git add internal/tui/form.go
git commit -m "feat: add Skills field to task form"
```

---

### Task 5: Wire skills into app.go — loading, form, and picker

**Files:**
- Modify: `internal/tui/app.go` (Model struct, NewModel, form creation, handleFormSubmit, handleMultiCheckSubmit, skill picker)

**Step 1: Add skills to Model struct**

After the `agents` field (line 157), add:

```go
skills []skill.Skill
```

Add import for `"github.com/davidberget/cctask-go/internal/skill"`.

**Step 2: Load skills in NewModel**

After `agents: agent.LoadAgents(projectRoot)` (line 200), add:

```go
skills: skill.LoadSkills(projectRoot),
```

**Step 3: Update all NewForm call sites**

Find all places `NewForm(...)` is called and add the available skill names parameter. Create a helper:

```go
func (m Model) skillNames() []string {
	cfg := store.LoadConfig(m.projectRoot)
	if cfg.DisableSkillPicker {
		return nil
	}
	names := make([]string, len(m.skills))
	for i, s := range m.skills {
		names[i] = s.Name
	}
	return names
}
```

Then update each `NewForm(heading, initial, width)` call to `NewForm(heading, initial, width, m.skillNames())`.

**Step 4: Handle FormSkillPickerMsg**

Add a case in the main `Update` method for `FormSkillPickerMsg`:

```go
case FormSkillPickerMsg:
	// Open a MultiCheck with available skills
	items := make([]CheckItem, len(m.skills))
	for i, s := range m.skills {
		label := s.Name
		if s.Description != "" {
			label += "  " + s.Description
		}
		items[i] = CheckItem{Label: label, Value: s.Name}
	}
	m.multiCheck = NewMultiCheck("Select skills", items)
	// Pre-select already chosen skills
	for _, name := range m.form.skills {
		m.multiCheck.Selected[name] = true
	}
	m.returnMode = m.mode
	m.mode = model.ModeSkillPicker
	return m, nil
```

**Step 5: Add ModeSkillPicker to viewmode.go**

In `internal/model/viewmode.go`, add `ModeSkillPicker` to the ViewMode constants.

**Step 6: Handle ModeSkillPicker in key routing**

In the key dispatch area of `Update`, add handling for `ModeSkillPicker` that delegates to `m.multiCheck.Update(msg)`.

**Step 7: Handle MultiCheckSubmitMsg for skill picker**

When `m.mode == ModeSkillPicker` and a `MultiCheckSubmitMsg` arrives:

```go
case MultiCheckSubmitMsg:
	if m.mode == model.ModeSkillPicker {
		m.form.SetSkills(msg.Selected)
		m.mode = model.ModeTaskForm
		return m, nil
	}
```

Handle `MultiCheckCancelMsg` similarly — return to `ModeTaskForm`.

**Step 8: Update handleFormSubmit to pass skills**

In `handleFormSubmit`, after parsing tags, handle skills:

```go
case actionAddTask:
	t, _ := store.AddTask(m.projectRoot, data.Title, data.Description, tags, "", data.WorkDir)
	if t != nil && len(data.Skills) > 0 {
		store.UpdateTask(m.projectRoot, t.ID, map[string]interface{}{"skills": data.Skills})
		m.reload()
	}
```

For `actionEditTask`, add `"skills": data.Skills` to the update map.

**Step 9: Run build and fix any remaining compile errors**

Run: `cd /Users/davidberget/github/cctask-go && go build ./...`
Expected: PASS

**Step 10: Run tests**

Run: `cd /Users/davidberget/github/cctask-go && go test ./...`
Expected: PASS

**Step 11: Commit**

```bash
git add internal/tui/app.go internal/model/viewmode.go
git commit -m "feat: wire skill loading, form field, and picker into TUI"
```

---

### Task 6: Append selected skills to system prompt at run time

**Files:**
- Modify: `internal/tui/app.go` (spawnPlanGeneration, spawnBackgroundRun, executeRun)

**Step 1: Create a skillSDKOpts helper**

Add near `agentSDKOpts` (after line 1928):

```go
func skillSDKOpts(skills []skill.Skill, taskSkillNames []string) []claudecode.Option {
	if len(taskSkillNames) == 0 {
		return nil
	}
	// Build a map for lookup
	byName := make(map[string]*skill.Skill, len(skills))
	for i := range skills {
		byName[skills[i].Name] = &skills[i]
	}
	var prompts []string
	for _, name := range taskSkillNames {
		if s, ok := byName[name]; ok && s.SystemPrompt != "" {
			prompts = append(prompts, fmt.Sprintf("## Skill: %s\n\n%s", s.Name, s.SystemPrompt))
		}
	}
	if len(prompts) == 0 {
		return nil
	}
	combined := strings.Join(prompts, "\n\n---\n\n")
	return []claudecode.Option{claudecode.WithAppendSystemPrompt(combined)}
}
```

**Step 2: Pass skill opts in spawnPlanGeneration**

In `spawnPlanGeneration`, after the `agentSDKOpts(a)` call, append skill opts:

```go
extraOpts := agentSDKOpts(a)
if t := /* the task being planned */; t != nil {
	extraOpts = append(extraOpts, skillSDKOpts(m.skills, t.Skills)...)
}
```

**Step 3: Pass skill opts in spawnBackgroundRun**

In `spawnBackgroundRun` (around line 1832), after `extraOpts := agentSDKOpts(a)`:

```go
if isTask {
	if t := m.selectedTask(); t != nil {
		extraOpts = append(extraOpts, skillSDKOpts(m.skills, t.Skills)...)
	}
}
```

**Step 4: Pass skill opts in executeRun**

In `executeRun`, when building the systemPrompt, append skill content similarly to how agent prompts are appended:

```go
if t := m.selectedTask(); t != nil && len(t.Skills) > 0 {
	for _, name := range t.Skills {
		if s := m.skillByName(name); s != nil && s.SystemPrompt != "" {
			systemPrompt = systemPrompt + "\n\n---\n\n## Skill: " + s.Name + "\n\n" + s.SystemPrompt
		}
	}
}
```

Add helper:

```go
func (m Model) skillByName(name string) *skill.Skill {
	for i := range m.skills {
		if m.skills[i].Name == name {
			return &m.skills[i]
		}
	}
	return nil
}
```

**Step 5: Run build**

Run: `cd /Users/davidberget/github/cctask-go && go build ./...`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat: append selected skill prompts to Claude system prompt at run time"
```

---

### Task 7: Slash-command autocomplete in description field

**Files:**
- Modify: `internal/tui/form.go` (autocomplete state, Update, View)

**Step 1: Add autocomplete state to FormModel**

Add to FormModel struct:

```go
// Slash-command autocomplete
acActive bool     // autocomplete dropdown is showing
acQuery  string   // text typed after /
acIndex  int      // selected index in filtered list
acItems  []string // filtered skill names matching query
```

**Step 2: Add autocomplete detection in Update**

In the `fieldDescription` branch of the Update method, intercept key events:

When a rune `/` is typed and the character before cursor is a newline, space, or position 0 (start of line), activate autocomplete:

```go
case fieldDescription:
	if m.acActive {
		// Handle autocomplete keys
		switch {
		case msg.Type == tea.KeyEscape:
			m.acActive = false
			return m, nil
		case msg.Type == tea.KeyEnter:
			if len(m.acItems) > 0 {
				// Replace /query with selected skill name
				m.insertAutocomplete(m.acItems[m.acIndex])
			}
			m.acActive = false
			return m, nil
		case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && string(msg.Runes) == "k" && false):
			if m.acIndex > 0 {
				m.acIndex--
			}
			return m, nil
		case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && string(msg.Runes) == "j" && false):
			if m.acIndex < len(m.acItems)-1 {
				m.acIndex++
			}
			return m, nil
		case msg.Type == tea.KeyTab:
			if m.acIndex < len(m.acItems)-1 {
				m.acIndex++
			} else {
				m.acIndex = 0
			}
			return m, nil
		default:
			// Pass through to textarea, then update filter
			m.desc, cmd = m.desc.Update(msg)
			m.updateAutocompleteFilter()
			if len(m.acItems) == 0 {
				m.acActive = false
			}
			return m, cmd
		}
	}

	// Normal description handling — pass to textarea
	m.desc, cmd = m.desc.Update(msg)

	// Check if / was just typed at start of line or after whitespace
	if msg.Type == tea.KeyRunes && string(msg.Runes) == "/" {
		m.tryActivateAutocomplete()
	}
```

**Step 3: Add helper methods**

```go
func (m *FormModel) tryActivateAutocomplete() {
	if len(m.availableSkills) == 0 {
		return
	}
	// Check character before / — should be start of input, newline, or space
	val := m.desc.Value()
	// Find the / we just typed — it's at the cursor position minus 1
	// For simplicity, check if last non-/ char before cursor is whitespace or start
	lines := strings.Split(val, "\n")
	row := m.desc.Line()
	if row >= len(lines) {
		return
	}
	line := lines[row]
	col := m.desc.CursorPosition()
	if col <= 0 || col > len(line) {
		return
	}
	if line[col-1] != '/' {
		return
	}
	// Check character before /
	if col == 1 || line[col-2] == ' ' || line[col-2] == '\t' {
		m.acActive = true
		m.acQuery = ""
		m.acIndex = 0
		m.acItems = m.availableSkills // show all initially
	}
}

func (m *FormModel) updateAutocompleteFilter() {
	// Extract current query: find the last / on current line before cursor
	val := m.desc.Value()
	lines := strings.Split(val, "\n")
	row := m.desc.Line()
	if row >= len(lines) {
		m.acActive = false
		return
	}
	line := lines[row]
	col := m.desc.CursorPosition()
	if col > len(line) {
		col = len(line)
	}

	// Find the / that started this autocomplete
	slashPos := -1
	for i := col - 1; i >= 0; i-- {
		if line[i] == '/' {
			slashPos = i
			break
		}
		if line[i] == ' ' || line[i] == '\t' {
			break
		}
	}
	if slashPos == -1 {
		m.acActive = false
		return
	}

	m.acQuery = strings.ToLower(line[slashPos+1 : col])
	m.acItems = nil
	for _, name := range m.availableSkills {
		if strings.Contains(strings.ToLower(name), m.acQuery) {
			m.acItems = append(m.acItems, name)
		}
	}
	if m.acIndex >= len(m.acItems) {
		m.acIndex = 0
	}
}

func (m *FormModel) insertAutocomplete(skillName string) {
	// Replace /query with /skillName
	val := m.desc.Value()
	lines := strings.Split(val, "\n")
	row := m.desc.Line()
	if row >= len(lines) {
		return
	}
	line := lines[row]
	col := m.desc.CursorPosition()
	if col > len(line) {
		col = len(line)
	}

	// Find the / position
	slashPos := -1
	for i := col - 1; i >= 0; i-- {
		if line[i] == '/' {
			slashPos = i
			break
		}
	}
	if slashPos == -1 {
		return
	}

	// Rebuild the line
	newLine := line[:slashPos] + "/" + skillName + " "
	lines[row] = newLine
	m.desc.SetValue(strings.Join(lines, "\n"))
}
```

**Step 4: Render autocomplete dropdown in View()**

After the description field rendering in View(), if `m.acActive && m.Active == fieldDescription`, render a dropdown:

```go
if m.acActive && m.Active == fieldDescription {
	indent := strings.Repeat(" ", 16)
	acBox := []string{indent + styleGray.Render("Skills:")}
	maxShow := 8
	if len(m.acItems) < maxShow {
		maxShow = len(m.acItems)
	}
	for i := 0; i < maxShow; i++ {
		prefix := "  "
		style := lipgloss.NewStyle().Foreground(colorWhite)
		if i == m.acIndex {
			prefix = "▸ "
			style = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
		}
		acBox = append(acBox, indent+style.Render(prefix+"/"+m.acItems[i]))
	}
	if len(m.acItems) > maxShow {
		acBox = append(acBox, indent+styleGray.Render(fmt.Sprintf("  ... %d more", len(m.acItems)-maxShow)))
	}
	lines = append(lines, strings.Join(acBox, "\n"))
}
```

**Step 5: Handle Up/Down keys in autocomplete with proper key types**

Use `tea.KeyUp` and `tea.KeyDown` (not j/k which should still type normally in textarea):

The autocomplete intercepts: Escape, Enter, Tab, Up, Down. All other keys pass through to textarea and re-filter.

**Step 6: Run build**

Run: `cd /Users/davidberget/github/cctask-go && go build ./...`
Expected: PASS

**Step 7: Manual test**

Run: `cd /Users/davidberget/github/cctask-go && go install ./... && cp ~/go/bin/cctask-go /opt/homebrew/bin/cctask`
Then: `cctask` → press `a` to add task → Tab to Description → type `/` → verify dropdown appears.

**Step 8: Commit**

```bash
git add internal/tui/form.go
git commit -m "feat: add slash-command autocomplete for skills in description field"
```

---

### Task 8: Display skills on task detail panel

**Files:**
- Modify: `internal/tui/panel_detail.go` (render skills in task detail view)

**Step 1: Add skills display**

Find where task details are rendered (tags, workDir, etc.) and add a Skills section after Tags:

```go
if len(task.Skills) > 0 {
	lines = append(lines, sectionHeader("Skills"))
	lines = append(lines, "  "+strings.Join(task.Skills, ", "))
	lines = append(lines, "")
}
```

**Step 2: Run build**

Run: `cd /Users/davidberget/github/cctask-go && go build ./...`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/tui/panel_detail.go
git commit -m "feat: display task skills in detail panel"
```

---

### Task 9: Final integration test and cleanup

**Step 1: Run full test suite**

Run: `cd /Users/davidberget/github/cctask-go && go test ./...`
Expected: PASS

**Step 2: Build and install**

Run: `cd /Users/davidberget/github/cctask-go && go install ./... && cp ~/go/bin/cctask-go /opt/homebrew/bin/cctask`

**Step 3: Manual smoke test**

1. Run `cctask`
2. Press `a` → add a task with title "test"
3. Tab to Description → type `/` → verify autocomplete shows skills
4. Tab to Skills → press Enter → verify MultiCheck picker opens
5. Select a skill, confirm → verify it shows on form
6. Submit → verify task shows skills in detail panel
7. Press `r` to run the task → verify skill prompt is included

**Step 4: Commit any final fixes**

```bash
git add -A
git commit -m "chore: final integration cleanup for skill support"
```
