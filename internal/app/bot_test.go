package app

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"martie/internal/ptchan"
	"martie/internal/state"
)

func TestPollPreservesNotifiedThreadOutsidePruneWindow(t *testing.T) {
	t.Parallel()

	store, err := state.Open(filepath.Join(t.TempDir(), "bot.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	lastSeenAt := time.Now().UTC().Add(-48 * time.Hour)
	notifiedAt := lastSeenAt.Add(-time.Hour)
	if err := store.UpsertThread(context.Background(), state.ThreadRecord{
		ThreadID:      "thread-1",
		Board:         "g",
		PostID:        42,
		LastBumpedAt:  lastSeenAt,
		LastSeenAt:    lastSeenAt,
		NotifiedNewAt: &notifiedAt,
	}); err != nil {
		t.Fatalf("seed record: %v", err)
	}

	b := bot{
		cfg: Config{
			PtchanBaseURL: "https://ptchan.example",
			MinReplyPosts: 100,
			PruneAfter:    24 * time.Hour,
		},
		store:  store,
		ptchan: newTestPtchanClient(t),
		logger: log.New(os.Stdout, "", 0),
	}

	if err := b.poll(context.Background()); err != nil {
		t.Fatalf("poll() error = %v", err)
	}

	record, found, err := store.GetThread(context.Background(), "thread-1")
	if err != nil {
		t.Fatalf("GetThread() error = %v", err)
	}
	if !found {
		t.Fatal("GetThread() found = false, want true")
	}
	if record.NotifiedNewAt == nil {
		t.Fatal("NotifiedNewAt = nil, want preserved timestamp")
	}
	if !record.NotifiedNewAt.Equal(notifiedAt) {
		t.Fatalf("NotifiedNewAt = %v, want %v", record.NotifiedNewAt, notifiedAt)
	}
	if !record.LastSeenAt.After(lastSeenAt) {
		t.Fatalf("LastSeenAt = %v, want later than %v", record.LastSeenAt, lastSeenAt)
	}
}

func newTestPtchanClient(t *testing.T) *ptchan.Client {
	t.Helper()

	type clientLayout struct {
		baseURL string
		http    *http.Client
	}

	return (*ptchan.Client)(unsafe.Pointer(&clientLayout{
		baseURL: "https://ptchan.example",
		http: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Path != "/catalog.json" {
					t.Fatalf("path = %q, want %q", req.URL.Path, "/catalog.json")
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"threads": [{
							"_id": "thread-1",
							"date": "2026-04-13T10:00:00Z",
							"board": "g",
							"replyposts": 1,
							"bumped": "2026-04-14T10:00:00Z",
							"postId": 42
						}]
					}`)),
				}, nil
			}),
		},
	}))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
