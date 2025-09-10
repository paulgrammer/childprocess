package executor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ExecutionResult contains the result of command execution
type ExecutionResult struct {
	JobID     string
	ExitCode  int
	Stdout    string
	Stderr    string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Error     error
}

type Runner interface {
	Run(ctx context.Context, jobID string, command string, args []string, workingDir string, stdout, stderr io.Writer) (*ExecutionResult, error)
}

// ExecutorConfig allows customization of execution behavior
type ExecutorConfig struct {
	DefaultCommand string
	CaptureOutput  bool
	MaxOutputSize  int // bytes, 0 for unlimited
	LogOutput      bool
	StreamOutput   bool // if true, output is streamed in real-time
	VerboseLogging bool // if true, more detailed logs are produced
}

type RunnerOption func(*execRunner)

func WithExecutorConfig(config *ExecutorConfig) RunnerOption {
	return func(r *execRunner) {
		r.config = config
	}
}

func NewExecRunner(args ...RunnerOption) Runner {
	config := &ExecutorConfig{
		DefaultCommand: os.Getenv("DEFAULT_COMMAND"),
		CaptureOutput:  true,
		MaxOutputSize:  1024 * 1024, // 1MB default
		LogOutput:      true,
		StreamOutput:   false,
		VerboseLogging: false,
	}

	runner := &execRunner{config: config}

	for _, arg := range args {
		arg(runner)
	}
	return runner
}

type execRunner struct {
	config *ExecutorConfig
}

func (er *execRunner) Run(ctx context.Context, jobID string, command string, args []string, workingDir string, stdout, stderr io.Writer) (*ExecutionResult, error) {
	if err := er.validateInput(command, jobID); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	result := &ExecutionResult{
		JobID:     jobID,
		StartTime: time.Now(),
	}

	// Use default command if none provided
	if command == "" {
		command = er.config.DefaultCommand
	}

	if er.config.VerboseLogging {
		slog.Info("Starting job execution",
			"job_id", jobID,
			"command", command,
			"args", args,
			"working_dir", workingDir,
			"start_time", result.StartTime.Format(time.RFC3339),
		)
	}

	cmd := exec.CommandContext(ctx, command, args...)
	if workingDir != "" {
		if err := er.validateWorkingDir(workingDir); err != nil {
			return nil, fmt.Errorf("invalid working directory: %w", err)
		}
		cmd.Dir = workingDir
	}

	// Always capture output for visibility
	if er.config.CaptureOutput {
		if er.config.StreamOutput {
			return er.runWithStreamedOutput(cmd, result, stdout, stderr)
		}
		return er.runWithCapturedOutput(cmd, result, stdout, stderr)
	}

	// Even for simple execution, we should capture some output
	return er.runSimpleWithOutput(cmd, result)
}

func (er *execRunner) runWithCapturedOutput(cmd *exec.Cmd, result *ExecutionResult, stdout, stderr io.Writer) (*ExecutionResult, error) {
	var stdoutBuilder, stderrBuilder strings.Builder
	cmd.Stdout = io.MultiWriter(&stdoutBuilder, stdout)
	cmd.Stderr = io.MultiWriter(&stderrBuilder, stderr)

	if err := cmd.Start(); err != nil {
		result.Error = fmt.Errorf("failed to start command: %w", err)
		return result, result.Error
	}

	// Wait for command completion
	err := cmd.Wait()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = -1
		}
		result.Error = fmt.Errorf("command execution failed: %w", err)
	} else {
		result.ExitCode = 0
	}

	er.logExecutionResult(result)
	return result, result.Error
}

func (er *execRunner) runWithStreamedOutput(cmd *exec.Cmd, result *ExecutionResult, stdout, stderr io.Writer) (*ExecutionResult, error) {
	var stdoutBuilder, stderrBuilder strings.Builder
	var wg sync.WaitGroup

	// Create pipes for real-time streaming
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		result.Error = fmt.Errorf("failed to start command: %w", err)
		return result, result.Error
	}

	wg.Add(2)

	// Stream stdout
	go func() {
		defer wg.Done()
		er.streamAndCapture(stdoutPipe, &stdoutBuilder, result.JobID, "stdout", stdout)
	}()

	// Stream stderr
	go func() {
		defer wg.Done()
		er.streamAndCapture(stderrPipe, &stderrBuilder, result.JobID, "stderr", stderr)
	}()

	// Wait for streaming to complete
	wg.Wait()

	err = cmd.Wait()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	result.Stdout = stdoutBuilder.String()
	result.Stderr = stderrBuilder.String()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = -1
		}
		result.Error = fmt.Errorf("command execution failed: %w", err)
	} else {
		result.ExitCode = 0
	}

	er.logExecutionResult(result)
	return result, result.Error
}

