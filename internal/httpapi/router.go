package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/paulgrammer/childprocess/internal/jobs"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type router struct {
	manager *jobs.Manager
}

func NewRouter(manager *jobs.Manager) http.Handler {
	r := &router{manager: manager}
	m := http.NewServeMux()
	m.HandleFunc("GET /healthz", r.handleHealth)
	m.HandleFunc("POST /jobs", r.handleJobs)
	m.HandleFunc("GET /jobs/{id}", r.handleJob)
	m.Handle("GET /metrics", promhttp.Handler())
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
