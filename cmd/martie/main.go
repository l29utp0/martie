package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"martie/internal/app"
	"martie/internal/ptchan"
	"martie/internal/state"
	"martie/internal/telegram"
)

func main() {
	command, err := parseCommand(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := app.LoadConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if command == "run" {
		if cfg.TelegramBotToken == "" {
			log.Fatalf("load config: TELEGRAM_BOT_TOKEN is required")
		}
		if cfg.TelegramChatID == 0 {
			log.Fatalf("load config: TELEGRAM_CHAT_ID is required")
		}
	}

	store, err := state.Open(cfg.SQLitePath)
	if err != nil {
		log.Fatalf("open sqlite: %v", err)
	}
	defer store.Close()

	ptchanClient := ptchan.NewClient(cfg.PtchanBaseURL)
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	switch command {
	case "run":
		if err := app.Run(ctx, cfg, store, ptchanClient, telegram.NewClient(cfg.TelegramBotToken), logger); err != nil {
			log.Fatalf("run service: %v", err)
		}
	case "snapshot":
		if err := app.Snapshot(ctx, cfg, store, ptchanClient, logger); err != nil {
			log.Fatalf("snapshot store: %v", err)
		}
	default:
		log.Fatalf("unsupported command: %s", command)
	}
}

func parseCommand(args []string) (string, error) {
	if len(args) == 0 {
		return "run", nil
	}

	switch args[0] {
	case "run", "snapshot":
		if len(args) > 1 {
			return "", fmt.Errorf("usage: martie [run|snapshot]")
		}
		return args[0], nil
	default:
		return "", fmt.Errorf("usage: martie [run|snapshot]")
	}
}
