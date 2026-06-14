package telegram

import (
	"fmt"
	"html"
	"strings"
	"time"

	"martie/internal/ptchan"
)

func FormatNotification(baseURL string, thread ptchan.Thread, minReplyPosts int, now time.Time) string {
	var parts []string

	parts = append(parts, "<b>"+html.EscapeString(fmt.Sprintf("/%s/ #%d", thread.Board, thread.PostID))+"</b>")
	parts = append(parts, "<i>"+html.EscapeString(notificationSummary(thread, minReplyPosts, now))+"</i>")
	parts = append(parts, fmt.Sprintf("%s/%s/thread/%d.html", strings.TrimRight(baseURL, "/"), thread.Board, thread.PostID))

	return strings.Join(parts, "\n")
}

func FormatMiauNotification(pageURL string) string {
	return strings.Join([]string{
		"<b>🔴 Miau stream live</b>",
		html.EscapeString(pageURL),
	}, "\n")
}

func notificationSummary(thread ptchan.Thread, minReplyPosts int, now time.Time) string {
	parts := []string{pluralize(thread.ReplyPosts, "reply")}
	if thread.ReplyFiles > 0 {
		parts = append(parts, pluralize(thread.ReplyFiles, "file"))
	}
	if label := thresholdReachedLabel(thread, minReplyPosts, now); label != "" {
		parts = append(parts, label)
	}
	return strings.Join(parts, ", ")
}

func thresholdReachedLabel(thread ptchan.Thread, minReplyPosts int, now time.Time) string {
	if minReplyPosts <= 1 || thread.Date.IsZero() || thread.ReplyPosts < minReplyPosts {
		return ""
	}

	elapsed := now.Sub(thread.Date)
	if elapsed < 0 {
		return ""
	}

	return fmt.Sprintf("hit %d in %s", minReplyPosts, humanizeElapsed(elapsed))
}

func pluralize(count int, noun string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	if strings.HasSuffix(noun, "y") {
		return fmt.Sprintf("%d %sies", count, strings.TrimSuffix(noun, "y"))
	}
	return fmt.Sprintf("%d %ss", count, noun)
}

func humanizeElapsed(d time.Duration) string {
	if d < time.Hour {
		minutes := int(d.Round(time.Minute) / time.Minute)
		if minutes < 1 {
			minutes = 1
		}
		return fmt.Sprintf("%dm", minutes)
	}

	if d < 48*time.Hour {
		hours := int(d.Round(time.Hour) / time.Hour)
		if hours < 1 {
			hours = 1
		}
		return fmt.Sprintf("%dh", hours)
	}

	days := int(d.Round(24*time.Hour) / (24 * time.Hour))
	if days < 1 {
		days = 1
	}
	return fmt.Sprintf("%dd", days)
}
