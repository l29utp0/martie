package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"martie/internal/ptchan"
	"martie/internal/state"
)

func Snapshot(ctx context.Context, cfg Config, store *state.Store, client *ptchan.Client, logger *log.Logger) error {
	return catalogWatcher{
		cfg:    cfg.Catalog,
		store:  store,
		client: client,
		logger: logger,
	}.snapshot(ctx)
}

func (s catalogWatcher) snapshot(ctx context.Context) error {
	catalog, err := s.client.FetchCatalog(ctx)
	if err != nil {
		return fmt.Errorf("fetch catalog for snapshot: %w", err)
	}

	now := time.Now().UTC()
	stored := 0
	handled := 0

	for _, thread := range catalog.Threads {
		if !s.cfg.Filter.Allows(thread, now) {
			continue
		}

		existing, _, err := s.store.GetThread(ctx, thread.ID)
		if err != nil {
			return fmt.Errorf("load thread %s for snapshot: %w", thread.ID, err)
		}

		record := recordFromThread(thread, now)
		record.NotifiedNewAt = existing.NotifiedNewAt
		if thread.ReplyPosts >= s.cfg.MinReplyPosts {
			// Snapshot marks already-eligible threads as handled without suppressing
			// existing low-reply threads that may cross the threshold later.
			record.NotifiedNewAt = &now
			handled++
		}
		if err := s.store.UpsertThread(ctx, record); err != nil {
			return fmt.Errorf("snapshot thread %s: %w", thread.ID, err)
		}
		stored++
	}

	s.logger.Printf("snapshot complete: %d threads stored, %d marked already handled", stored, handled)
	return nil
}

func (s catalogWatcher) prune(ctx context.Context) error {
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
