package jobs

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	JobsQueuedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "jobs_queued_total",
		Help: "Total number of jobs queued",
	})
	JobsInProgress = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "jobs_in_progress",
		Help: "Number of jobs currently in progress",
	})
	JobsCompletedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "jobs_completed_total",
		Help: "Total number of jobs completed successfully",
	})
	JobsFailedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "jobs_failed_total",
		Help: "Total number of jobs failed",
	})
	JobsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "jobs_active",
		Help: "Number of jobs known to the system (not GC'd)",
	})
)

func init() {
	prometheus.MustRegister(JobsQueuedTotal, JobsInProgress, JobsCompletedTotal, JobsFailedTotal, JobsActive)
}
