package types

import "time"

// Schedule types
const (
	AgentScheduleTypeOnce    = "once"
	AgentScheduleTypeDaily   = "daily"
	AgentScheduleTypeWeekly  = "weekly"
	AgentScheduleTypeMonthly = "monthly"
)

// Status values
const (
	AgentSchedulerStatusActive   = "active"
	AgentSchedulerStatusPaused   = "paused"
	AgentSchedulerStatusFinished = "finished"
)

const (
	AgentSchedulerTaskStatusRunning = "running"
	AgentSchedulerTaskStatusSuccess = "success"
	AgentSchedulerTaskStatusFailed  = "failed"
)

// AgentSchedulerQueueName is the Temporal task queue used for agent scheduler workflows and workers.
const AgentSchedulerQueueName = "workflow_agent_scheduler_queue"

// Request/Response types
type CreateAgentSchedulerRequest struct {
	InstanceID   int64      `json:"-"` // set from route path
	Name         string     `json:"name" binding:"required,max=255"`
	Prompt       string     `json:"prompt" binding:"required"`
	ScheduleType string     `json:"schedule_type" binding:"required,oneof=once daily weekly monthly"`
	StartDate    time.Time  `json:"start_date" binding:"required"`
	StartTime    time.Time  `json:"start_time" binding:"required"`
	EndDate      *time.Time `json:"end_date,omitempty"`
}

type UpdateAgentSchedulerRequest struct {
	Name         *string    `json:"name,omitempty" binding:"omitempty,max=255"`
	Prompt       *string    `json:"prompt,omitempty"`
	ScheduleType *string    `json:"schedule_type,omitempty" binding:"omitempty,oneof=once daily weekly monthly"`
	StartDate    *time.Time `json:"start_date,omitempty"`
	StartTime    *time.Time `json:"start_time,omitempty"`
	EndDate      *time.Time `json:"end_date,omitempty"`
	Status       *string    `json:"status,omitempty" binding:"omitempty,oneof=active paused finished"`
}

type AgentSchedulerResponse struct {
	ID             int64      `json:"id"`
	InstanceID     int64      `json:"instance_id"`
	Name           string     `json:"name"`
	Prompt         string     `json:"prompt"`
	ScheduleType   string     `json:"schedule_type"`
	CronExpression string     `json:"cron_expression"`
	StartDate      time.Time  `json:"start_date"`
	StartTime      time.Time  `json:"start_time"`
	EndDate        *time.Time `json:"end_date,omitempty"`
	Status         string     `json:"status"`
	LastRunAt      *time.Time `json:"last_run_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type AgentSchedulerTaskResponse struct {
	ID           int64      `json:"id"`
	SchedulerID  int64      `json:"scheduler_id"`
	Name         string     `json:"name"`
	WorkflowID   string     `json:"workflow_id,omitempty"`
	SessionUUID  string     `json:"session_uuid,omitempty"`
	Status       string     `json:"status"`
	ErrorMessage string     `json:"error_message,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

type AgentSchedulerFilter struct {
	InstanceID   *int64
	Status       string
	ScheduleType string
	Search       string // search by name (partial, case-insensitive)
	// NotFinished when non-nil excludes: once-type schedulers, and recurring schedulers whose end_date has passed (*true = apply filter)
	NotFinished *bool
}

// AgentSchedulerTaskFilter filters for listing scheduler tasks
type AgentSchedulerTaskFilter struct {
	Search      string // search by task name (partial, case-insensitive)
	Status      string // running, success, failed
	SchedulerID *int64 // optional filter by scheduler
}

// UpdateAgentSchedulerTaskRequest is the request body for updating a scheduler task's status.
type UpdateAgentSchedulerTaskRequest struct {
	Status       string  `json:"status" binding:"required,oneof=running success failed"`
	Name         *string `json:"name,omitempty" binding:"omitempty,max=255"`
	ErrorMessage *string `json:"error_message,omitempty"`
	SessionUUID  *string `json:"session_uuid,omitempty"`
}

// Workflow input
type AgentSchedulerWorkflowInput struct {
	SchedulerID  int64  `json:"scheduler_id"`
	UserUUID     string `json:"user_uuid"`
	InstanceID   int64  `json:"instance_id"`
	ContentID    string `json:"content_id"`
	InstanceType string `json:"instance_type"`
	Prompt       string `json:"prompt"`
}
