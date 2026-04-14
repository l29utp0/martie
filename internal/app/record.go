package app

import (
	"time"

	"martie/internal/ptchan"
	"martie/internal/state"
)

func recordFromThread(thread ptchan.Thread, seenAt time.Time) state.ThreadRecord {
	return state.ThreadRecord{
		ThreadID:     thread.ID,
		Board:        thread.Board,
		PostID:       thread.PostID,
		LastBumpedAt: thread.Bumped,
		LastSeenAt:   seenAt,
	}
}
