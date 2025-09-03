package jobs

import (
    "time"
)

type JobStatus string

const (
    JobStatusQueued     JobStatus = "queued"
    JobStatusInProgress JobStatus = "in_progress"
    JobStatusCompleted  JobStatus = "completed"
    JobStatusFailed     JobStatus = "failed"
)

type CreateJobRequest struct {
    Command     string   `json:"command"`
    Args        []string `json:"args,omitempty"`
    WorkingDir  string   `json:"working_dir,omitempty"`
    WebhookURL  string   `json:"webhook_url"`
    Metadata    map[string]string `json:"metadata,omitempty"`
}

type Job struct {
    ID          string            `json:"id"`
    Command     string            `json:"command"`
    Args        []string          `json:"args,omitempty"`
    WorkingDir  string            `json:"working_dir,omitempty"`
    WebhookURL  string            `json:"webhook_url"`
    Metadata    map[string]string `json:"metadata,omitempty"`

    Status      JobStatus         `json:"status"`
    Error       string            `json:"error,omitempty"`
    CreatedAt   time.Time         `json:"created_at"`
    StartedAt   *time.Time        `json:"started_at,omitempty"`
    CompletedAt *time.Time        `json:"completed_at,omitempty"`
}


