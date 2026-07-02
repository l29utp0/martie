package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func New(botToken string) *Client {
	return &Client{
		baseURL: fmt.Sprintf("https://api.telegram.org/bot%s", botToken),
		http: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

// Send uses the Bot API sendMessage method:
// https://core.telegram.org/bots/api#sendmessage
//
// We rely on Telegram's default link preview behavior described here:
// https://core.telegram.org/bots/api#linkpreviewoptions
func (c *Client) Send(ctx context.Context, chatID int64, message OutgoingMessage) error {
	form := url.Values{}
	form.Set("chat_id", fmt.Sprintf("%d", chatID))
	form.Set("text", message.text)
	if message.parseMode != "" {
		form.Set("parse_mode", message.parseMode)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sendMessage", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		var requestError *url.Error
		if errors.As(err, &requestError) {
			err = requestError.Err
		}
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("telegram api unexpected status: %s: %s", resp.Status, strings.TrimSpace(string(body)))
		}
		return fmt.Errorf("decode response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram api unexpected status: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if !result.OK {
		return fmt.Errorf("telegram api error: %s", strings.TrimSpace(string(body)))
	}

	return nil
}
