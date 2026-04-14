package ptchan

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestClientFetchCatalog(t *testing.T) {
	var gotRequest *http.Request

	client := &Client{
		baseURL: "https://ptchan.org",
		http: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				gotRequest = req

				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body: io.NopCloser(strings.NewReader(`{
						"threads": [{
							"_id": "thread-1",
							"date": "2026-04-14T10:00:00Z",
							"board": "g",
							"subject": "hello",
							"nomarkup": "world",
							"replyposts": 12,
							"replyfiles": 3,
							"bumped": "2026-04-14T11:00:00Z",
							"postId": 42
						}]
					}`)),
				}, nil
			}),
		},
	}

	catalog, err := client.FetchCatalog(context.Background())
	if err != nil {
		t.Fatalf("FetchCatalog() error = %v", err)
	}
	if gotRequest == nil {
		t.Fatal("request was not sent")
	}
	if gotRequest.URL.Path != "/catalog.json" {
		t.Fatalf("path = %q, want %q", gotRequest.URL.Path, "/catalog.json")
	}
	if len(catalog.Threads) != 1 {
		t.Fatalf("len(Threads) = %d, want 1", len(catalog.Threads))
	}

	thread := catalog.Threads[0]
	if thread.ID != "thread-1" {
		t.Fatalf("ID = %q, want %q", thread.ID, "thread-1")
	}
	if thread.Board != "g" {
		t.Fatalf("Board = %q, want %q", thread.Board, "g")
	}
	if thread.Message != "world" {
		t.Fatalf("Message = %q, want %q", thread.Message, "world")
	}
	if thread.PostID != 42 {
		t.Fatalf("PostID = %d, want 42", thread.PostID)
	}

	wantDate := time.Date(2026, time.April, 14, 10, 0, 0, 0, time.UTC)
	if !thread.Date.Equal(wantDate) {
		t.Fatalf("Date = %v, want %v", thread.Date, wantDate)
	}
}

func TestClientFetchCatalogStatusFailure(t *testing.T) {
	client := &Client{
		baseURL: "https://ptchan.org",
		http: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Status:     "502 Bad Gateway",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("bad gateway")),
				}, nil
			}),
		},
	}

	_, err := client.FetchCatalog(context.Background())
	if err == nil {
		t.Fatal("FetchCatalog() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Fatalf("FetchCatalog() error = %q, want to contain %q", err, "502")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
