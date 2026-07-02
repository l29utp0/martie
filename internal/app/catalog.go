package app

import (
	"context"
	"fmt"
	"time"

	"martie/internal/telegram"
)

func (s catalogWatcher) poll(ctx context.Context) error {
	if err := s.sync(ctx); err != nil {
		return err
	}
	return s.prune(ctx)
}

func (s catalogWatcher) sync(ctx context.Context) error {
	catalog, err := s.client.FetchCatalog(ctx)
	if err != nil {
		return fmt.Errorf("fetch catalog: %w", err)
	}

	now := time.Now().UTC()
	s.metrics.observeCatalog(catalog, s.cfg, now)

	newThreads := 0

	for _, thread := range catalog.Threads {
		if !s.cfg.Filter.Allows(thread, now) {
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

		message := telegram.FormatThreadNotification(s.cfg.BaseURL, telegram.ThreadNotice{
			Board:      thread.Board,
			PostID:     thread.PostID,
			Date:       thread.Date,
			ReplyPosts: thread.ReplyPosts,
			ReplyFiles: thread.ReplyFiles,
		}, s.cfg.MinReplyPosts, now)
		if err := s.telegram.Send(ctx, s.chatID, message); err != nil {
			// TODO: Telegram may return retry_after, but this loop currently retries on
			// the next poll, so a short PollInterval may retry sooner than requested.
			return fmt.Errorf("send telegram message for %s: %w", thread.ID, err)
		}

		record.NotifiedNewAt = &now
		if err := s.store.UpsertThread(ctx, record); err != nil {
			s.logger.Printf("warning: thread %s was sent but could not be marked notified: %v", thread.ID, err)
		}

		newThreads++
	}

	s.metrics.addNotifications("catalog", newThreads)
	s.logger.Printf("catalog sync complete: %d threads seen, %d new notifications", len(catalog.Threads), newThreads)
	return nil
}
