package ptchan

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxThreadResponseBytes = 4 << 20

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

type Catalog struct {
	Threads []Thread `json:"threads"`
}

type Thread struct {
	ID         string    `json:"_id"`
	Date       time.Time `json:"date"`
	Board      string    `json:"board"`
	Name       string    `json:"name"`
	Subject    string    `json:"subject"`
	Message    string    `json:"nomarkup"`
	ReplyPosts int       `json:"replyposts"`
	ReplyFiles int       `json:"replyfiles"`
	Bumped     time.Time `json:"bumped"`
	PostID     int64     `json:"postId"`
	Tripcode   string    `json:"tripcode"`
	Capcode    string    `json:"capcode"`
	Quotes     []Quote   `json:"quotes"`
	Replies    []Post    `json:"replies"`
}

type Post struct {
	Date     time.Time `json:"date"`
	Board    string    `json:"board"`
	Name     string    `json:"name"`
	Message  string    `json:"nomarkup"`
	ThreadID int64     `json:"thread"`
	PostID   int64     `json:"postId"`
	Tripcode string    `json:"tripcode"`
	Capcode  string    `json:"capcode"`
	Quotes   []Quote   `json:"quotes"`
}

type Quote struct {
	ThreadID int64 `json:"thread"`
	PostID   int64 `json:"postId"`
}

func (c *Client) FetchCatalog(ctx context.Context) (Catalog, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/catalog.json", nil)
	if err != nil {
		return Catalog{}, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Catalog{}, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Catalog{}, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var catalog Catalog
	if err := json.NewDecoder(resp.Body).Decode(&catalog); err != nil {
		return Catalog{}, fmt.Errorf("decode catalog: %w", err)
	}

	return catalog, nil
}

func (c *Client) FetchThread(ctx context.Context, board string, threadID int64) (Thread, error) {
	path := "/" + url.PathEscape(board) + "/thread/" + strconv.FormatInt(threadID, 10) + ".json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return Thread{}, fmt.Errorf("create thread request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Thread{}, fmt.Errorf("send thread request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Thread{}, fmt.Errorf("unexpected thread status: %s", resp.Status)
	}

	body := &io.LimitedReader{R: resp.Body, N: maxThreadResponseBytes + 1}
	var thread Thread
	if err := json.NewDecoder(body).Decode(&thread); err != nil {
		if body.N == 0 {
			return Thread{}, fmt.Errorf("thread response exceeds %d bytes", maxThreadResponseBytes)
		}
		return Thread{}, fmt.Errorf("decode thread: %w", err)
	}
	if body.N == 0 {
		return Thread{}, fmt.Errorf("thread response exceeds %d bytes", maxThreadResponseBytes)
	}
	return thread, nil
}
