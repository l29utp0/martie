package app

import (
	"context"
	"fmt"

	"martie/internal/miau"
	"martie/internal/state"
	"martie/internal/telegram"
)

const miauEndMissThreshold = 2

func (s bot) syncMiau(ctx context.Context) error {
	for _, channel := range miau.Channels {
		started, err := s.miau.StreamStarted(ctx, channel)
		if err != nil {
			return fmt.Errorf("check miau stream %s: %w", channel.Key, err)
		}

		stream, _, err := s.store.GetStreamState(ctx, miauStateKey(channel.Key))
		if err != nil {
			return fmt.Errorf("load miau stream %s: %w", channel.Key, err)
		}

		if started {
			if err := s.handleStartedMiauStream(ctx, channel, stream); err != nil {
				return err
			}
			continue
		}

		if err := s.handleStoppedMiauStream(ctx, channel, stream); err != nil {
			return err
		}
	}

	return nil
}

func (s bot) handleStartedMiauStream(ctx context.Context, channel miau.Channel, stream state.StreamState) error {
	wasActive := stream.Active
	previousMisses := stream.Consecutive404s
	stream.Key = miauStateKey(channel.Key)
	stream.Active = true
	stream.Consecutive404s = 0

	if wasActive && stream.LiveNotified {
		if previousMisses == 0 {
			return nil
		}
		return s.storeMiauState(ctx, channel.Key, stream)
	}

	stream.LiveNotified = false
	if err := s.storeMiauState(ctx, channel.Key, stream); err != nil {
		return err
	}

	message := telegram.FormatMiauNotification(channel.PageURL)
	if err := s.telegram.SendMessage(ctx, s.cfg.TelegramChatID, message); err != nil {
		return fmt.Errorf("send miau telegram message for %s: %w", channel.Key, err)
	}

	stream.LiveNotified = true
	if err := s.store.UpsertStreamState(ctx, stream); err != nil {
		s.logger.Printf("warning: miau stream %s was sent but could not be marked notified: %v", channel.Key, err)
	}

	return nil
}

func (s bot) handleStoppedMiauStream(ctx context.Context, channel miau.Channel, stream state.StreamState) error {
	if !stream.Active {
		return nil
	}

	stream.Consecutive404s++
	if stream.Consecutive404s < miauEndMissThreshold {
		return s.storeMiauState(ctx, channel.Key, stream)
	}

	stream.Active = false
	stream.LiveNotified = false
	stream.Consecutive404s = 0
	return s.storeMiauState(ctx, channel.Key, stream)
}

func (s bot) storeMiauState(ctx context.Context, channelKey string, stream state.StreamState) error {
	if err := s.store.UpsertStreamState(ctx, stream); err != nil {
		return fmt.Errorf("store miau stream %s: %w", channelKey, err)
	}
	return nil
}

func miauStateKey(channelKey string) string {
	return "miau:" + channelKey
}
