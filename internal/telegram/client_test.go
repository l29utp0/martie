package telegram

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestClientSend(t *testing.T) {
	var gotRequest *http.Request

	client := &Client{
		baseURL: "https://api.telegram.org/bottoken",
		http: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				gotRequest = req

				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":1}}`)),
				}, nil
			}),
		},
	}

	err := client.Send(context.Background(), 12345, HTMLMessage("<b>Hello</b>"))
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if gotRequest == nil {
		t.Fatal("request was not sent")
	}

	if gotRequest.URL.Path != "/bottoken/sendMessage" {
		t.Fatalf("path = %q, want %q", gotRequest.URL.Path, "/bottoken/sendMessage")
	}
	if got := gotRequest.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/x-www-form-urlencoded")
	}

	body, err := io.ReadAll(gotRequest.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	form, err := url.ParseQuery(string(body))
	if err != nil {
		t.Fatalf("parse form body: %v", err)
	}

	if got := form.Get("chat_id"); got != "12345" {
		t.Fatalf("chat_id = %q, want %q", got, "12345")
	}
	if got := form.Get("text"); got != "<b>Hello</b>" {
		t.Fatalf("text = %q, want %q", got, "<b>Hello</b>")
	}
	if got := form.Get("parse_mode"); got != "HTML" {
		t.Fatalf("parse_mode = %q, want %q", got, "HTML")
	}
	if got := form.Get("disable_web_page_preview"); got != "" {
		t.Fatalf("disable_web_page_preview = %q, want empty", got)
	}
}

func TestClientSendAPIFailure(t *testing.T) {
	client := &Client{
		baseURL: "https://api.telegram.org/bottoken",
		http: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Status:     "200 OK",
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"ok":false,"description":"chat not found"}`)),
				}, nil
			}),
		},
	}

	err := client.Send(context.Background(), 12345, TextMessage("hello"))
	if err == nil {
		t.Fatal("Send() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), `"description":"chat not found"`) {
		t.Fatalf("Send() error = %q, want to contain %q", err, `"description":"chat not found"`)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
