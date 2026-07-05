BINARY ?= martie
IMAGE ?= martie:local
BOT_ENV ?= dev
ENV_FILE ?= .env.$(BOT_ENV)
CONFIG_FILE ?= config/$(BOT_ENV).toml
CONTAINER ?= martie-$(BOT_ENV)
VOLUME ?= martie-$(BOT_ENV)-data
DOCKER_RUN_EXTRA ?=
DOCKER_LOG_DRIVER ?= local
DOCKER_NETWORK ?=
GO_BUILD_FLAGS ?= -trimpath -buildvcs=false
LOAD_ENV = set -a; . ./$(ENV_FILE); set +a; \
	BOT_ENV=$(BOT_ENV); \
	CONFIG_FILE=$(CONFIG_FILE); \
	SQLITE_PATH=$${SQLITE_PATH:-data/$(BOT_ENV).db}; \
	export BOT_ENV CONFIG_FILE SQLITE_PATH
DOCKER_RUN_FLAGS = --env-file $(ENV_FILE) \
	-e CONFIG_FILE=/etc/martie/config.toml \
	-e SQLITE_PATH=/data/bot.db \
	--mount type=bind,source=$(abspath $(CONFIG_FILE)),target=/etc/martie/config.toml,readonly \
	--mount type=volume,source=$(VOLUME),target=/data \
	--read-only \
	--tmpfs /tmp:rw,noexec,nosuid,nodev,size=16m \
	--cap-drop ALL \
	--security-opt no-new-privileges

ifeq ($(DOCKER_LOG_DRIVER),journald)
DOCKER_LOG_FLAGS = --log-driver journald --log-opt tag=martie-$(BOT_ENV)
DOCKER_LOG_COMMAND = journalctl -t martie-$(BOT_ENV) -f
else
DOCKER_LOG_FLAGS = --log-driver local --log-opt max-size=10m --log-opt max-file=5
DOCKER_LOG_COMMAND = docker logs -f $(CONTAINER)
endif

ifneq ($(strip $(DOCKER_NETWORK)),)
DOCKER_NETWORK_FLAGS = --network $(DOCKER_NETWORK)
endif

.PHONY: help fmt lint test tidy build run snapshot docker-build docker-run docker-snapshot docker-deploy docker-logs docker-clean check clean

help:
	@printf '%s\n' \
		'Targets: fmt lint test tidy build run snapshot check clean' \
		'Docker:  docker-build docker-run docker-snapshot docker-deploy docker-logs docker-clean' \
		'Config:  BOT_ENV=dev reads config/dev.toml and .env.dev' \
		'Image:   IMAGE=martie:local' \
		'Logs:    DOCKER_LOG_DRIVER=local or journald' \
		'Network: DOCKER_NETWORK=monitoring joins an existing Docker network'

fmt:
	gofmt -w cmd internal

lint:
	go vet ./...

test:
	go test ./...

tidy:
	go mod tidy

build:
	go build $(GO_BUILD_FLAGS) -o $(BINARY) ./cmd/martie

run:
	$(LOAD_ENV); go run $(GO_BUILD_FLAGS) ./cmd/martie

snapshot:
	$(LOAD_ENV); go run $(GO_BUILD_FLAGS) ./cmd/martie snapshot

docker-build:
	docker build --pull -t $(IMAGE) .

docker-run:
	docker run -d \
		--name $(CONTAINER) \
		--restart unless-stopped \
		$(DOCKER_RUN_FLAGS) \
		$(DOCKER_LOG_FLAGS) \
		$(DOCKER_NETWORK_FLAGS) \
		$(DOCKER_RUN_EXTRA) \
		$(IMAGE)

docker-snapshot:
	docker run --rm \
		$(DOCKER_RUN_FLAGS) \
		$(DOCKER_LOG_FLAGS) \
		$(DOCKER_NETWORK_FLAGS) \
		$(IMAGE) snapshot

docker-deploy: docker-build
	-docker rm -f $(CONTAINER)
	docker run -d \
		--name $(CONTAINER) \
		--restart unless-stopped \
		$(DOCKER_RUN_FLAGS) \
		$(DOCKER_LOG_FLAGS) \
		$(DOCKER_NETWORK_FLAGS) \
		$(DOCKER_RUN_EXTRA) \
		$(IMAGE)

docker-logs:
	$(DOCKER_LOG_COMMAND)

docker-clean:
	-docker rm -f martie-dev martie-prod
	-docker volume rm martie-dev-data martie-prod-data

check: fmt lint test

clean:
	rm -f $(BINARY) martie-*
