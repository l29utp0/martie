package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(botToken string) *Client {
	return &Client{
		baseURL: fmt.Sprintf("https://api.telegram.org/bot%s", botToken),
		http: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	form := url.Values{}
	form.Set("chat_id", fmt.Sprintf("%d", chatID))
	form.Set("text", text)
	form.Set("parse_mode", "HTML")
	form.Set("disable_web_page_preview", "false")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sendMessage", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if !result.OK {
		if result.Description == "" {
			return fmt.Errorf("telegram api returned ok=false")
		}
		return fmt.Errorf("telegram api: %s", result.Description)
	}

	return nil
}
