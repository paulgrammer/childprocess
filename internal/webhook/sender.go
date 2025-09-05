package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type Event struct {
	JobID     string            `json:"job_id"`
	Status    string            `json:"status"`
	Error     string            `json:"error,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Data      any               `json:"data,omitempty"`
}

type Sender interface {
	Notify(ctx context.Context, url string, event Event) error
}

type httpsender struct {
	client      *http.Client
	maxRetries  int
	baseBackoff time.Duration
}

func NewHTTPSender(timeout time.Duration, maxRetries int) Sender {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if maxRetries < 0 {
		maxRetries = 3
	}
	return &httpsender{
		client:      &http.Client{Timeout: timeout},
		maxRetries:  maxRetries,
		baseBackoff: 500 * time.Millisecond,
	}
}

func (s *httpsender) Notify(ctx context.Context, url string, event Event) error {
	body, _ := json.Marshal(event)
	var lastErr error
	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("content-type", "application/json")
		resp, err := s.client.Do(req)
		if err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if resp.Body != nil {
				_ = resp.Body.Close()
			}
			return nil
		}
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if err == nil {
			lastErr = errors.New(resp.Status)
		} else {
			lastErr = err
		}
		// exponential backoff with jitter
		backoff := s.baseBackoff * (1 << attempt)
		select {
		case <-time.After(backoff + time.Duration(int64(time.Millisecond)*int64(attempt*50))):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return lastErr
}
