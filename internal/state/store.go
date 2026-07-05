package state

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Fixed-width UTC timestamps keep SQLite text comparisons in chronological order.
const sqliteTimeLayout = "2006-01-02T15:04:05.000000000Z"

type Store struct {
	db *sql.DB
}

type ThreadRecord struct {
	ThreadID      string
	Board         string
	PostID        int64
	LastBumpedAt  time.Time
	LastSeenAt    time.Time
	NotifiedNewAt *time.Time
}

type StreamState struct {
	Key             string
	Active          bool
	LiveNotified    bool
	Consecutive404s int
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)

	store := &Store{db: db}
	if err := store.initSchema(context.Background()); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) PruneSeenBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	const statement = `DELETE FROM threads WHERE last_seen_at < ?;`

	result, err := s.db.ExecContext(ctx, statement, formatTime(cutoff))
	if err != nil {
		return 0, fmt.Errorf("prune threads: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count pruned threads: %w", err)
	}

	return rows, nil
}

func (s *Store) initSchema(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS threads (
  thread_id TEXT PRIMARY KEY,
  board TEXT NOT NULL,
  post_id INTEGER NOT NULL,
  last_bumped_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  notified_new_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_threads_post_id ON threads(post_id);
CREATE INDEX IF NOT EXISTS idx_threads_last_seen_at ON threads(last_seen_at);

CREATE TABLE IF NOT EXISTS stream_states (
  stream_key TEXT PRIMARY KEY,
  active INTEGER NOT NULL,
  live_notified INTEGER NOT NULL,
  consecutive_404s INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS cursors (
  name TEXT PRIMARY KEY,
  position INTEGER NOT NULL
);
`

	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("init schema: %w", err)
	}
	return nil
}

func (s *Store) GetThread(ctx context.Context, threadID string) (ThreadRecord, bool, error) {
	const query = `
SELECT thread_id, board, post_id, last_bumped_at, last_seen_at, notified_new_at
FROM threads
WHERE thread_id = ?;
`

	var record ThreadRecord
	var lastBumpedRaw string
	var lastSeenRaw string
	var notifiedRaw sql.NullString

	err := s.db.QueryRowContext(ctx, query, threadID).Scan(
		&record.ThreadID,
		&record.Board,
		&record.PostID,
		&lastBumpedRaw,
		&lastSeenRaw,
		&notifiedRaw,
	)
	if err == sql.ErrNoRows {
		return ThreadRecord{}, false, nil
	}
	if err != nil {
		return ThreadRecord{}, false, fmt.Errorf("query thread: %w", err)
	}

	record.LastBumpedAt, err = parseTime(lastBumpedRaw)
	if err != nil {
		return ThreadRecord{}, false, fmt.Errorf("parse last_bumped_at: %w", err)
	}
	record.LastSeenAt, err = parseTime(lastSeenRaw)
	if err != nil {
		return ThreadRecord{}, false, fmt.Errorf("parse last_seen_at: %w", err)
	}
	if notifiedRaw.Valid {
		parsed, err := parseTime(notifiedRaw.String)
		if err != nil {
			return ThreadRecord{}, false, fmt.Errorf("parse notified_new_at: %w", err)
		}
		record.NotifiedNewAt = &parsed
	}

	return record, true, nil
}

func (s *Store) UpsertThread(ctx context.Context, record ThreadRecord) error {
	const statement = `
INSERT INTO threads (thread_id, board, post_id, last_bumped_at, last_seen_at, notified_new_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(thread_id) DO UPDATE SET
  board = excluded.board,
  post_id = excluded.post_id,
  last_bumped_at = excluded.last_bumped_at,
  last_seen_at = excluded.last_seen_at,
  notified_new_at = excluded.notified_new_at;
`

	var notified any
	if record.NotifiedNewAt != nil {
		notified = formatTime(*record.NotifiedNewAt)
	}

	_, err := s.db.ExecContext(
		ctx,
		statement,
		record.ThreadID,
		record.Board,
		record.PostID,
		formatTime(record.LastBumpedAt),
		formatTime(record.LastSeenAt),
		notified,
	)
	if err != nil {
		return fmt.Errorf("upsert thread: %w", err)
	}
	return nil
}

func (s *Store) GetStreamState(ctx context.Context, key string) (StreamState, bool, error) {
	const query = `
SELECT stream_key, active, live_notified, consecutive_404s
FROM stream_states
WHERE stream_key = ?;
`

	var stream StreamState
	var active int
	var liveNotified int

	err := s.db.QueryRowContext(ctx, query, key).Scan(
		&stream.Key,
		&active,
		&liveNotified,
		&stream.Consecutive404s,
	)
	if err == sql.ErrNoRows {
		return StreamState{}, false, nil
	}
	if err != nil {
		return StreamState{}, false, fmt.Errorf("query stream state: %w", err)
	}

	stream.Active = active != 0
	stream.LiveNotified = liveNotified != 0
	return stream, true, nil
}

func (s *Store) UpsertStreamState(ctx context.Context, stream StreamState) error {
	const statement = `
INSERT INTO stream_states (stream_key, active, live_notified, consecutive_404s)
VALUES (?, ?, ?, ?)
ON CONFLICT(stream_key) DO UPDATE SET
  active = excluded.active,
  live_notified = excluded.live_notified,
  consecutive_404s = excluded.consecutive_404s;
`

	_, err := s.db.ExecContext(
		ctx,
		statement,
		stream.Key,
		boolToInt(stream.Active),
		boolToInt(stream.LiveNotified),
		stream.Consecutive404s,
	)
	if err != nil {
		return fmt.Errorf("upsert stream state: %w", err)
	}

	return nil
}

func (s *Store) GetCursor(ctx context.Context, name string) (int64, bool, error) {
	const query = `SELECT position FROM cursors WHERE name = ?;`

	var position int64
	err := s.db.QueryRowContext(ctx, query, name).Scan(&position)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("query cursor: %w", err)
	}
	return position, true, nil
}

func (s *Store) SetCursor(ctx context.Context, name string, position int64) error {
	const statement = `
INSERT INTO cursors (name, position)
VALUES (?, ?)
ON CONFLICT(name) DO UPDATE SET position = excluded.position;
`

	if _, err := s.db.ExecContext(ctx, statement, name, position); err != nil {
		return fmt.Errorf("set cursor: %w", err)
	}
	return nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func formatTime(t time.Time) string {
	return t.UTC().Format(sqliteTimeLayout)
}

func parseTime(value string) (time.Time, error) {
	t, err := time.Parse(sqliteTimeLayout, value)
	if err == nil {
		return t, nil
	}

	// Accept older rows written before we switched to the fixed-width format.
	return time.Parse(time.RFC3339Nano, value)
}
