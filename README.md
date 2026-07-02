# martie

`martie` watches the `ptchan` overboard catalog and forwards newly seen threads to a Telegram chat.
It also checks a small set of `miau` stream URLs and notifies when a stream comes online.

It stays intentionally small:

- polls `https://ptchan.org/catalog.json`
- tracks seen threads in SQLite
- sends Telegram messages for new matches
- sends Telegram messages when its default `miau` streams go live
- optionally exposes Prometheus metrics
- stores only the state it needs

- no webhook
- no inbound API beyond optional `/metrics`
- no Telegram commands
- no full-thread fetches

## Config

Copy `.env.example` to `.env.dev` and `.env.prod`, then fill in the settings you need.
Configuration is grouped by concern:

- Telegram:
  - `TELEGRAM_BOT_TOKEN`
  - `TELEGRAM_CHAT_ID`
- Catalog:
  - `PTCHAN_BASE_URL`
  - `MIN_REPLY_POSTS`
  - `BOARD_DENYLIST`
  - `KEYWORD_DENYLIST`
  - `MAX_THREAD_AGE_HOURS`
  - `PRUNE_AFTER_HOURS`
- Runtime:
  - `POLL_INTERVAL_SECONDS`
  - `METRICS_ADDR`
- Storage:
  - `SQLITE_PATH`

The required settings for `run` are:

- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_CHAT_ID`

Make targets default to `BOT_ENV=dev`. Use `BOT_ENV=prod` for `.env.prod`.

If `SQLITE_PATH` is blank, local runs use `data/dev.db` or `data/prod.db`.

Set `METRICS_ADDR` to enable a Prometheus scrape endpoint at `/metrics`, for example `:9090`.
Workflow health is labeled by `workflow` (`catalog` or `streams`), notification counts use the same values in `source`, and catalog gauges are labeled by `board`.
The principal metric families are:

- `martie_workflow_runs_total`
- `martie_workflow_duration_seconds`
- `martie_workflow_last_success`
- `martie_workflow_last_successful_timestamp_seconds`
- `martie_notifications_sent_total`
- `martie_ptchan_catalog_*`

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

Local builds require Go 1.25 or newer.
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

To publish the metrics endpoint from Docker, set `METRICS_ADDR=:9090` and pass a port mapping:

```bash
make docker-run BOT_ENV=prod DOCKER_RUN_EXTRA='-p 127.0.0.1:9090:9090'
```

The image is a static `scratch` runtime with CA certificates, a non-root user, no default exposed ports, and SQLite state under `/data`. Secrets stay in the runtime environment.

## Behavior

- The bot only notifies for newly discovered threads.
- On first run it will notify for everything currently in the catalog unless you run `make snapshot` first.
- `MIN_REPLY_POSTS` can delay a notification until a thread reaches the reply threshold.
- `make snapshot` stores the current catalog and marks only threads that already meet `MIN_REPLY_POSTS` as handled.
- `BOARD_DENYLIST`, `KEYWORD_DENYLIST`, `MAX_THREAD_AGE_HOURS`, and `PRUNE_AFTER_HOURS` filter what is tracked and how long it stays in SQLite.
- New threads are stored before send; if Telegram accepts a message but the follow-up SQLite write fails, that notification may be retried on the next poll.
- Ptchan and miau polling run independently; a failure in one does not stop the other.
- The default miau channels are `oficial` and `l29utp0`.
- Miau stream checks treat a live URL as active until it returns `404` for 2 consecutive poll cycles.

## License

This project is licensed under the GNU General Public License, version 3 or any later version. See `LICENSE`.
