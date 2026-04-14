package app

import (
	"slices"
	"strings"
	"time"

	"martie/internal/ptchan"
)

func threadAllowed(cfg Config, thread ptchan.Thread, now time.Time) bool {
	if boardDenied(cfg.BoardDenylist, thread.Board) {
		return false
	}

	if keywordDenied(cfg.KeywordDenylist, thread) {
		return false
	}

	if cfg.MaxThreadAge > 0 && !thread.Date.IsZero() && now.Sub(thread.Date) > cfg.MaxThreadAge {
		return false
	}

	return true
}

func boardDenied(denylist []string, board string) bool {
	return slices.Contains(denylist, strings.ToLower(board))
}

func keywordDenied(denylist []string, thread ptchan.Thread) bool {
	if len(denylist) == 0 {
		return false
	}

	text := strings.ToLower(strings.Join([]string{
		thread.Board,
		thread.Subject,
		thread.Message,
	}, "\n"))

	for _, keyword := range denylist {
		if strings.Contains(text, keyword) {
			return true
		}
	}

	return false
}
