package app

import (
	"context"
	"fmt"
	"time"

	"martie/internal/telegram"
)

func (s catalogPoller) poll(ctx context.Context) error {
	if err := s.sync(ctx); err != nil {
		return err
	}
	return s.prune(ctx)
}

func (s catalogPoller) sync(ctx context.Context) error {
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

		message := s.format.ThreadNotification(s.cfg.BaseURL, telegram.ThreadNotice{
			Board:      thread.Board,
			PostID:     thread.PostID,
			Date:       thread.Date,
			ReplyPosts: thread.ReplyPosts,
			ReplyFiles: thread.ReplyFiles,
		}, s.cfg.MinReplyPosts, now)
		if err := s.telegram.Send(ctx, telegram.SendRequest{ChatID: s.chatID, Message: message}); err != nil {
			return fmt.Errorf("send telegram message for %s: %w", thread.ID, err)
		}

		record.NotifiedNewAt = &now
		if err := s.store.UpsertThread(ctx, record); err != nil {
			s.logger.Warn("notification sent but thread could not be marked notified", "thread", thread.ID, "error", err)
		}

		newThreads++
	}

	s.metrics.addNotifications(string(componentCatalog), newThreads)
	s.logger.Debug("catalog sync complete", "threads", len(catalog.Threads), "notifications", newThreads)
	return nil
}
