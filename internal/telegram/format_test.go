package telegram

import (
	"strings"
	"testing"
	"time"

	"martie/internal/localization"
)

func TestFormatThreadNotificationLocales(t *testing.T) {
	now := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	thread := ThreadNotice{Board: "g", PostID: 42, Date: now.Add(-2 * time.Hour), ReplyPosts: 10, ReplyFiles: 1}

	tests := []struct {
		locale localization.Locale
		want   string
	}{
		{locale: localization.English, want: "10 replies, 1 file, hit 10 in 2h"},
		{locale: localization.PortuguesePortugal, want: "10 respostas, 1 ficheiro, chegou a 10 em 2h"},
	}
	for _, test := range tests {
		t.Run(string(test.locale), func(t *testing.T) {
			message := NewFormatter(localization.New(test.locale)).ThreadNotification("https://example.com", thread, 10, now)
			if !strings.Contains(message.text, test.want) {
				t.Fatalf("message = %q, want %q", message.text, test.want)
			}
		})
	}
}

func TestFormatMiauStreamNotificationLocales(t *testing.T) {
	english := NewFormatter(localization.New(localization.English)).MiauStreamNotification(MiauStreamNotice{PageURL: "https://example.com"})
	portuguese := NewFormatter(localization.New(localization.PortuguesePortugal)).MiauStreamNotification(MiauStreamNotice{PageURL: "https://example.com"})
	if !strings.Contains(english.text, "stream live") || !strings.Contains(portuguese.text, "em direto") {
		t.Fatalf("localized streams = (%q, %q)", english.text, portuguese.text)
	}
}

func TestTextMessageHasNoParseMode(t *testing.T) {
	message := TextMessage("Plain text.")
	if message.text != "Plain text." || message.parseMode != "" {
		t.Fatalf("TextMessage() = %+v", message)
	}
}

func TestMarkdownMessage(t *testing.T) {
	message := MarkdownMessage("*Bold* and _italic_ with [a link](https://example.com).")
	if message.text != "*Bold* and _italic_ with [a link](https://example.com)." || message.parseMode != "Markdown" {
		t.Fatalf("MarkdownMessage() = %+v", message)
	}
}
