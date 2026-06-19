package app

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"martie/internal/ptchan"
)

type metrics struct {
	mu sync.RWMutex

	pollSuccessTotal uint64
	pollErrorTotal   uint64

	lastPollDuration       float64
	lastPollSuccess        bool
	lastSuccessfulPollTime time.Time

	threadsByBoard        map[string]int
	trackedThreadsByBoard map[string]int
	replyPostsByBoard     map[string]int
	replyFilesByBoard     map[string]int
	threadAgeByBoard      map[string]float64
	bumpAgeByBoard        map[string]float64
	oldestThreadByBoard   map[string]float64
	oldestBumpByBoard     map[string]float64

	notificationsSent uint64
}

func newMetrics() *metrics {
	return &metrics{
		threadsByBoard:        make(map[string]int),
		trackedThreadsByBoard: make(map[string]int),
		replyPostsByBoard:     make(map[string]int),
		replyFilesByBoard:     make(map[string]int),
		threadAgeByBoard:      make(map[string]float64),
		bumpAgeByBoard:        make(map[string]float64),
		oldestThreadByBoard:   make(map[string]float64),
		oldestBumpByBoard:     make(map[string]float64),
	}
}

func (m *metrics) observePoll(duration time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastPollDuration = duration.Seconds()
	m.lastPollSuccess = err == nil
	if err == nil {
		m.pollSuccessTotal++
		return
	}

	m.pollErrorTotal++
}

