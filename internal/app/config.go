package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"martie/internal/ptchan"
)

type Config struct {
	Telegram TelegramConfig
	Catalog  CatalogConfig
	Runtime  RuntimeConfig
	Storage  StorageConfig
}

type TelegramConfig struct {
	BotToken string
	ChatID   int64
}

type CatalogConfig struct {
	BaseURL       string
	MinReplyPosts int
	Filter        ptchan.Filter
	PruneAfter    time.Duration
}

type RuntimeConfig struct {
	MetricsAddr  string
	PollInterval time.Duration
}

type StorageConfig struct {
	SQLitePath string
}

func LoadConfig() (Config, error) {
	cfg := Config{
		Catalog: CatalogConfig{
			BaseURL: envOr("PTCHAN_BASE_URL", "https://ptchan.org"),
		},
		Storage: StorageConfig{
			SQLitePath: envOr("SQLITE_PATH", "data/bot.db"),
		},
	}

	cfg.Telegram.BotToken = strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	cfg.Runtime.MetricsAddr = strings.TrimSpace(os.Getenv("METRICS_ADDR"))

	if raw := strings.TrimSpace(os.Getenv("TELEGRAM_CHAT_ID")); raw != "" {
		chatID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("parse TELEGRAM_CHAT_ID: %w", err)
		}
		cfg.Telegram.ChatID = chatID
	}

	pollSeconds, err := strconv.Atoi(envOr("POLL_INTERVAL_SECONDS", "60"))
	if err != nil || pollSeconds <= 0 {
		return Config{}, fmt.Errorf("POLL_INTERVAL_SECONDS must be a positive integer")
	}
	cfg.Runtime.PollInterval = time.Duration(pollSeconds) * time.Second

	minReplyPosts, err := strconv.Atoi(envOr("MIN_REPLY_POSTS", "0"))
	if err != nil || minReplyPosts < 0 {
		return Config{}, fmt.Errorf("MIN_REPLY_POSTS must be a non-negative integer")
	}
	cfg.Catalog.MinReplyPosts = minReplyPosts

	cfg.Catalog.Filter.BoardDenylist = splitCommaList(os.Getenv("BOARD_DENYLIST"))
	cfg.Catalog.Filter.KeywordDenylist = splitCommaList(os.Getenv("KEYWORD_DENYLIST"))

	maxThreadAgeHours, err := strconv.Atoi(envOr("MAX_THREAD_AGE_HOURS", "0"))
	if err != nil || maxThreadAgeHours < 0 {
		return Config{}, fmt.Errorf("MAX_THREAD_AGE_HOURS must be a non-negative integer")
	}
	cfg.Catalog.Filter.MaxThreadAge = time.Duration(maxThreadAgeHours) * time.Hour

	pruneAfterHours, err := strconv.Atoi(envOr("PRUNE_AFTER_HOURS", "720"))
	if err != nil || pruneAfterHours < 0 {
		return Config{}, fmt.Errorf("PRUNE_AFTER_HOURS must be a non-negative integer")
	}
	cfg.Catalog.PruneAfter = time.Duration(pruneAfterHours) * time.Hour

	cfg.Storage.SQLitePath = filepath.Clean(cfg.Storage.SQLitePath)
	cfg.Catalog.BaseURL = strings.TrimRight(cfg.Catalog.BaseURL, "/")

	return cfg, nil
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
