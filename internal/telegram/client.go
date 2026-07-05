package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxResponseBytes = 1 << 20

type Client struct {
	baseURL string
	http    *http.Client
	logger  *slog.Logger
}

type APIError struct {
	Description string
	RetryAfter  int
}

func (e *APIError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("telegram api error: %s (retry after %ds)", e.Description, e.RetryAfter)
	}
	return "telegram api error: " + e.Description
}

func New(botToken string, logger *slog.Logger) *Client {
	return &Client{
		baseURL: fmt.Sprintf("https://api.telegram.org/bot%s", botToken),
		logger:  logger.With("component", "telegram"),
		http: &http.Client{
			Timeout: 40 * time.Second,
		},
	}
}

type SendRequest struct {
	ChatID           int64
	Message          OutgoingMessage
	ReplyToMessageID int64
	MessageThreadID  int64
}

func (c *Client) SendTyping(ctx context.Context, chatID, messageThreadID int64) error {
	form := url.Values{}
	form.Set("chat_id", fmt.Sprintf("%d", chatID))
	form.Set("action", "typing")
	if messageThreadID != 0 {
		form.Set("message_thread_id", fmt.Sprintf("%d", messageThreadID))
	}
	_, err := call[json.RawMessage](ctx, c, "sendChatAction", form)
	return err
}

// Send uses the Bot API sendMessage method:
// https://core.telegram.org/bots/api#sendmessage
//
// We rely on Telegram's default link preview behavior described here:
// https://core.telegram.org/bots/api#linkpreviewoptions
func (c *Client) Send(ctx context.Context, request SendRequest) error {
	form := url.Values{}
	form.Set("chat_id", fmt.Sprintf("%d", request.ChatID))
	form.Set("text", request.Message.text)
	if request.Message.parseMode != "" {
		form.Set("parse_mode", request.Message.parseMode)
	}
	if request.MessageThreadID != 0 {
		form.Set("message_thread_id", fmt.Sprintf("%d", request.MessageThreadID))
	}
	if request.ReplyToMessageID != 0 {
		replyParameters, err := json.Marshal(struct {
			MessageID int64 `json:"message_id"`
		}{MessageID: request.ReplyToMessageID})
		if err != nil {
			return fmt.Errorf("encode reply parameters: %w", err)
		}
		form.Set("reply_parameters", string(replyParameters))
	}

	_, err := call[json.RawMessage](ctx, c, "sendMessage", form)
	var apiErr *APIError
	if request.Message.parseMode != "" && errors.As(err, &apiErr) && strings.Contains(strings.ToLower(apiErr.Description), "can't parse entities") {
		if c.logger != nil {
			c.logger.Warn("telegram markdown rejected; retrying as plain text", "chat_id", request.ChatID, "error", err)
		}
		form.Del("parse_mode")
		_, err = call[json.RawMessage](ctx, c, "sendMessage", form)
	}
	return err
}

func call[T any](ctx context.Context, client *Client, method string, form url.Values) (T, error) {
	var zero T
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+"/"+method, strings.NewReader(form.Encode()))
	if err != nil {
		return zero, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.http.Do(req)
	if err != nil {
		var requestError *url.Error
		if errors.As(err, &requestError) {
			err = requestError.Err
		}
		return zero, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return zero, fmt.Errorf("read response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return zero, fmt.Errorf("telegram api response exceeds %d bytes", maxResponseBytes)
	}

	var result struct {
		OK          bool   `json:"ok"`
		Result      T      `json:"result"`
		Description string `json:"description"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		if resp.StatusCode != http.StatusOK {
			return zero, fmt.Errorf("telegram api unexpected status: %s: %s", resp.Status, strings.TrimSpace(string(body)))
		}
		return zero, fmt.Errorf("decode response: %w", err)
	}

	if !result.OK {
		return zero, &APIError{
			Description: result.Description,
			RetryAfter:  result.Parameters.RetryAfter,
		}
	}
	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("telegram api unexpected status: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	return result.Result, nil
}