func (er *execRunner) runSimpleWithOutput(cmd *exec.Cmd, result *ExecutionResult) (*ExecutionResult, error) {
	// Even in simple mode, capture output for visibility
	var stdoutBuilder, stderrBuilder strings.Builder
	cmd.Stdout = &stdoutBuilder
	cmd.Stderr = &stderrBuilder

	err := cmd.Run()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	result.Stdout = stdoutBuilder.String()
	result.Stderr = stderrBuilder.String()

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = -1
		}
		result.Error = fmt.Errorf("command execution failed: %w", err)
	} else {
		result.ExitCode = 0
	}

	er.logExecutionResult(result)
	return result, result.Error
}

func (er *execRunner) streamAndCapture(reader io.Reader, builder *strings.Builder, jobID, streamType string, writer io.Writer) {
	scanner := bufio.NewScanner(reader)
	maxScanTokenSize := 64 * 1024 // 64KB buffer for long lines
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	for scanner.Scan() {
		line := scanner.Bytes()

		// Write to builder for capture
		if er.config.MaxOutputSize <= 0 || builder.Len() < er.config.MaxOutputSize {
			builder.Write(line)
			builder.WriteString("\n")
		}

		// Stream output in real-time
		if writer != nil {
			writer.Write(append(line, '\n'))
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error("Error scanning output",
			"job_id", jobID,
			"stream", streamType,
			"error", err,
		)
	}
}

func (er *execRunner) validateInput(command, jobID string) error {
	if strings.TrimSpace(jobID) == "" {
		return errors.New("jobID cannot be empty")
	}

	if command == "" && er.config.DefaultCommand == "" {
		return errors.New("command must not be empty and no default command configured")
	}

	return nil
}

func (er *execRunner) validateWorkingDir(workingDir string) error {
	if workingDir == "" {
		return nil
	}

	info, err := os.Stat(workingDir)
	if err != nil {
		return fmt.Errorf("working directory does not exist: %w", err)
	}

	if !info.IsDir() {
		return errors.New("working directory path is not a directory")
	}

	return nil
}

func (er *execRunner) logExecutionResult(result *ExecutionResult) {
	logLevel := slog.LevelInfo
	if result.Error != nil {
		logLevel = slog.LevelError
	}

	attrs := []any{
		"job_id", result.JobID,
		"exit_code", result.ExitCode,
		"duration", result.Duration.String(),
		"stdout_length", len(result.Stdout),
		"stderr_length", len(result.Stderr),
	}

	if result.Error != nil {
		attrs = append(attrs, "error", result.Error.Error())
	}

	slog.Log(context.Background(), logLevel, "Job execution completed", attrs...)

	// Always log output content for debugging (with truncation for safety)
	if er.config.LogOutput || er.config.VerboseLogging {
		if result.Stdout != "" {
			stdoutToLog := result.Stdout
			if len(stdoutToLog) > 1000 {
				stdoutToLog = stdoutToLog[:1000] + "... (truncated)"
			}
			slog.Info("Command stdout", "job_id", result.JobID, "stdout", stdoutToLog)
		}
		if result.Stderr != "" {
			stderrToLog := result.Stderr
			if len(stderrToLog) > 1000 {
				stderrToLog = stderrToLog[:1000] + "... (truncated)"
			}
			slog.Info("Command stderr", "job_id", result.JobID, "stderr", stderrToLog)
		}
	}

	// If no output was captured and command succeeded, log a warning
	if result.ExitCode == 0 && result.Stdout == "" && result.Stderr == "" && er.config.VerboseLogging {
		slog.Warn("Command completed successfully but no output was captured",
			"job_id", result.JobID,
		)
	}
}
