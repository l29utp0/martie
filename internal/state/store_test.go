package state

import (
	"testing"
	"time"
)

func TestFormatTimeIsFixedWidth(t *testing.T) {
	got := formatTime(time.Date(2026, time.April, 14, 12, 34, 56, 120000000, time.UTC))
	want := "2026-04-14T12:34:56.120000000Z"

	if got != want {
		t.Fatalf("formatTime() = %q, want %q", got, want)
	}
}

func TestParseTimeAcceptsOldAndNewFormats(t *testing.T) {
	tests := []string{
		"2026-04-14T12:34:56.120000000Z",
		time.Date(2026, time.April, 14, 12, 34, 56, 120000000, time.UTC).Format(time.RFC3339Nano),
	}

	want := time.Date(2026, time.April, 14, 12, 34, 56, 120000000, time.UTC)

	for _, tc := range tests {
		got, err := parseTime(tc)
		if err != nil {
			t.Fatalf("parseTime(%q) error = %v", tc, err)
		}
		if !got.Equal(want) {
			t.Fatalf("parseTime(%q) = %v, want %v", tc, got, want)
		}
	}
}
