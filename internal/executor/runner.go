package executor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"
)

type Runner interface {
	Run(ctx context.Context, jobID string, command string, args []string, workingDir string) error
}

func NewExecRunner() Runner {
	return execRunner{
		command: os.Getenv("DEFAULT_COMMAND"),
	}
}

type execRunner struct {
	command string
}

func (er execRunner) Run(ctx context.Context, jobID string, command string, args []string, workingDir string) error {
	start := time.Now()
	slog.Info("Starting job", "job_id", jobID, "start_time", start.Format(time.RFC3339))

	if command == "" && er.command == "" {
		return errors.New("command must not be empty")
	}

	if command == "" {
		command = er.command
	}

	slog.Info("Executing command", "command", command, "args", args, "working_dir", workingDir)

	cmd := exec.CommandContext(ctx, command, args...)
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	endTime := time.Now()
	elapsed := time.Since(start)

	slog.Info("Job completed",
		"job_id", jobID,
		"start_time", start.Format(time.RFC3339),
		"end_time", endTime.Format(time.RFC3339),
		"elapsed_time", elapsed.String(),
	)

	return nil
}
