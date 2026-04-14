package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"martie/internal/ptchan"
	"martie/internal/state"
)

func Seed(ctx context.Context, cfg Config, store *state.Store, ptchan *ptchan.Client, logger *log.Logger) error {
	return bot{
		cfg:    cfg,
		store:  store,
		ptchan: ptchan,
		logger: logger,
	}.seed(ctx)
}

func (s bot) seed(ctx context.Context) error {
	catalog, err := s.ptchan.FetchCatalog(ctx)
	if err != nil {
		return fmt.Errorf("fetch catalog for seed: %w", err)
	}

	now := time.Now().UTC()
	seeded := 0

	for _, thread := range catalog.Threads {
		if !threadAllowed(s.cfg, thread, now) {
			continue
		}

		record := recordFromThread(thread, now)
		// Seed establishes the current catalog as an already-handled baseline.
		record.NotifiedNewAt = &now
		if err := s.store.UpsertThread(ctx, record); err != nil {
			return fmt.Errorf("seed thread %s: %w", thread.ID, err)
		}
		seeded++
	}

	s.logger.Printf("seed complete: %d threads stored as already handled", seeded)
	return nil
}

func (s bot) prune(ctx context.Context) error {
	if s.cfg.PruneAfter == 0 {
		return nil
	}

	cutoff := time.Now().UTC().Add(-s.cfg.PruneAfter)
	pruned, err := s.store.PruneSeenBefore(ctx, cutoff)
	if err != nil {
		return err
	}
	if pruned > 0 {
		s.logger.Printf("pruned %d threads last seen before %s", pruned, cutoff.Format(time.RFC3339))
	}

	return nil
}
