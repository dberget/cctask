package model

import (
	"strings"
	"time"
)

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusPlanning   TaskStatus = "planning"
	StatusInProgress TaskStatus = "in-progress"
	StatusDone       TaskStatus = "done"
	StatusMerged     TaskStatus = "merged"
)

func (s TaskStatus) Next() TaskStatus {
	switch s {
	case StatusPending:
		return StatusPlanning
	case StatusPlanning:
		return StatusInProgress
	case StatusInProgress:
		return StatusDone
	case StatusMerged:
		return StatusMerged
	default:
		return StatusPending
	}
}

type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      TaskStatus `json:"status"`
	Tags        []string   `json:"tags"`
	Group       string     `json:"group,omitempty"`
	WorkDir     string     `json:"workDir,omitempty"`
	Skills      []string   `json:"skills,omitempty"`
	PlanFile    string     `json:"planFile,omitempty"`
	MergedInto  string     `json:"mergedInto,omitempty"`
	ProofBefore string     `json:"proofBefore,omitempty"`
	ProofAfter  string     `json:"proofAfter,omitempty"`
	Created     string     `json:"created"`
	Updated     string     `json:"updated"`
}

// HasTag returns true if the task has the given tag (case-insensitive).
func (t Task) HasTag(tag string) bool {
	tag = strings.ToLower(tag)
	for _, tt := range t.Tags {
		if strings.ToLower(tt) == tag {
			return true
		}
	}
	return false
}

// IsProof returns true if the task is tagged with "PROOF".
func (t Task) IsProof() bool {
	return t.HasTag("PROOF")
}

// HasProof returns true if both proof screenshot paths are set.
func (t Task) HasProof() bool {
	return t.ProofBefore != "" && t.ProofAfter != ""
}

type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ParentGroup string `json:"parentGroup,omitempty"`
	WorkDir     string `json:"workDir,omitempty"`
	PlanFile    string `json:"planFile,omitempty"`
	Created     string `json:"created"`
}

type CombinedPlan struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	SourceTaskIDs []string `json:"sourceTaskIds"`
	PlanFile      string   `json:"planFile"`
	Created       string   `json:"created"`
}

type TaskStore struct {
	Tasks         []Task         `json:"tasks"`
	Groups        []Group        `json:"groups"`
	CombinedPlans []CombinedPlan `json:"combinedPlans"`
	NextID        int            `json:"nextId"`
}

type ServerConfig struct {
	Port      int    `json:"port,omitempty"`
	AuthToken string `json:"authToken,omitempty"`
	Enabled   bool   `json:"enabled,omitempty"`
}

type Config struct {
	Model              string       `json:"model,omitempty"`
	Budget             int          `json:"budget,omitempty"`
	Theme              string       `json:"theme,omitempty"`
	TimeoutMinutes     int          `json:"timeoutMinutes,omitempty"`
	DisableSkillPicker bool         `json:"disableSkillPicker,omitempty"`
	Server             ServerConfig `json:"server,omitempty"`
}

const DefaultTimeoutMinutes = 60

// Timeout returns the configured process timeout as a time.Duration.
func (c Config) Timeout() time.Duration {
	if c.TimeoutMinutes > 0 {
		return time.Duration(c.TimeoutMinutes) * time.Minute
	}
	return DefaultTimeoutMinutes * time.Minute
}

type ProcessStatus string

const (
	ProcessRunning ProcessStatus = "running"
	ProcessDone    ProcessStatus = "done"
	ProcessError   ProcessStatus = "error"
	ProcessWaiting ProcessStatus = "waiting" // Claude finished turn, user can respond
)

// CompletionAction determines what side effect to run when a streaming process finishes.
type CompletionAction int

const (
	CompletionNone             CompletionAction = iota
	CompletionSavePlan                          // Save output as task plan
	CompletionSaveGroupPlan                     // Save output as group plan
	CompletionApplyGroupAction                  // Parse JSON and apply group action
	CompletionCombinePlans                      // Save as combined plan
	CompletionSaveFollowUp                      // Save/update plan from follow-up
	CompletionRunTask                           // Mark task done on completion
	CompletionApplyBulkAdd                      // Parse JSON and create tasks from bulk add
)

// EventKind categorizes streaming events for rendering.
type EventKind int

const (
	EventText       EventKind = iota // Text content from assistant
	EventThinking                    // Thinking/reasoning content
	EventToolUse                     // Tool invocation
	EventToolResult                  // Tool execution result
	EventUserMsg                     // User follow-up message
	EventSystem                      // System/status message
	EventToolQuestion                // MCP tool asking user a question
)

// StreamEvent is a single structured event from a Claude streaming session.
type StreamEvent struct {
	Kind       EventKind
	Text       string
	ToolName   string
	ToolID     string
	ToolInput  string // Truncated preview of tool input
	ToolResult string // Truncated preview of tool result
	IsError    bool
	Timestamp  time.Time
}

type ClaudeProcess struct {
	ID        string
	Label     string
	Status    ProcessStatus
	Output    string // Legacy plain-text output (kept for backward compat)
	LogFile   string
	StartedAt time.Time

	// Streaming fields
	SessionID        string
	Events           []StreamEvent
	IsInteractive    bool             // If true, don't auto-remove on completion
	CompletionAction CompletionAction
	CompletionMeta   map[string]string // Context for completion handler (taskID, groupID, planFile, etc.)
	TurnCount        int
	CostUSD          float64
	QueuedMessage    string // Message queued while process is running, auto-sent on turn completion
}

func Now() string {
	return time.Now().Format(time.RFC3339)
}

func NowTime() time.Time {
	return time.Now()
}
