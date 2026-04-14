package ptchan

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
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
	Subject    string    `json:"subject"`
	Message    string    `json:"nomarkup"`
	ReplyPosts int       `json:"replyposts"`
	ReplyFiles int       `json:"replyfiles"`
	Bumped     time.Time `json:"bumped"`
	PostID     int64     `json:"postId"`
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

func ThreadURL(baseURL, board string, postID int64) string {
	return fmt.Sprintf("%s/%s/thread/%d.html", strings.TrimRight(baseURL, "/"), board, postID)
}
