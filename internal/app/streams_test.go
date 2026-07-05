package app

import (
	"context"
	"errors"
	"testing"

	"martie/internal/localization"
	"martie/internal/miau"
	"martie/internal/state"
	"martie/internal/telegram"
)

func TestStreamPollContinuesAfterChannelFailure(t *testing.T) {
	client := &fakeStreamClient{fail: "first"}
	watcher := streamPoller{
		channels:         []miau.Channel{{Key: "first"}, {Key: "second"}},
		endMissThreshold: 2,
		client:           client,
		store:            &fakeStreamStore{},
		telegram:         &fakeMessageSender{},
	}

	err := watcher.poll(context.Background())
	if err == nil || !errors.Is(err, errStreamCheck) {
		t.Fatalf("poll() error = %v, want stream check error", err)
	}
	if len(client.checked) != 2 || client.checked[0] != "first" || client.checked[1] != "second" {
		t.Fatalf("checked channels = %v, want [first second]", client.checked)
	}
}

func TestStartedStreamIsMarkedNotifiedAfterSend(t *testing.T) {
	store := &fakeStreamStore{}
	sender := &fakeMessageSender{}
	watcher := streamPoller{
		chatID:           42,
		format:           telegram.NewFormatter(localization.New(localization.English)),
		endMissThreshold: 2,
		store:            store,
		telegram:         sender,
		metrics:          newMetrics(),
		logger:           discardLogger(),
	}
	channel := miau.Channel{Key: "live", PageURL: "https://example.com/live"}

	if err := watcher.handleStartedStream(context.Background(), channel, state.StreamState{}); err != nil {
		t.Fatalf("handleStartedStream() error = %v", err)
	}
	if len(sender.requests) != 1 {
		t.Fatalf("sent messages = %d, want 1", len(sender.requests))
	}
	if !store.state.Active || !store.state.LiveNotified || store.state.Key != "miau:live" {
		t.Fatalf("stored state = %+v, want active and notified", store.state)
	}
}

func TestStoppedStreamRequiresConsecutiveMisses(t *testing.T) {
	store := &fakeStreamStore{}
	watcher := streamPoller{endMissThreshold: 2, store: store, logger: discardLogger()}
	channel := miau.Channel{Key: "live"}
	stream := state.StreamState{Key: "miau:live", Active: true, LiveNotified: true}

	if err := watcher.handleStoppedStream(context.Background(), channel, stream); err != nil {
		t.Fatalf("first handleStoppedStream() error = %v", err)
	}
	if !store.state.Active || store.state.Consecutive404s != 1 {
		t.Fatalf("state after first miss = %+v, want active with one miss", store.state)
	}
	if err := watcher.handleStoppedStream(context.Background(), channel, store.state); err != nil {
		t.Fatalf("second handleStoppedStream() error = %v", err)
	}
	if store.state.Active || store.state.LiveNotified || store.state.Consecutive404s != 0 {
		t.Fatalf("state after second miss = %+v, want reset offline state", store.state)
	}
}

var errStreamCheck = errors.New("stream check failed")

type fakeStreamClient struct {
	fail    string
	checked []string
}

func (f *fakeStreamClient) IsLive(_ context.Context, channel miau.Channel) (bool, error) {
	f.checked = append(f.checked, channel.Key)
	if channel.Key == f.fail {
		return false, errStreamCheck
	}
	return false, nil
}

type fakeStreamStore struct {
	state state.StreamState
}

func (*fakeStreamStore) GetStreamState(context.Context, string) (state.StreamState, bool, error) {
	return state.StreamState{}, false, nil
}

func (f *fakeStreamStore) UpsertStreamState(_ context.Context, stream state.StreamState) error {
	f.state = stream
	return nil
}

type fakeMessageSender struct {
	requests []telegram.SendRequest
}

func (f *fakeMessageSender) Send(_ context.Context, request telegram.SendRequest) error {
	f.requests = append(f.requests, request)
	return nil
}
