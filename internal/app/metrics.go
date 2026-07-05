package app

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"martie/internal/deepseek"
	"martie/internal/ptchan"
)

type metrics struct {
	registry *prometheus.Registry

	workflowRuns             *prometheus.CounterVec
	workflowDuration         *prometheus.HistogramVec
	workflowLastSuccess      *prometheus.GaugeVec
	workflowLastRun          *prometheus.GaugeVec
	notifications            *prometheus.CounterVec
	assistantUpdates         *prometheus.CounterVec
	assistantResponses       *prometheus.CounterVec
	assistantContextRequests *prometheus.CounterVec
	activeConversations      prometheus.Gauge
	aiRequests               *prometheus.CounterVec
	aiDuration               prometheus.Histogram
	aiTokens                 *prometheus.CounterVec

	catalogThreads          *prometheus.GaugeVec
	catalogTrackedThreads   *prometheus.GaugeVec
	catalogReplyPosts       *prometheus.GaugeVec
	catalogReplyFiles       *prometheus.GaugeVec
	catalogAverageThreadAge *prometheus.GaugeVec
	catalogAverageBumpAge   *prometheus.GaugeVec
	catalogOldestThreadAge  *prometheus.GaugeVec
	catalogOldestBumpAge    *prometheus.GaugeVec
}

const (
	metricResultSuccess = "success"
	metricResultError   = "error"
)

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
		assistantUpdates: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "martie_assistant_updates_total",
			Help: "Assistant updates by admission result.",
		}, []string{"result"}),
		assistantResponses: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "martie_assistant_responses_total",
			Help: "Assistant responses by delivery result.",
		}, []string{"result"}),
		assistantContextRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "martie_assistant_context_requests_total",
			Help: "Assistant requests that include recent history or replied-to message context.",
		}, []string{"type"}),
		activeConversations: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "martie_assistant_active_conversations",
			Help: "In-memory conversations with unexpired history.",
		}),
		aiRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "martie_ai_requests_total",
			Help: "AI completion requests by result and finish reason.",
		}, []string{"result", "finish_reason"}),
		aiDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "martie_ai_request_duration_seconds",
			Help:    "AI completion request duration in seconds.",
			Buckets: prometheus.ExponentialBuckets(0.5, 2, 10),
		}),
		aiTokens: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "martie_ai_tokens_total",
			Help: "AI tokens consumed by mutually exclusive input cache status or output.",
		}, []string{"type"}),
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
		m.assistantUpdates,
		m.assistantResponses,
		m.assistantContextRequests,
		m.activeConversations,
		m.aiRequests,
		m.aiDuration,
		m.aiTokens,
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
	result := metricResultSuccess
	success := 1.0
	if err != nil {
		result = metricResultError
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

func (m *metrics) observeAssistantUpdate(result admissionResult) {
	m.assistantUpdates.WithLabelValues(string(result)).Inc()
}

func (m *metrics) observeAssistantResponse(result string) {
	m.assistantResponses.WithLabelValues(result).Inc()
}

func (m *metrics) observeAssistantContext(contextType string) {
	m.assistantContextRequests.WithLabelValues(contextType).Inc()
}

func (m *metrics) setActiveConversations(count int) {
	m.activeConversations.Set(float64(count))
}

func (m *metrics) observeAICompletion(duration time.Duration, completion deepseek.Completion, err error) {
	m.aiDuration.Observe(duration.Seconds())
	if err != nil {
		m.aiRequests.WithLabelValues(metricResultError, "").Inc()
		return
	}
	m.aiRequests.WithLabelValues(metricResultSuccess, string(completion.FinishReason)).Inc()
	m.aiTokens.WithLabelValues("input_cache_hit").Add(float64(completion.Usage.PromptCacheHitTokens))
	m.aiTokens.WithLabelValues("input_cache_miss").Add(float64(completion.Usage.PromptCacheMissTokens))
	m.aiTokens.WithLabelValues("output").Add(float64(completion.Usage.CompletionTokens))
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
