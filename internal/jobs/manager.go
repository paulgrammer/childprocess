package jobs

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/paulgrammer/childprocess/internal/executor"
	"github.com/paulgrammer/childprocess/internal/webhook"
)

type Manager struct {
	concurrency int
	jobsChan    chan string
	wg          sync.WaitGroup
	stopped     atomic.Bool
	store       Store
	sender      webhook.Sender
	runner      executor.Runner
	streamer    *LogStreamer
}

func NewManager(poolSize int, store Store, sender webhook.Sender, runner executor.Runner, streamer *LogStreamer) (*Manager, error) {
	if poolSize <= 0 {
		return nil, errors.New("pool size must be > 0")
	}

	m := &Manager{
		concurrency: poolSize,
		jobsChan:    make(chan string, 1024),
		store:       store,
		sender:      sender,
		runner:      runner,
		streamer:    streamer,
	}
	for i := 0; i < m.concurrency; i++ {
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			for id := range m.jobsChan {
				m.execute(id)
			}
		}()
	}
	return m, nil
}

func (m *Manager) Stop() {
	if m.stopped.Swap(true) {
		return
	}
	close(m.jobsChan)
	m.wg.Wait()
}

func (m *Manager) Submit(ctx context.Context, req CreateJobRequest) (string, error) {
	id := uuid.NewString()
	job := &Job{
		ID:         id,
		Command:    req.Command,
		Args:       req.Args,
		WorkingDir: req.WorkingDir,
		WebhookURL: req.WebhookURL,
		Metadata:   req.Metadata,
		Status:     JobStatusQueued,
		CreatedAt:  time.Now().UTC(),
	}
	if err := m.store.Create(job); err != nil {
		return "", err
	}
	JobsQueuedTotal.Inc()
	JobsActive.Inc()
	// Notify queued
	defer m.notify(ctx, *job)
	if m.stopped.Load() {
		return "", errors.New("manager stopped")
	}
	// Enqueue; may block if queue is full
	m.jobsChan <- id
	return id, nil
}

func (m *Manager) Get(id string) (Job, bool) {
	j, ok := m.store.Get(id)
	if !ok {
		return Job{}, false
	}
	return *j, true
}

func (m *Manager) execute(id string) {
	ctx := context.Background()
	job, ok := m.store.Get(id)
	if !ok {
		slog.Warn("job not found", "job_id", id)
		return
	}
	now := time.Now().UTC()
	job.Status = JobStatusInProgress
	job.StartedAt = &now
	_ = m.store.Update(job)
	m.notify(ctx, *job)
	JobsInProgress.Inc()

	// Streamer
	m.streamer.Broadcast(job.ID, []byte("Job started...\n"))
	defer m.streamer.Close(job.ID)

	// Create a writer that broadcasts to the streamer
	writer := &logStreamWriter{streamer: m.streamer, jobID: job.ID}

	result, err := m.runner.Run(ctx, job.ID, job.Command, job.Args, job.WorkingDir, writer, writer)
	if err != nil {
		job.Status = JobStatusFailed
		job.Error = err.Error()
		_ = m.store.Update(job)
		m.notify(ctx, *job)
		JobsInProgress.Dec()
		JobsFailedTotal.Inc()
		m.streamer.Broadcast(job.ID, []byte("Job failed: "+err.Error()+"\n"))
		return
	}

	// Update job with results
	job.ExitCode = &result.ExitCode
	job.Stdout = &result.Stdout
	job.Stderr = &result.Stderr

	slog.Info("job execution completed",
		"job_id", job.ID,
		"exit_code", result.ExitCode,
		"stdout", result.Stdout,
		"stderr", result.Stderr,
		"duration", result.Duration.String(),
		"error", result.Error,
	)

	done := time.Now().UTC()
	job.Status = JobStatusCompleted
	job.CompletedAt = &done
	_ = m.store.Update(job)
	m.notify(ctx, *job)
	JobsInProgress.Dec()
	JobsCompletedTotal.Inc()
}

func (m *Manager) notify(ctx context.Context, job Job) {
	if job.WebhookURL == "" {
		return
	}
	_ = m.sender.Notify(ctx, job.WebhookURL, webhook.Event{
		JobID:     job.ID,
		Data:      job,
		Status:    string(job.Status),
		Error:     job.Error,
		Timestamp: time.Now().UTC(),
		Metadata:  job.Metadata,
	})
}

type logStreamWriter struct {
	streamer *LogStreamer
	jobID    string
}

func (l *logStreamWriter) Write(p []byte) (n int, err error) {
	l.streamer.Broadcast(l.jobID, p)
	return len(p), nil
}
