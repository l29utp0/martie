package telegram

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestClientSendMessage(t *testing.T) {
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

	err := client.SendMessage(context.Background(), 12345, "<b>Hello</b>")
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
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
}

func TestClientSendMessageAPIFailure(t *testing.T) {
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

	err := client.SendMessage(context.Background(), 12345, "hello")
	if err == nil {
		t.Fatal("SendMessage() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "chat not found") {
		t.Fatalf("SendMessage() error = %q, want to contain %q", err, "chat not found")
	}
}

func TestClientSendMessageStatusFailure(t *testing.T) {
	client := &Client{
		baseURL: "https://api.telegram.org/bottoken",
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

	err := client.SendMessage(context.Background(), 12345, "hello")
	if err == nil {
		t.Fatal("SendMessage() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Fatalf("SendMessage() error = %q, want to contain %q", err, "502")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
