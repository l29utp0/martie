package miau

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	http *http.Client
}

type Channel struct {
	Key      string
	ProbeURL string
	PageURL  string
}

var Channels = []Channel{
	{
		Key:      "oficial",
		ProbeURL: "https://stream-global.bfcdn.host/app/031304855496+miau/llhls.m3u8",
		PageURL:  "https://miau.gg/oficial",
	},
	{
		Key:      "l29utp0",
		ProbeURL: "https://stream-global.bfcdn.host/app/031304855496+l29utp0/llhls.m3u8",
		PageURL:  "https://miau.gg/l29utp0",
	},
}

func NewClient() *Client {
	return &Client{
		http: &http.Client{
			Timeout: 20 * time.Second,
		},
	}
}

func (c *Client) StreamStarted(ctx context.Context, channel Channel) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, channel.ProbeURL, nil)
	if err != nil {
		return false, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return false, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected status: %s", resp.Status)
	}
}