func (m *metrics) observeCatalog(catalog ptchan.Catalog, cfg Config, now time.Time) {
	threadsByBoard := make(map[string]int)
	trackedThreadsByBoard := make(map[string]int)
	replyPostsByBoard := make(map[string]int)
	replyFilesByBoard := make(map[string]int)
	threadAgeByBoard := make(map[string]float64)
	bumpAgeByBoard := make(map[string]float64)
	oldestThreadByBoard := make(map[string]float64)
	oldestBumpByBoard := make(map[string]float64)
	threadAgeCounts := make(map[string]int)
	bumpAgeCounts := make(map[string]int)

	for _, thread := range catalog.Threads {
		threadsByBoard[thread.Board]++
		replyPostsByBoard[thread.Board] += thread.ReplyPosts
		replyFilesByBoard[thread.Board] += thread.ReplyFiles

		if !thread.Date.IsZero() {
			age := now.Sub(thread.Date).Seconds()
			threadAgeByBoard[thread.Board] += age
			threadAgeCounts[thread.Board]++
			oldestThreadByBoard[thread.Board] = max(oldestThreadByBoard[thread.Board], age)
		}
		if !thread.Bumped.IsZero() {
			age := now.Sub(thread.Bumped).Seconds()
			bumpAgeByBoard[thread.Board] += age
			bumpAgeCounts[thread.Board]++
			oldestBumpByBoard[thread.Board] = max(oldestBumpByBoard[thread.Board], age)
		}

		if threadAllowed(cfg, thread, now) {
			trackedThreadsByBoard[thread.Board]++
		}
	}

	for board, count := range threadAgeCounts {
		threadAgeByBoard[board] /= float64(count)
	}
	for board, count := range bumpAgeCounts {
		bumpAgeByBoard[board] /= float64(count)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastSuccessfulPollTime = now
	m.threadsByBoard = threadsByBoard
	m.trackedThreadsByBoard = trackedThreadsByBoard
	m.replyPostsByBoard = replyPostsByBoard
	m.replyFilesByBoard = replyFilesByBoard
	m.threadAgeByBoard = threadAgeByBoard
	m.bumpAgeByBoard = bumpAgeByBoard
	m.oldestThreadByBoard = oldestThreadByBoard
	m.oldestBumpByBoard = oldestBumpByBoard
}

func (m *metrics) addNotifications(count int) {
	if count == 0 {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.notificationsSent += uint64(count)
}

func (m *metrics) render() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var builder strings.Builder
	writeGauge(&builder, "martie_up", "Whether the martie process is running.", nil, 1)
	writeHelp(&builder, "martie_poll_total", "Completed martie poll cycles by result.", "counter")
	writeMetric(&builder, "martie_poll_total", map[string]string{"result": "success"}, float64(m.pollSuccessTotal))
	writeMetric(&builder, "martie_poll_total", map[string]string{"result": "error"}, float64(m.pollErrorTotal))
	writeGauge(&builder, "martie_last_poll_success", "Whether the last martie poll cycle succeeded.", nil, boolFloat(m.lastPollSuccess))
	writeGauge(&builder, "martie_last_poll_duration_seconds", "Duration of the last martie poll cycle.", nil, m.lastPollDuration)

	writeGauge(&builder, "martie_catalog_last_successful_poll_timestamp_seconds", "Unix timestamp for the last successful catalog poll.", nil, timestampSeconds(m.lastSuccessfulPollTime))
	writeCounter(&builder, "martie_notifications_sent_total", "Notifications sent by this martie process.", nil, float64(m.notificationsSent))
	writeGaugeMap(&builder, "martie_catalog_threads", "Threads in the most recent catalog poll.", "board", m.threadsByBoard)
	writeGaugeMap(&builder, "martie_catalog_tracked_threads", "Threads from the most recent catalog poll that match martie filters.", "board", m.trackedThreadsByBoard)
	writeGaugeMap(&builder, "martie_catalog_reply_posts", "Reply posts in the most recent catalog poll.", "board", m.replyPostsByBoard)
	writeGaugeMap(&builder, "martie_catalog_reply_files", "Reply files in the most recent catalog poll.", "board", m.replyFilesByBoard)
	writeGaugeMap(&builder, "martie_catalog_average_thread_age_seconds", "Average age of threads in the most recent catalog poll.", "board", m.threadAgeByBoard)
	writeGaugeMap(&builder, "martie_catalog_average_bump_age_seconds", "Average time since the last bump for threads in the most recent catalog poll.", "board", m.bumpAgeByBoard)
	writeGaugeMap(&builder, "martie_catalog_oldest_thread_age_seconds", "Age of the oldest thread in the most recent catalog poll.", "board", m.oldestThreadByBoard)
	writeGaugeMap(&builder, "martie_catalog_oldest_bump_age_seconds", "Time since the oldest bump in the most recent catalog poll.", "board", m.oldestBumpByBoard)

	return builder.String()
}

func writeGaugeMap[V int | float64](builder *strings.Builder, name, help, label string, values map[string]V) {
	writeHelp(builder, name, help, "gauge")
	for _, key := range sortedKeys(values) {
		writeMetric(builder, name, map[string]string{label: key}, float64(values[key]))
	}
}

func writeGauge(builder *strings.Builder, name, help string, labels map[string]string, value float64) {
	writeHelp(builder, name, help, "gauge")
	writeMetric(builder, name, labels, value)
}

func writeCounter(builder *strings.Builder, name, help string, labels map[string]string, value float64) {
	writeHelp(builder, name, help, "counter")
	writeMetric(builder, name, labels, value)
}

func writeHelp(builder *strings.Builder, name, help, metricType string) {
	fmt.Fprintf(builder, "# HELP %s %s\n", name, help)
	fmt.Fprintf(builder, "# TYPE %s %s\n", name, metricType)
}

func writeMetric(builder *strings.Builder, name string, labels map[string]string, value float64) {
	builder.WriteString(name)
	if len(labels) > 0 {
		builder.WriteByte('{')
		keys := sortedKeys(labels)
		for index, key := range keys {
			if index > 0 {
				builder.WriteByte(',')
			}
			fmt.Fprintf(builder, `%s="%s"`, key, escapeLabelValue(labels[key]))
		}
		builder.WriteByte('}')
	}
	builder.WriteByte(' ')
	builder.WriteString(strconv.FormatFloat(value, 'f', -1, 64))
	builder.WriteByte('\n')
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func escapeLabelValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func timestampSeconds(value time.Time) float64 {
	if value.IsZero() {
		return 0
	}
	return float64(value.Unix())
}
