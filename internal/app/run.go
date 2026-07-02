package app

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"martie/internal/miau"
	"martie/internal/ptchan"
	"martie/internal/state"
	"martie/internal/telegram"
)

type catalogWatcher struct {
	cfg      CatalogConfig
	chatID   int64
	store    *state.Store
	client   *ptchan.Client
	telegram *telegram.Client
	metrics  *metrics
	logger   *log.Logger
}

type streamWatcher struct {
	channels []miau.Channel
	chatID   int64
	store    *state.Store
	client   *miau.Client
	telegram *telegram.Client
	metrics  *metrics
	logger   *log.Logger
}

func Run(
	ctx context.Context,
	cfg Config,
	store *state.Store,
	streamClient *miau.Client,
	catalogClient *ptchan.Client,
	telegramClient *telegram.Client,
	logger *log.Logger,
) error {
	metrics := newMetrics()
	server, err := startMetricsServer(cfg.Runtime.MetricsAddr, metrics, logger)
	if err != nil {
		return err
	}
	defer shutdownMetricsServer(server, logger)

	catalog := catalogWatcher{
		cfg:      cfg.Catalog,
		chatID:   cfg.Telegram.ChatID,
		store:    store,
		client:   catalogClient,
		telegram: telegramClient,
		metrics:  metrics,
		logger:   logger,
	}
	streams := streamWatcher{
		channels: miau.DefaultChannels(),
		chatID:   cfg.Telegram.ChatID,
		store:    store,
		client:   streamClient,
		telegram: telegramClient,
		metrics:  metrics,
		logger:   logger,
	}

	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		runWorkflow(ctx, cfg.Runtime.PollInterval, "catalog", catalog.poll, metrics, logger)
	}()
	go func() {
		defer workers.Done()
		runWorkflow(ctx, cfg.Runtime.PollInterval, "streams", streams.poll, metrics, logger)
	}()

	<-ctx.Done()
	workers.Wait()
	logger.Printf("shutdown requested")
	return nil
}

func runWorkflow(ctx context.Context, interval time.Duration, name string, run func(context.Context) error, metrics *metrics, logger *log.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		startedAt := time.Now()
		err := run(ctx)
		metrics.observeWorkflow(name, time.Since(startedAt), err)
		if err != nil && ctx.Err() == nil {
			logger.Printf("%s run failed: %v", name, err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func startMetricsServer(addr string, metrics *metrics, logger *log.Logger) (*http.Server, error) {
	if addr == "" {
		return nil, nil
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen for metrics: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.handler())

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Printf("metrics server failed: %v", err)
		}
	}()

	logger.Printf("metrics listening on %s", listener.Addr())
	return server, nil
}

func shutdownMetricsServer(server *http.Server, logger *log.Logger) {
	if server == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Printf("metrics server shutdown failed: %v", err)
	}
}
