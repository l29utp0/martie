package app

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"martie/internal/localization"
	"martie/internal/miau"
)

func TestLoadConfig(t *testing.T) {
	path := writeConfig(t, `
locale = "pt-PT"
name = "Marta"

[telegram]
notification_chat_id = 123
discussion_chat_id = -456
allowed_user_ids = [7, 8]

[assistant]
max_input_runes = 2000
log_memory = true
system_prompt = " {{name}} is {{name}}. "
chat_prompt = " Chat with {{name}}. "

[assistant.memory]
ttl = "15m"
history_exchanges = 6

[assistant.rate_limit]
window = "30m"
user_limit = 20
user_burst = 4
global_limit = 80
global_burst = 10

[deepseek]
model = "deepseek-test"
max_tokens = 300
timeout = "45s"

[catalog]
base_url = "https://example.com/"
poll_interval = "4m"
min_reply_posts = 5
board_denylist = ["a"]
keyword_denylist = ["b"]
max_thread_age = "24h"
prune_after = "168h"

[runtime]
components = ["catalog", "assistant"]
metrics_addr = ":9090"

[runtime.logging]
level = "debug"
format = "json"

[streams]
end_miss_threshold = 3
poll_interval = "2m"

[[streams.channel]]
key = "live"
probe_url = "https://stream.example.com/live"
page_url = "https://example.com/live"
`)
	t.Setenv("CONFIG_FILE", path)
	t.Setenv("TELEGRAM_BOT_TOKEN", " token ")
	t.Setenv("DEEPSEEK_API_KEY", " key ")
	t.Setenv("SQLITE_PATH", "data/test.db")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Assistant.Name != "Marta" || cfg.Assistant.SystemPrompt != "Marta is Marta." || cfg.Assistant.ChatPrompt != "Chat with Marta." {
		t.Fatalf("identity = (%q, %q, %q)", cfg.Assistant.Name, cfg.Assistant.SystemPrompt, cfg.Assistant.ChatPrompt)
	}
	if cfg.Locale != localization.PortuguesePortugal {
		t.Fatalf("locale = %q", cfg.Locale)
	}
	if cfg.Telegram.BotToken != "token" || cfg.DeepSeek.APIKey != "key" {
		t.Fatalf("secrets were not loaded from the environment")
	}
	if cfg.Telegram.NotificationChatID != 123 || cfg.Assistant.DiscussionChatID != -456 || cfg.Assistant.AllowAllUsers || len(cfg.Assistant.AllowedUserIDs) != 2 || cfg.Assistant.AllowedUserIDs[0] != 7 || cfg.Assistant.AllowedUserIDs[1] != 8 {
		t.Fatalf("telegram config = %+v, assistant = %+v", cfg.Telegram, cfg.Assistant)
	}
	if cfg.Assistant.RateLimitWindow != 30*time.Minute || cfg.Assistant.ConversationTTL != 15*time.Minute || cfg.DeepSeek.Timeout != 45*time.Second || cfg.Catalog.PollInterval != 4*time.Minute || cfg.Streams.PollInterval != 2*time.Minute {
		t.Fatalf("durations were not parsed: %+v", cfg)
	}
	if len(cfg.Runtime.Components) != 2 || !cfg.runs(componentCatalog) || !cfg.runs(componentAssistant) || cfg.runs(componentStreams) {
		t.Fatalf("components = %+v", cfg.Runtime.Components)
	}
	if cfg.Assistant.HistoryExchanges != 6 || cfg.Assistant.MaxInputRunes != 2000 || !cfg.Assistant.LogMemory || cfg.Assistant.UserRequestLimit != 20 || cfg.Assistant.UserRequestBurst != 4 || cfg.Assistant.GlobalRequestLimit != 80 || cfg.Assistant.GlobalRequestBurst != 10 {
		t.Fatalf("assistant config = %+v", cfg.Assistant)
	}
	if cfg.DeepSeek.Model != "deepseek-test" || cfg.DeepSeek.MaxTokens != 300 {
		t.Fatalf("deepseek config = %+v", cfg.DeepSeek)
	}
	if cfg.Catalog.BaseURL != "https://example.com" || cfg.Catalog.MinReplyPosts != 5 || cfg.Catalog.Filter.MaxThreadAge != 24*time.Hour || cfg.Catalog.PruneAfter != 168*time.Hour || len(cfg.Catalog.Filter.BoardDenylist) != 1 || cfg.Catalog.Filter.BoardDenylist[0] != "a" || len(cfg.Catalog.Filter.KeywordDenylist) != 1 || cfg.Catalog.Filter.KeywordDenylist[0] != "b" {
		t.Fatalf("catalog config = %+v", cfg.Catalog)
	}
	if cfg.Runtime.MetricsAddr != ":9090" || cfg.Storage.SQLitePath != "data/test.db" {
		t.Fatalf("runtime = %+v, storage = %+v", cfg.Runtime, cfg.Storage)
	}
	if cfg.Runtime.Logging.Level != slog.LevelDebug || cfg.Runtime.Logging.Format != LogJSON {
		t.Fatalf("logging = %+v", cfg.Runtime.Logging)
	}
	if cfg.Streams.EndMissThreshold != 3 || len(cfg.Streams.Channels) != 1 || cfg.Streams.Channels[0].Key != "live" || cfg.Streams.Channels[0].ProbeURL != "https://stream.example.com/live" || cfg.Streams.Channels[0].PageURL != "https://example.com/live" {
		t.Fatalf("streams = %+v", cfg.Streams)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	path := writeConfig(t, `
name = "Martie"

[telegram]
allow_all_users = true

[assistant]
system_prompt = "You are {{name}}."
`)
	t.Setenv("CONFIG_FILE", path)
	t.Setenv("SQLITE_PATH", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Locale != localization.English || !cfg.Assistant.AllowAllUsers || cfg.Assistant.MaxInputRunes != 4096 || cfg.Assistant.ConversationTTL != 10*time.Minute || cfg.Assistant.HistoryExchanges != 8 || cfg.Assistant.RateLimitWindow != time.Hour || cfg.Assistant.UserRequestLimit != 25 || cfg.Assistant.UserRequestBurst != 6 || cfg.Assistant.GlobalRequestLimit != 100 || cfg.Assistant.GlobalRequestBurst != 12 {
		t.Fatalf("assistant defaults were not applied: %+v", cfg.Assistant)
	}
	if cfg.DeepSeek.Model != "deepseek-v4-flash" || cfg.DeepSeek.MaxTokens != 500 || cfg.DeepSeek.Timeout != time.Minute || cfg.Catalog.BaseURL != "https://ptchan.org" || cfg.Catalog.PollInterval != time.Minute || cfg.Catalog.Filter.MaxThreadAge != 0 || cfg.Catalog.PruneAfter != 720*time.Hour || cfg.Streams.PollInterval != time.Minute || cfg.Runtime.Logging.Level != slog.LevelInfo || cfg.Runtime.Logging.Format != LogText || cfg.Streams.EndMissThreshold != 2 || cfg.Storage.SQLitePath != "data/bot.db" {
		t.Fatalf("defaults were not applied: %+v", cfg)
	}
	if len(cfg.Runtime.Components) != 0 {
		t.Fatalf("default components = %+v, want none", cfg.Runtime.Components)
	}
}

func TestLoadConfigRejectsUnknownKeys(t *testing.T) {
	path := writeConfig(t, `
name = "Martie"
surprise = true

[assistant]
system_prompt = "Hello."
`)
	t.Setenv("CONFIG_FILE", path)

	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "surprise") {
		t.Fatalf("unknown key error = %v", err)
	}
}

func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name        string
		old         string
		replacement string
		want        string
	}{
		{name: "unsupported locale", old: `locale = "en"`, replacement: `locale = "pt"`, want: "locale must be"},
		{name: "zero input limit", old: "max_input_runes = 4096", replacement: "max_input_runes = 0", want: "assistant.max_input_runes"},
		{name: "zero history limit", old: "history_exchanges = 8", replacement: "history_exchanges = 0", want: "assistant.memory.history_exchanges"},
		{name: "zero user limit", old: "user_limit = 25", replacement: "user_limit = 0", want: "assistant.rate_limit.user_burst"},
		{name: "zero user burst", old: "user_burst = 6", replacement: "user_burst = 0", want: "assistant.rate_limit.user_burst"},
		{name: "user burst above limit", old: "user_burst = 6", replacement: "user_burst = 26", want: "assistant.rate_limit.user_burst"},
		{name: "zero global limit", old: "global_limit = 100", replacement: "global_limit = 0", want: "assistant.rate_limit.global_burst"},
		{name: "zero global burst", old: "global_burst = 12", replacement: "global_burst = 0", want: "assistant.rate_limit.global_burst"},
		{name: "global burst above limit", old: "global_burst = 12", replacement: "global_burst = 101", want: "assistant.rate_limit.global_burst"},
		{name: "empty model", old: `model = "deepseek-v4-flash"`, replacement: `model = " "`, want: "deepseek.model"},
		{name: "zero max tokens", old: "max_tokens = 500", replacement: "max_tokens = 0", want: "deepseek.max_tokens"},
		{name: "empty catalog URL", old: `base_url = "https://ptchan.org"`, replacement: `base_url = " "`, want: "catalog.base_url"},
		{name: "negative reply posts", old: "min_reply_posts = 10", replacement: "min_reply_posts = -1", want: "catalog.min_reply_posts"},
		{name: "zero stream misses", old: "end_miss_threshold = 2", replacement: "end_miss_threshold = 0", want: "streams.end_miss_threshold"},
		{name: "empty stream key", old: `key = "oficial"`, replacement: `key = " "`, want: "requires key, probe_url, and page_url"},
		{name: "empty stream probe", old: `probe_url = "https://stream.example.com/live"`, replacement: `probe_url = " "`, want: "requires key, probe_url, and page_url"},
		{name: "empty stream page", old: `page_url = "https://example.com/live"`, replacement: `page_url = " "`, want: "requires key, probe_url, and page_url"},
		{name: "invalid memory TTL", old: `ttl = "10m"`, replacement: `ttl = "later"`, want: "assistant.memory.ttl"},
		{name: "zero memory TTL", old: `ttl = "10m"`, replacement: `ttl = "0s"`, want: "assistant.memory.ttl"},
		{name: "invalid rate window", old: `window = "60m"`, replacement: `window = "hourly"`, want: "assistant.rate_limit.window"},
		{name: "invalid provider timeout", old: `timeout = "60s"`, replacement: `timeout = "soon"`, want: "deepseek.timeout"},
		{name: "invalid catalog poll interval", old: `poll_interval = "60s"`, replacement: `poll_interval = "often"`, want: "catalog.poll_interval"},
		{name: "invalid stream poll interval", old: `poll_interval = "30s"`, replacement: `poll_interval = "often"`, want: "streams.poll_interval"},
		{name: "invalid log level", old: `level = "info"`, replacement: `level = "verbose"`, want: "runtime.logging.level"},
		{name: "invalid log format", old: `format = "text"`, replacement: `format = "xml"`, want: "runtime.logging.format"},
		{name: "negative maximum age", old: `max_thread_age = "0s"`, replacement: `max_thread_age = "-1s"`, want: "catalog.max_thread_age"},
		{name: "invalid maximum age", old: `max_thread_age = "0s"`, replacement: `max_thread_age = "old"`, want: "catalog.max_thread_age"},
		{name: "negative prune duration", old: `prune_after = "720h"`, replacement: `prune_after = "-1s"`, want: "catalog.prune_after"},
		{name: "invalid prune duration", old: `prune_after = "720h"`, replacement: `prune_after = "eventually"`, want: "catalog.prune_after"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contents := strings.Replace(validConfig, test.old, test.replacement, 1)
			if contents == validConfig {
				t.Fatalf("test replacement did not match %q", test.old)
			}
			t.Setenv("CONFIG_FILE", writeConfig(t, contents))

			if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("LoadConfig() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestLoadConfigRejectsMalformedDocuments(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		want     string
	}{
		{name: "invalid TOML", contents: `name = "unterminated`, want: "decode config"},
		{name: "duplicate key", contents: strings.Replace(validConfig, `name = "Martie"`, "name = \"Martie\"\nname = \"Marta\"", 1), want: "name"},
		{name: "wrong scalar type", contents: strings.Replace(validConfig, "max_input_runes = 4096", `max_input_runes = "many"`, 1), want: "MaxInputRunes"},
		{name: "unknown root key", contents: validConfig + "\nsurprise = true\n", want: "surprise"},
		{name: "unknown nested key", contents: strings.Replace(validConfig, "[deepseek]", "[deepseek]\nsurprise = true", 1), want: "surprise"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("CONFIG_FILE", writeConfig(t, test.contents))

			if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("LoadConfig() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestLoadConfigRequiresFile(t *testing.T) {
	t.Setenv("CONFIG_FILE", " \t ")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("missing CONFIG_FILE was accepted")
	}
}

func TestLoadConfigReportsMissingFile(t *testing.T) {
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing.toml"))

	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "open config") {
		t.Fatalf("missing file error = %v", err)
	}
}

func TestLoadConfigRejectsDuplicateStreamKeys(t *testing.T) {
	path := writeConfig(t, `
name = "Martie"

[assistant]
system_prompt = "Hello."

[[streams.channel]]
key = "same"
probe_url = "https://example.com/one"
page_url = "https://example.com/one"

[[streams.channel]]
key = "same"
probe_url = "https://example.com/two"
page_url = "https://example.com/two"
`)
	t.Setenv("CONFIG_FILE", path)

	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("duplicate stream error = %v", err)
	}
}

func TestLoadConfigRejectsInvalidComponents(t *testing.T) {
	tests := []struct {
		name       string
		components string
		want       string
	}{
		{name: "unknown", components: `["catalog", "search"]`, want: `unknown component "search"`},
		{name: "duplicate", components: `["catalog", "catalog"]`, want: `duplicate component "catalog"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contents := strings.Replace(validConfig, `["catalog", "streams", "assistant"]`, test.components, 1)
			t.Setenv("CONFIG_FILE", writeConfig(t, contents))
			if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("LoadConfig() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestValidateRunUsesSelectedComponentDependencies(t *testing.T) {
	base := Config{
		Telegram: TelegramConfig{BotToken: "token", NotificationChatID: 1},
		Assistant: AssistantConfig{
			Name:             "Martie",
			DiscussionChatID: 2,
			AllowAllUsers:    true,
			SystemPrompt:     "Be useful.",
			ChatPrompt:       "Keep group context separate.",
		},
		DeepSeek: DeepSeekConfig{APIKey: "key"},
		Streams:  StreamsConfig{Channels: []miau.Channel{{Key: "live", ProbeURL: "https://stream.example.com", PageURL: "https://example.com"}}},
	}

	tests := []struct {
		name       string
		components []ComponentName
		change     func(*Config)
		want       string
	}{
		{name: "requires a component", want: "at least one component"},
		{name: "catalog only", components: []ComponentName{componentCatalog}, change: func(cfg *Config) {
			cfg.Assistant = AssistantConfig{}
			cfg.DeepSeek.APIKey = ""
		}},
		{name: "streams only", components: []ComponentName{componentStreams}},
		{name: "assistant only", components: []ComponentName{componentAssistant}, change: func(cfg *Config) { cfg.Telegram.NotificationChatID = 0 }},
		{name: "all components require Telegram", components: []ComponentName{componentCatalog}, change: func(cfg *Config) { cfg.Telegram.BotToken = "" }, want: "TELEGRAM_BOT_TOKEN"},
		{name: "catalog requires notification chat", components: []ComponentName{componentCatalog}, change: func(cfg *Config) { cfg.Telegram.NotificationChatID = 0 }, want: "notification_chat_id"},
		{name: "streams require channels", components: []ComponentName{componentStreams}, change: func(cfg *Config) { cfg.Streams.Channels = nil }, want: "at least one channel"},
		{name: "assistant requires name", components: []ComponentName{componentAssistant}, change: func(cfg *Config) { cfg.Assistant.Name = "" }, want: "name is required"},
		{name: "assistant requires prompt", components: []ComponentName{componentAssistant}, change: func(cfg *Config) { cfg.Assistant.SystemPrompt = "" }, want: "system_prompt"},
		{name: "assistant requires chat prompt", components: []ComponentName{componentAssistant}, change: func(cfg *Config) { cfg.Assistant.ChatPrompt = "" }, want: "chat_prompt"},
		{name: "assistant requires discussion chat", components: []ComponentName{componentAssistant}, change: func(cfg *Config) { cfg.Assistant.DiscussionChatID = 0 }, want: "discussion_chat_id"},
		{name: "assistant requires access policy", components: []ComponentName{componentAssistant}, change: func(cfg *Config) { cfg.Assistant.AllowAllUsers = false }, want: "allowed_user_ids"},
		{name: "assistant requires api key", components: []ComponentName{componentAssistant}, change: func(cfg *Config) { cfg.DeepSeek.APIKey = "" }, want: "DEEPSEEK_API_KEY"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := base
			cfg.Runtime.Components = test.components
			if test.change != nil {
				test.change(&cfg)
			}
			err := cfg.ValidateRun()
			if test.want == "" && err != nil {
				t.Fatal(err)
			}
			if test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("ValidateRun() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestExampleConfigLoads(t *testing.T) {
	t.Setenv("CONFIG_FILE", filepath.Join("..", "..", "config.example.toml"))

	if _, err := LoadConfig(); err != nil {
		t.Fatal(err)
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const validConfig = `
locale = "en"
name = "Martie"

[assistant]
max_input_runes = 4096
system_prompt = "Hello {{name}}."
chat_prompt = "Keep group context separate."

[assistant.memory]
ttl = "10m"
history_exchanges = 8

[assistant.rate_limit]
window = "60m"
user_limit = 25
user_burst = 6
global_limit = 100
global_burst = 12

[deepseek]
model = "deepseek-v4-flash"
max_tokens = 500
timeout = "60s"

[catalog]
base_url = "https://ptchan.org"
poll_interval = "60s"
min_reply_posts = 10
max_thread_age = "0s"
prune_after = "720h"

[runtime]
components = ["catalog", "streams", "assistant"]

[runtime.logging]
level = "info"
format = "text"

[streams]
end_miss_threshold = 2
poll_interval = "30s"

[[streams.channel]]
key = "oficial"
probe_url = "https://stream.example.com/live"
page_url = "https://example.com/live"
`
