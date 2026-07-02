package app

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"martie/internal/ptchan"
)

type metrics struct {
	registry *prometheus.Registry

	workflowRuns        *prometheus.CounterVec
	workflowDuration    *prometheus.HistogramVec
	workflowLastSuccess *prometheus.GaugeVec
	workflowLastRun     *prometheus.GaugeVec
	notifications       *prometheus.CounterVec

	catalogThreads          *prometheus.GaugeVec
	catalogTrackedThreads   *prometheus.GaugeVec
	catalogReplyPosts       *prometheus.GaugeVec
	catalogReplyFiles       *prometheus.GaugeVec
	catalogAverageThreadAge *prometheus.GaugeVec
	catalogAverageBumpAge   *prometheus.GaugeVec
	catalogOldestThreadAge  *prometheus.GaugeVec
	catalogOldestBumpAge    *prometheus.GaugeVec
}

func newMetrics() *metrics {
	m := &metrics{
		registry: prometheus.NewRegistry(),
		workflowRuns: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "martie_workflow_runs_total",
			Help: "Completed workflow runs by result.",
		}, []string{"workflow", "result"}),
		workflowDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "martie_workflow_duration_seconds",
			Help:    "Workflow run duration in seconds.",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 12),
		}, []string{"workflow"}),
		workflowLastSuccess: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "martie_workflow_last_success",
			Help: "Whether the last workflow run succeeded.",
		}, []string{"workflow"}),
		workflowLastRun: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "martie_workflow_last_successful_timestamp_seconds",
			Help: "Unix timestamp of the last successful workflow run.",
		}, []string{"workflow"}),
		notifications: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "martie_notifications_sent_total",
			Help: "Notifications sent by source.",
		}, []string{"source"}),
		catalogThreads:          newBoardGauge("martie_ptchan_catalog_threads", "Threads in the most recent catalog."),
		catalogTrackedThreads:   newBoardGauge("martie_ptchan_catalog_tracked_threads", "Threads in the most recent catalog that match martie filters."),
		catalogReplyPosts:       newBoardGauge("martie_ptchan_catalog_reply_posts", "Reply posts in the most recent catalog."),
		catalogReplyFiles:       newBoardGauge("martie_ptchan_catalog_reply_files", "Reply files in the most recent catalog."),
		catalogAverageThreadAge: newBoardGauge("martie_ptchan_catalog_average_thread_age_seconds", "Average age of threads in the most recent catalog."),
		catalogAverageBumpAge:   newBoardGauge("martie_ptchan_catalog_average_bump_age_seconds", "Average time since the last bump in the most recent catalog."),
		catalogOldestThreadAge:  newBoardGauge("martie_ptchan_catalog_oldest_thread_age_seconds", "Age of the oldest thread in the most recent catalog."),
		catalogOldestBumpAge:    newBoardGauge("martie_ptchan_catalog_oldest_bump_age_seconds", "Time since the oldest bump in the most recent catalog."),
	}

	m.registry.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "martie_up",
			Help: "Whether the martie process is running.",
		}, func() float64 { return 1 }),
		m.workflowRuns,
		m.workflowDuration,
		m.workflowLastSuccess,
		m.workflowLastRun,
		m.notifications,
		m.catalogThreads,
		m.catalogTrackedThreads,
		m.catalogReplyPosts,
		m.catalogReplyFiles,
		m.catalogAverageThreadAge,
		m.catalogAverageBumpAge,
		m.catalogOldestThreadAge,
		m.catalogOldestBumpAge,
	)

	return m
}

func newBoardGauge(name, help string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	}, []string{"board"})
}

func (m *metrics) handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *metrics) observeWorkflow(name string, duration time.Duration, err error) {
	result := "success"
	success := 1.0
	if err != nil {
		result = "error"
		success = 0
	}

	m.workflowRuns.WithLabelValues(name, result).Inc()
	m.workflowDuration.WithLabelValues(name).Observe(duration.Seconds())
	m.workflowLastSuccess.WithLabelValues(name).Set(success)
	if err == nil {
		m.workflowLastRun.WithLabelValues(name).SetToCurrentTime()
	}
}

func (m *metrics) addNotifications(source string, count int) {
	if count > 0 {
		m.notifications.WithLabelValues(source).Add(float64(count))
	}
}

func (m *metrics) observeCatalog(catalog ptchan.Catalog, cfg CatalogConfig, now time.Time) {
	threads := make(map[string]int)
	trackedThreads := make(map[string]int)
	replyPosts := make(map[string]int)
	replyFiles := make(map[string]int)
	threadAges := make(map[string]float64)
	bumpAges := make(map[string]float64)
	oldestThread := make(map[string]float64)
	oldestBump := make(map[string]float64)
	threadAgeCounts := make(map[string]int)
	bumpAgeCounts := make(map[string]int)

	for _, thread := range catalog.Threads {
		threads[thread.Board]++
		replyPosts[thread.Board] += thread.ReplyPosts
		replyFiles[thread.Board] += thread.ReplyFiles

		if !thread.Date.IsZero() {
			age := now.Sub(thread.Date).Seconds()
			threadAges[thread.Board] += age
			threadAgeCounts[thread.Board]++
			oldestThread[thread.Board] = max(oldestThread[thread.Board], age)
		}
		if !thread.Bumped.IsZero() {
			age := now.Sub(thread.Bumped).Seconds()
			bumpAges[thread.Board] += age
			bumpAgeCounts[thread.Board]++
			oldestBump[thread.Board] = max(oldestBump[thread.Board], age)
		}

		if cfg.Filter.Allows(thread, now) {
			trackedThreads[thread.Board]++
		}
	}

	for board, count := range threadAgeCounts {
		threadAges[board] /= float64(count)
	}
	for board, count := range bumpAgeCounts {
		bumpAges[board] /= float64(count)
	}

	setBoardGauges(m.catalogThreads, threads)
	setBoardGauges(m.catalogTrackedThreads, trackedThreads)
	setBoardGauges(m.catalogReplyPosts, replyPosts)
	setBoardGauges(m.catalogReplyFiles, replyFiles)
	setBoardGauges(m.catalogAverageThreadAge, threadAges)
	setBoardGauges(m.catalogAverageBumpAge, bumpAges)
	setBoardGauges(m.catalogOldestThreadAge, oldestThread)
	setBoardGauges(m.catalogOldestBumpAge, oldestBump)
}

func setBoardGauges[V int | float64](gauges *prometheus.GaugeVec, values map[string]V) {
	gauges.Reset()
	for board, value := range values {
		gauges.WithLabelValues(board).Set(float64(value))
	}
}
