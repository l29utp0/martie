BINARY ?= martie
IMAGE ?= martie:local
BOT_ENV ?= dev
ENV_FILE ?= .env.$(BOT_ENV)
CONTAINER ?= martie-$(BOT_ENV)
VOLUME ?= martie-$(BOT_ENV)-data
GO_BUILD_FLAGS ?= -trimpath -buildvcs=false
LOAD_ENV = set -a; . ./$(ENV_FILE); set +a; \
	BOT_ENV=$(BOT_ENV); \
	SQLITE_PATH=$${SQLITE_PATH:-data/$(BOT_ENV).db}; \
	export BOT_ENV SQLITE_PATH
DOCKER_RUN_FLAGS = --env-file $(ENV_FILE) \
	-e SQLITE_PATH=/data/bot.db \
	--mount type=volume,source=$(VOLUME),target=/data \
	--read-only \
	--tmpfs /tmp:rw,noexec,nosuid,nodev,size=16m \
	--cap-drop ALL \
	--security-opt no-new-privileges

.PHONY: help fmt lint test tidy build run snapshot docker-build docker-run docker-snapshot docker-stop docker-logs docker-clean check clean

help:
	@printf '%s\n' \
		'Targets: fmt lint test tidy build run snapshot check clean' \
		'Docker:  docker-build docker-run docker-snapshot docker-stop docker-logs docker-clean' \
		'Env:     BOT_ENV=dev reads .env.dev; BOT_ENV=prod reads .env.prod' \
		'Image:   IMAGE=martie:local'

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
		$(IMAGE)

docker-snapshot:
	docker run --rm \
		$(DOCKER_RUN_FLAGS) \
		$(IMAGE) snapshot

docker-stop:
	docker stop $(CONTAINER)

docker-logs:
	docker logs -f $(CONTAINER)

docker-clean:
	-docker rm -f martie-dev martie-prod
	-docker volume rm martie-dev-data martie-prod-data

check: fmt lint test

clean:
	rm -f $(BINARY) martie-*
