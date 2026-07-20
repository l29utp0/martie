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

func TestClientFetchThread(t *testing.T) {
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
						"_id": "thread-1",
						"date": "2026-04-14T10:00:00Z",
						"board": "i",
						"name": "Anónimo",
						"nomarkup": "op",
						"postId": 42,
						"replies": [{
							"date": "2026-04-14T10:01:00Z",
							"board": "i",
							"name": "Anon",
							"nomarkup": ">>42\r\nreply",
							"thread": 42,
							"postId": 43,
							"quotes": [{"thread": 42, "postId": 42}]
						}]
					}`)),
				}, nil
			}),
		},
	}

	thread, err := client.FetchThread(context.Background(), "i", 42)
	if err != nil {
		t.Fatalf("FetchThread() error = %v", err)
	}
	if gotRequest == nil {
		t.Fatal("request was not sent")
	}
	if gotRequest.URL.Path != "/i/thread/42.json" {
		t.Fatalf("path = %q, want /i/thread/42.json", gotRequest.URL.Path)
	}
	if thread.Board != "i" || thread.PostID != 42 || thread.Message != "op" {
		t.Fatalf("thread = %+v", thread)
	}
	if len(thread.Replies) != 1 || thread.Replies[0].PostID != 43 || len(thread.Replies[0].Quotes) != 1 {
		t.Fatalf("replies = %+v", thread.Replies)
	}
}

func TestClientFetchThreadRejectsOversizedResponse(t *testing.T) {
	client := &Client{
		baseURL: "https://ptchan.org",
		http: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				body := `{"board":"i","postId":42,"nomarkup":"` + strings.Repeat("x", maxThreadResponseBytes) + `"}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			}),
		},
	}

	_, err := client.FetchThread(context.Background(), "i", 42)
	if err == nil || !strings.Contains(err.Error(), "response exceeds") {
		t.Fatalf("FetchThread() error = %v, want oversized response error", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
