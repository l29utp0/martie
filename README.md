# martie

Passive-only Telegram bot for watching `ptchan` overboard threads and forwarding new ones to a Telegram chat.

## What It Does

- polls `https://ptchan.org/catalog.json`
- detects threads not seen before
- stores state in local SQLite
- sends a Telegram message for each new thread
- avoids storing more than it needs

This first version is intentionally small:
- no webhook
- no inbound HTTP server
- no Telegram commands
- no full-thread fetches yet

## Stack

- Go
- Telegram Bot API over raw HTTP
- SQLite via `modernc.org/sqlite`

## Configuration

Copy `.env.example` values into your environment:

- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_CHAT_ID`
- `PTCHAN_BASE_URL`
- `POLL_INTERVAL_SECONDS`
- `SQLITE_PATH`
- `MIN_REPLY_POSTS`
- `BOARD_DENYLIST`
- `KEYWORD_DENYLIST`
- `MAX_THREAD_AGE_HOURS`
- `PRUNE_AFTER_HOURS`

## Run

```bash
make tidy
make run
```

To snapshot the current catalog as already handled and exit:

```bash
make seed
```

## Build

`make build` uses Go's `-trimpath` and `-buildvcs=false` flags so release binaries do not embed your local filesystem path or repo state.

## License

This project is licensed under the GNU General Public License, version 3 or any later version. See `LICENSE`.

## Behavior

- The bot notifies only for newly discovered threads.
- On first run it will notify for everything currently in the catalog unless you run `make seed` first to store the current catalog as already handled.
- `MIN_REPLY_POSTS` can delay a notification until a thread reaches the reply threshold.
- `BOARD_DENYLIST`, `KEYWORD_DENYLIST`, `MAX_THREAD_AGE_HOURS`, and `PRUNE_AFTER_HOURS` filter what is tracked and how long it stays in SQLite.
- New threads are stored before send; if Telegram accepts a message but the follow-up SQLite write fails, that notification may be retried on the next poll.
