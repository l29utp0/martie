package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"martie/internal/ptchan"
	"martie/internal/state"
	"martie/internal/telegram"
)

type bot struct {
	cfg      Config
	store    *state.Store
	ptchan   *ptchan.Client
	telegram *telegram.Client
	logger   *log.Logger
}

func Run(
	ctx context.Context,
	cfg Config,
	store *state.Store,
	ptchan *ptchan.Client,
	telegram *telegram.Client,
	logger *log.Logger,
) error {
	return bot{
		cfg:      cfg,
		store:    store,
		ptchan:   ptchan,
		telegram: telegram,
		logger:   logger,
	}.run(ctx)
}

func (s bot) run(ctx context.Context) error {
	if err := s.poll(ctx); err != nil {
		s.logger.Printf("initial sync failed: %v", err)
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
	if err := s.syncOnce(ctx); err != nil {
		return err
	}

	if err := s.prune(ctx); err != nil {
		return err
	}

	return nil
}

func (s bot) syncOnce(ctx context.Context) error {
	catalog, err := s.ptchan.FetchCatalog(ctx)
	if err != nil {
		return fmt.Errorf("fetch catalog: %w", err)
	}

	now := time.Now().UTC()
	newThreads := 0

	for _, thread := range catalog.Threads {
		if !threadAllowed(s.cfg, thread, now) {
			continue
		}

		record, _, err := s.store.GetThread(ctx, thread.ID)
		if err != nil {
			return fmt.Errorf("load thread %s: %w", thread.ID, err)
		}

		notifiedAt := record.NotifiedNewAt
		record = recordFromThread(thread, now)
		record.NotifiedNewAt = notifiedAt

		if err := s.store.UpsertThread(ctx, record); err != nil {
			return fmt.Errorf("store thread %s: %w", thread.ID, err)
		}

		if record.NotifiedNewAt != nil {
			continue
		}
		if thread.ReplyPosts < s.cfg.MinReplyPosts {
			continue
		}

		message := telegram.FormatNotification(s.cfg.PtchanBaseURL, thread, s.cfg.MinReplyPosts, now)
		if err := s.telegram.SendMessage(ctx, s.cfg.TelegramChatID, message); err != nil {
			return fmt.Errorf("send telegram message for %s: %w", thread.ID, err)
		}

		record.NotifiedNewAt = &now
		if err := s.store.UpsertThread(ctx, record); err != nil {
			s.logger.Printf("warning: thread %s was sent but could not be marked notified: %v", thread.ID, err)
		}

		newThreads++
	}

	s.logger.Printf("sync complete: %d threads seen, %d new notifications", len(catalog.Threads), newThreads)
	return nil
}
