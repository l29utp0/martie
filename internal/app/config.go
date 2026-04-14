package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	TelegramBotToken string
	TelegramChatID   int64
	PtchanBaseURL    string
	PollInterval     time.Duration
	SQLitePath       string
	MinReplyPosts    int
	BoardDenylist    []string
	KeywordDenylist  []string
	MaxThreadAge     time.Duration
	PruneAfter       time.Duration
}

func LoadConfig() (Config, error) {
	cfg := Config{
		PtchanBaseURL: envOr("PTCHAN_BASE_URL", "https://ptchan.org"),
		SQLitePath:    envOr("SQLITE_PATH", "data/bot.db"),
	}

	cfg.TelegramBotToken = strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))

	if raw := strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID")); raw != "" {
		chatID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("parse TELEGRAM_CHAT_ID: %w", err)
		}
		cfg.TelegramChatID = chatID
	}

	pollSeconds, err := strconv.Atoi(envOr("POLL_INTERVAL_SECONDS", "60"))
	if err != nil || pollSeconds <= 0 {
		return Config{}, fmt.Errorf("POLL_INTERVAL_SECONDS must be a positive integer")
	}
	cfg.PollInterval = time.Duration(pollSeconds) * time.Second

	minReplyPosts, err := strconv.Atoi(envOr("MIN_REPLY_POSTS", "0"))
	if err != nil || minReplyPosts < 0 {
		return Config{}, fmt.Errorf("MIN_REPLY_POSTS must be a non-negative integer")
	}
	cfg.MinReplyPosts = minReplyPosts

	cfg.BoardDenylist = lowercaseAll(splitCommaList(os.Getenv("BOARD_DENYLIST")))
	cfg.KeywordDenylist = lowercaseAll(splitCommaList(os.Getenv("KEYWORD_DENYLIST")))

	maxThreadAgeHours, err := strconv.Atoi(envOr("MAX_THREAD_AGE_HOURS", "0"))
	if err != nil || maxThreadAgeHours < 0 {
		return Config{}, fmt.Errorf("MAX_THREAD_AGE_HOURS must be a non-negative integer")
	}
	cfg.MaxThreadAge = time.Duration(maxThreadAgeHours) * time.Hour

	pruneAfterHours, err := strconv.Atoi(envOr("PRUNE_AFTER_HOURS", "720"))
	if err != nil || pruneAfterHours < 0 {
		return Config{}, fmt.Errorf("PRUNE_AFTER_HOURS must be a non-negative integer")
	}
	cfg.PruneAfter = time.Duration(pruneAfterHours) * time.Hour

	cfg.SQLitePath = filepath.Clean(cfg.SQLitePath)
	cfg.PtchanBaseURL = strings.TrimRight(cfg.PtchanBaseURL, "/")

	return cfg, nil
}

func (cfg Config) ValidateRun() error {
	if cfg.TelegramBotToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.TelegramChatID == 0 {
		return fmt.Errorf("TELEGRAM_CHAT_ID is required")
	}

	return nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func splitCommaList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		items = append(items, item)
	}

	return items
}

func lowercaseAll(items []string) []string {
	for i, item := range items {
		items[i] = strings.ToLower(item)
	}

	return items
}
