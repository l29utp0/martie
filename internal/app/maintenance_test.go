package app

import (
	"context"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"martie/internal/ptchan"
	"martie/internal/state"
)

func TestSnapshotLeavesBelowThresholdThreadUnnotified(t *testing.T) {
	t.Parallel()

	store, err := state.Open(filepath.Join(t.TempDir(), "bot.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	b := bot{
		cfg: Config{
			MinReplyPosts: 10,
		},
		store: store,
		ptchan: newSnapshotTestPtchanClient(t, `{
			"threads": [{
				"_id": "thread-1",
				"date": "2026-04-16T10:00:00Z",
				"board": "cyb",
				"replyposts": 3,
				"bumped": "2026-04-16T11:00:00Z",
				"postId": 16647
			}]
		}`),
		logger: log.New(io.Discard, "", 0),
	}

	if err := b.snapshot(context.Background()); err != nil {
		t.Fatalf("snapshot() error = %v", err)
	}

	record, found, err := store.GetThread(context.Background(), "thread-1")
	if err != nil {
		t.Fatalf("GetThread() error = %v", err)
	}
	if !found {
		t.Fatal("GetThread() found = false, want true")
	}
	if record.NotifiedNewAt != nil {
		t.Fatalf("NotifiedNewAt = %v, want nil", record.NotifiedNewAt)
	}
}

func TestSnapshotMarksEligibleThreadHandled(t *testing.T) {
	t.Parallel()

	store, err := state.Open(filepath.Join(t.TempDir(), "bot.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	b := bot{
		cfg: Config{
			MinReplyPosts: 10,
		},
		store: store,
		ptchan: newSnapshotTestPtchanClient(t, `{
			"threads": [{
				"_id": "thread-1",
				"date": "2026-04-16T10:00:00Z",
				"board": "cyb",
				"replyposts": 10,
				"bumped": "2026-04-16T11:00:00Z",
				"postId": 16647
			}]
		}`),
		logger: log.New(io.Discard, "", 0),
	}

	if err := b.snapshot(context.Background()); err != nil {
		t.Fatalf("snapshot() error = %v", err)
	}

	record, found, err := store.GetThread(context.Background(), "thread-1")
	if err != nil {
		t.Fatalf("GetThread() error = %v", err)
	}
	if !found {
		t.Fatal("GetThread() found = false, want true")
	}
	if record.NotifiedNewAt == nil {
		t.Fatal("NotifiedNewAt = nil, want timestamp")
	}
}

func newSnapshotTestPtchanClient(t *testing.T, body string) *ptchan.Client {
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
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			}),
		},
	}))
}
