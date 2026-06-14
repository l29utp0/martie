package app

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"martie/internal/miau"
	"martie/internal/ptchan"
	"martie/internal/state"
	"martie/internal/telegram"
)

type bot struct {
	cfg      Config
	store    *state.Store
	miau     *miau.Client
	ptchan   *ptchan.Client
	telegram *telegram.Client
	metrics  *metrics
	logger   *log.Logger
}

func Run(
	ctx context.Context,
	cfg Config,
	store *state.Store,
	miau *miau.Client,
	ptchan *ptchan.Client,
	telegram *telegram.Client,
	logger *log.Logger,
) error {
	return bot{
		cfg:      cfg,
		store:    store,
		miau:     miau,
		ptchan:   ptchan,
		telegram: telegram,
		metrics:  newMetrics(),
		logger:   logger,
	}.run(ctx)
}

func (s bot) run(ctx context.Context) error {
	metricsServer, err := s.startMetricsServer()
	if err != nil {
		return err
	}
	defer shutdownMetricsServer(metricsServer, s.logger)

	if err := s.poll(ctx); err != nil {
		return fmt.Errorf("initial sync: %w", err)
	}

	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.logger.Printf("shutdown requested")
			return nil
		case <-ticker.C:
			if err := s.poll(ctx); err != nil {
				s.logger.Printf("sync failed: %v", err)
			}
		}
	}
}

func (s bot) poll(ctx context.Context) (err error) {
	startedAt := time.Now()
	defer func() {
		s.metrics.observePoll(time.Since(startedAt), err)
	}()

	if err = s.syncPtchan(ctx); err != nil {
		return err
	}

	if err = s.syncMiau(ctx); err != nil {
		return err
	}

	if err = s.prune(ctx); err != nil {
		return err
	}

	return nil
}

func (s bot) startMetricsServer() (*http.Server, error) {
	if s.cfg.MetricsAddr == "" {
		return nil, nil
	}

	listener, err := net.Listen("tcp", s.cfg.MetricsAddr)
	if err != nil {
		return nil, fmt.Errorf("listen for metrics: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", s.handleMetrics)

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.Printf("metrics server failed: %v", err)
		}
	}()

	s.logger.Printf("metrics listening on %s", listener.Addr())
	return server, nil
}

func (s bot) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(s.metrics.render()))
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
