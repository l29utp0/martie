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
	lastPollDuration float64
	lastPollSuccess  bool

	ptchanLastSuccessfulPoll time.Time
	ptchanThreads            int
	ptchanTrackedThreads     int
	ptchanReplyPosts         int
	ptchanReplyFiles         int
	ptchanThreadsByBoard     map[string]int
	ptchanReplyPostsByBoard  map[string]int
	ptchanReplyFilesByBoard  map[string]int
	ptchanNotificationsSent  uint64
}

func newMetrics() *metrics {
	return &metrics{
		ptchanThreadsByBoard:    make(map[string]int),
		ptchanReplyPostsByBoard: make(map[string]int),
		ptchanReplyFilesByBoard: make(map[string]int),
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

func (m *metrics) observePtchanCatalog(catalog ptchan.Catalog, cfg Config, now time.Time) {
	threadsByBoard := make(map[string]int)
	replyPostsByBoard := make(map[string]int)
	replyFilesByBoard := make(map[string]int)
	replyPosts := 0
	replyFiles := 0
	trackedThreads := 0

	for _, thread := range catalog.Threads {
		replyPosts += thread.ReplyPosts
		replyFiles += thread.ReplyFiles
		threadsByBoard[thread.Board]++
		replyPostsByBoard[thread.Board] += thread.ReplyPosts
		replyFilesByBoard[thread.Board] += thread.ReplyFiles
		if threadAllowed(cfg, thread, now) {
			trackedThreads++
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.ptchanLastSuccessfulPoll = now
	m.ptchanThreads = len(catalog.Threads)
	m.ptchanTrackedThreads = trackedThreads
	m.ptchanReplyPosts = replyPosts
	m.ptchanReplyFiles = replyFiles
	for board, count := range threadsByBoard {
		m.ptchanThreadsByBoard[board] = count
	}
	for board, count := range replyPostsByBoard {
		m.ptchanReplyPostsByBoard[board] = count
	}
	for board, count := range replyFilesByBoard {
		m.ptchanReplyFilesByBoard[board] = count
	}
}

func (m *metrics) addPtchanNotifications(count int) {
	if count == 0 {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.ptchanNotificationsSent += uint64(count)
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

	writeGauge(&builder, "martie_ptchan_catalog_threads", "Threads in the most recent ptchan catalog poll.", nil, float64(m.ptchanThreads))
	writeGauge(&builder, "martie_ptchan_tracked_threads", "Threads from the most recent ptchan catalog poll that match martie filters.", nil, float64(m.ptchanTrackedThreads))
	writeGauge(&builder, "martie_ptchan_catalog_reply_posts", "Reply posts in the most recent ptchan catalog poll.", nil, float64(m.ptchanReplyPosts))
	writeGauge(&builder, "martie_ptchan_catalog_reply_files", "Reply files in the most recent ptchan catalog poll.", nil, float64(m.ptchanReplyFiles))
	writeGauge(&builder, "martie_ptchan_last_successful_poll_timestamp_seconds", "Unix timestamp for the last successful ptchan catalog poll.", nil, timestampSeconds(m.ptchanLastSuccessfulPoll))
	writeCounter(&builder, "martie_ptchan_notifications_sent_total", "Ptchan Telegram notifications sent by this martie process.", nil, float64(m.ptchanNotificationsSent))
	writeGaugeMap(&builder, "martie_ptchan_catalog_threads_by_board", "Threads in the most recent ptchan catalog poll by board.", "board", m.ptchanThreadsByBoard)
	writeGaugeMap(&builder, "martie_ptchan_catalog_reply_posts_by_board", "Reply posts in the most recent ptchan catalog poll by board.", "board", m.ptchanReplyPostsByBoard)
	writeGaugeMap(&builder, "martie_ptchan_catalog_reply_files_by_board", "Reply files in the most recent ptchan catalog poll by board.", "board", m.ptchanReplyFilesByBoard)

	return builder.String()
}

func writeGaugeMap(builder *strings.Builder, name, help, label string, values map[string]int) {
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
