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

	header := fmt.Sprintf("/%s/ #%d | %d replies, %d files", thread.Board, thread.PostID, thread.ReplyPosts, thread.ReplyFiles)
	if label := thresholdReachedLabel(thread, minReplyPosts, now); label != "" {
		header = fmt.Sprintf("%s | %s", header, label)
	}
	parts = append(parts, "<b>"+html.EscapeString(header)+"</b>")
	parts = append(parts, html.EscapeString(ptchan.ThreadURL(baseURL, thread.Board, thread.PostID)))

	if subject := compactWhitespace(thread.Subject); subject != "" {
		parts = append(parts, html.EscapeString(subject))
	}

	return strings.Join(parts, "\n")
}

func compactWhitespace(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

func thresholdReachedLabel(thread ptchan.Thread, minReplyPosts int, now time.Time) string {
	if minReplyPosts <= 0 || thread.Date.IsZero() || thread.ReplyPosts < minReplyPosts {
		return ""
	}

	elapsed := now.Sub(thread.Date)
	if elapsed < 0 {
		return ""
	}

	return "reached in " + humanizeElapsed(elapsed)
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
