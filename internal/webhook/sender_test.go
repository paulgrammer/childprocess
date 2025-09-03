package webhook

import (
    "context"
    "net/http"
    "net/http/httptest"
    "sync/atomic"
    "testing"
    "time"
)

func TestHTTPSender_Success(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
    defer srv.Close()

    s := NewHTTPSender(2*time.Second, 0)
    ctx := context.Background()
    err := s.Notify(ctx, srv.URL, Event{JobID: "1", Status: "queued", Timestamp: time.Now()})
    if err != nil {
        t.Fatalf("expected success, got error: %v", err)
    }
}

func TestHTTPSender_RetryThenSuccess(t *testing.T) {
    var hits int32
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if atomic.AddInt32(&hits, 1) < 3 {
            http.Error(w, "boom", http.StatusInternalServerError)
            return
        }
        w.WriteHeader(http.StatusOK)
    }))
    defer srv.Close()

    s := NewHTTPSender(2*time.Second, 5)
    ctx := context.Background()
    start := time.Now()
    err := s.Notify(ctx, srv.URL, Event{JobID: "2", Status: "queued", Timestamp: time.Now()})
    if err != nil {
        t.Fatalf("expected eventual success, got error: %v", err)
    }
    if atomic.LoadInt32(&hits) < 3 {
        t.Fatalf("expected at least 3 attempts, got %d", hits)
    }
    if time.Since(start) < 500*time.Millisecond {
        t.Fatalf("expected backoff delay to elapse, too fast: %s", time.Since(start))
    }
}

func TestHTTPSender_ExhaustRetries(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        http.Error(w, "boom", http.StatusInternalServerError)
    }))
    defer srv.Close()

    s := NewHTTPSender(500*time.Millisecond, 2)
    ctx := context.Background()
    err := s.Notify(ctx, srv.URL, Event{JobID: "3", Status: "queued", Timestamp: time.Now()})
    if err == nil {
        t.Fatalf("expected error after exhausting retries")
    }
}

func TestHTTPSender_ContextCancel(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        time.Sleep(2 * time.Second)
        w.WriteHeader(http.StatusOK)
    }))
    defer srv.Close()

    s := NewHTTPSender(5*time.Second, 3)
    ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
    defer cancel()
    err := s.Notify(ctx, srv.URL, Event{JobID: "4", Status: "queued", Timestamp: time.Now()})
    if err == nil {
        t.Fatalf("expected context timeout error")
    }
}


