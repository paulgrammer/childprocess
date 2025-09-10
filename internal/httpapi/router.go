package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/paulgrammer/childprocess/internal/jobs"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins
	},
}

type router struct {
	manager  *jobs.Manager
	streamer *jobs.LogStreamer
}

func NewRouter(manager *jobs.Manager, streamer *jobs.LogStreamer) http.Handler {
	r := &router{manager: manager, streamer: streamer}
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", r.handleHealth)
	m.HandleFunc("POST /jobs", r.handleJobs)
	m.HandleFunc("GET /jobs/{id}", r.handleJob)
	m.HandleFunc("GET /jobs/{id}/logs", r.handleJobLogs)
	m.Handle("GET /metrics", promhttp.Handler())
	m.Handle("/", http.FileServer(http.Dir("./frontend")))
	return logging(m)
}

func (r *router) handleJobs(w http.ResponseWriter, req *http.Request) {
	var body jobs.CreateJobRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid json")
		return
	}

	// Ensure command is set safely
	if body.Command == "" {
		if len(body.Args) > 0 {
			// use first arg as command, rest as args
			body.Command = body.Args[0]
			body.Args = body.Args[1:]
		}
	}

	id, err := r.manager.Submit(req.Context(), body)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to queue job")
		return
	}
	respondWithJSON(w, http.StatusAccepted, map[string]string{"job_id": id, "status": string(jobs.JobStatusQueued)})
}

func (r *router) handleJob(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "job id required")
		return
	}
	job, ok := r.manager.Get(id)
	if !ok {
		respondWithError(w, http.StatusNotFound, "not found")
		return
	}
	respondWithJSON(w, http.StatusOK, job)
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start).String())
	})
}

func (r *router) handleHealth(w http.ResponseWriter, req *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *router) handleJobLogs(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	if id == "" {
		respondWithError(w, http.StatusBadRequest, "job id required")
		return
	}

	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		slog.Error("failed to upgrade connection", "error", err)
		return
	}

	r.streamer.Subscribe(id, conn)
	defer r.streamer.Unsubscribe(id, conn)

	// Keep the connection open
	for {
		if _, _, err := conn.NextReader(); err != nil {
			conn.Close()
			break
		}
	}
}
