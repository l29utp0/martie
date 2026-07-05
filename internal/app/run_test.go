package app

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestPollingComponentWaitsAfterPollCompletes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	interval := 30 * time.Millisecond
	var calls []time.Time
	component := pollingComponent("test", interval, func(context.Context) error {
		calls = append(calls, time.Now())
		if len(calls) == 2 {
			cancel()
		}
		return nil
	}, newMetrics(), discardLogger())

	if err := component.run(ctx); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("poll calls = %d, want 2", len(calls))
	}
	if elapsed := calls[1].Sub(calls[0]); elapsed < interval {
		t.Fatalf("time between polls = %s, want at least %s", elapsed, interval)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
