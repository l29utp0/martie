# martie

`martie` watches the `ptchan` overboard catalog and forwards newly seen threads to a Telegram chat.

It stays intentionally small:

- polls `https://ptchan.org/catalog.json`
- tracks seen threads in SQLite
- sends Telegram messages for new matches
- stores only the state it needs

- no webhook
- no inbound HTTP server
- no Telegram commands
- no full-thread fetches

## Config

Copy `.env.example` to `.env.dev` and `.env.prod`, then fill in:

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

Make targets default to `BOT_ENV=dev`. Use `BOT_ENV=prod` for `.env.prod`.

If `SQLITE_PATH` is blank, local runs use `data/dev.db` or `data/prod.db`.

## Local Run

```bash
cp .env.example .env.dev
make tidy
make run
```

To use the production config locally:

```bash
make run BOT_ENV=prod
```

To snapshot the current catalog and exit without sending notifications:

```bash
make snapshot
```

## Build

`make build` uses Go's `-trimpath` and `-buildvcs=false` flags so release binaries do not embed your local filesystem path or repo state.

## Docker

First run:

```bash
make docker-build
make docker-run BOT_ENV=prod
```

Update an existing server:

```bash
git pull
make docker-deploy BOT_ENV=prod
```

Other useful commands:

```bash
make docker-snapshot BOT_ENV=prod
make docker-logs BOT_ENV=prod
make docker-clean
```

Docker uses the same `BOT_ENV` selection as local runs. SQLite always lives at `/data/bot.db` in the container, backed by a named Docker volume such as `martie-prod-data`, so state survives redeploys.

The image is a static `scratch` runtime with CA certificates, a non-root user, no exposed ports, and SQLite state under `/data`. Secrets stay in the runtime environment.

## Behavior

- The bot only notifies for newly discovered threads.
- On first run it will notify for everything currently in the catalog unless you run `make snapshot` first.
- `MIN_REPLY_POSTS` can delay a notification until a thread reaches the reply threshold.
- `make snapshot` stores the current catalog and marks only threads that already meet `MIN_REPLY_POSTS` as handled.
- `BOARD_DENYLIST`, `KEYWORD_DENYLIST`, `MAX_THREAD_AGE_HOURS`, and `PRUNE_AFTER_HOURS` filter what is tracked and how long it stays in SQLite.
- New threads are stored before send; if Telegram accepts a message but the follow-up SQLite write fails, that notification may be retried on the next poll.

## License

This project is licensed under the GNU General Public License, version 3 or any later version. See `LICENSE`.
