package app

import (
	"context"
	"fmt"
	"log"
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
		logger:   logger,
	}.run(ctx)
}

func (s bot) run(ctx context.Context) error {
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

func (s bot) poll(ctx context.Context) error {
	if err := s.syncPtchan(ctx); err != nil {
		return err
	}

	if err := s.syncMiau(ctx); err != nil {
		return err
	}

	if err := s.prune(ctx); err != nil {
		return err
	}

	return nil
}
