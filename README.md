# martie

Martie is a small Telegram bot for the ptchan community. It can run three independent components:

- `catalog` watches the ptchan catalog and announces new threads.
- `streams` watches configured stream URLs and announces when they go live.
- `assistant` answers messages addressed to the bot in a Telegram discussion group using DeepSeek.

Martie uses long polling, stores its small amount of durable state in SQLite, and can expose Prometheus metrics. It has no webhook or public API beyond the optional metrics endpoint.

## Run locally

Requirements: Go 1.25 or newer, a Telegram bot token, and a DeepSeek API key when running the assistant.

```bash
cp .env.example .env.dev
mkdir -p config
cp config.example.toml config/dev.toml
```

Edit both files, then run:

```bash
make run
```

Configuration is split deliberately:

- `.env.dev` contains secrets.
- `config/dev.toml` contains application settings.
- `runtime.components` selects `catalog`, `streams`, `assistant`, or any combination.

The example TOML documents every setting. Unknown keys and invalid values fail at startup. `BOT_ENV=prod` selects `.env.prod`, `config/prod.toml`, and `data/prod.db`.

## First catalog run

The catalog component announces every eligible thread it has not seen before. To establish the current catalog as the baseline without sending notifications, run this once before starting Martie:

```bash
make snapshot BOT_ENV=prod
```

The snapshot command only needs catalog and storage configuration; it does not require the runtime component list or Telegram and DeepSeek secrets.

## Deploy with Docker

Create `.env.prod` and `config/prod.toml`, then deploy:

```bash
make docker-deploy BOT_ENV=prod
```

Useful operational commands:

```bash
make docker-logs BOT_ENV=prod
make docker-snapshot BOT_ENV=prod
make docker-clean
```

The container runs as a non-root user with a read-only filesystem. The selected TOML file is mounted read-only, secrets are passed through the environment, and SQLite is stored in the persistent `martie-prod-data` volume.

Docker logging defaults to the rotating `local` driver, capped at five 10 MB files per container. This is safe without host setup, but removing a container removes its history. On a systemd server, use journald to retain logs across deployments:

```bash
make docker-deploy BOT_ENV=prod DOCKER_LOG_DRIVER=journald
make docker-logs BOT_ENV=prod DOCKER_LOG_DRIVER=journald
```

Ensure the host journal is persistent and bounded with `/etc/systemd/journald.conf.d/martie.conf`:

```ini
[Journal]
Storage=persistent
SystemMaxUse=500M
MaxRetentionSec=30day
```

Apply the host configuration with `sudo systemctl restart systemd-journald`. In journald mode, `make docker-logs` runs `journalctl`; the current user therefore needs journal access. If it is denied, use `sudo journalctl -t martie-prod -f` or grant the user the host's journal-reader group. Historical logs can be queried with `journalctl -t martie-prod --since yesterday`. Hosts without journald should keep the default `local` driver.

To scrape Martie from Prometheus in another container, set `runtime.metrics_addr = ":9090"` and attach both containers to the same user-defined Docker network:

```bash
docker network create monitoring # once, unless the network already exists
make docker-deploy BOT_ENV=prod DOCKER_NETWORK=monitoring
```

Prometheus can then scrape `martie-prod:9090` without publishing the port on the host. `DOCKER_NETWORK` must name an existing network. For a host-based or external Prometheus, publish the port explicitly with `DOCKER_RUN_EXTRA`; metrics are available at `/metrics`.

## Telegram setup notes

The notification chat receives catalog and stream announcements. The discussion chat is where the assistant listens for mentions and replies.

To receive ordinary group mentions, make the bot a group administrator or disable Group Privacy in BotFather. If you do not know the discussion chat ID, run Martie, mention it in the group, and inspect the debug log for the observed chat ID.

Access to the assistant is fail-closed by default. Configure `telegram.allowed_user_ids`, or set `telegram.allow_all_users = true` intentionally.

When the assistant is enabled, addressed message text and recent conversation context are sent to the configured DeepSeek API. Telegram identities are replaced with temporary aliases, but message content is not anonymized.

The assistant can optionally enrich requests that contain ptchan thread links. When `assistant.ptchan_context.enabled` is true, Martie fetches the live thread JSON from ptchan, wraps a bounded OP + recent replies snapshot as untrusted external context, and sends that only for the current completion. The fetched snapshot is not persisted in conversation history.

## Development

```bash
make check   # format, vet, and test
make build
```

See `make help` for the complete command list.

## License

GNU General Public License, version 3 or later. See `LICENSE`.
