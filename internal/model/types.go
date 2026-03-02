package model

import "time"

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in-progress"
	StatusDone       TaskStatus = "done"
)

func (s TaskStatus) Next() TaskStatus {
	switch s {
	case StatusPending:
		return StatusInProgress
	case StatusInProgress:
		return StatusDone
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
	PlanFile    string     `json:"planFile,omitempty"`
	Created     string     `json:"created"`
	Updated     string     `json:"updated"`
}

type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
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

type Config struct {
	Model  string `json:"model,omitempty"`
	Budget int    `json:"budget,omitempty"`
}

type ProcessStatus string

const (
	ProcessRunning ProcessStatus = "running"
	ProcessDone    ProcessStatus = "done"
	ProcessError   ProcessStatus = "error"
)

type ClaudeProcess struct {
	ID      string
	Label   string
	Status  ProcessStatus
	Output  string
	LogFile string
}

func Now() string {
	return time.Now().Format(time.RFC3339)
}

func NowTime() time.Time {
	return time.Now()
}
