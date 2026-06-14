package app

import (
	"context"
	"fmt"
	"time"

	"martie/internal/telegram"
)

func (s bot) syncPtchan(ctx context.Context) error {
	catalog, err := s.ptchan.FetchCatalog(ctx)
	if err != nil {
		return fmt.Errorf("fetch catalog: %w", err)
	}

	now := time.Now().UTC()
	s.metrics.observePtchanCatalog(catalog, s.cfg, now)

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

	s.metrics.addPtchanNotifications(newThreads)
	s.logger.Printf("ptchan sync complete: %d threads seen, %d new notifications", len(catalog.Threads), newThreads)
	return nil
}
