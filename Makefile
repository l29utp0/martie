BINARY ?= martie
GOFILES := $(shell rg --files -g '*.go')
GO_BUILD_FLAGS ?= -trimpath -buildvcs=false

.PHONY: help fmt lint test tidy build run seed check clean

help:
	@printf '%s\n' \
		'Available targets:' \
		'  make fmt    - format Go source with gofmt' \
		'  make lint   - run go vet' \
		'  make test   - run go test ./...' \
		'  make tidy   - sync module files' \
		'  make build  - build the martie binary' \
		'  make run    - run the bot locally' \
		'  make seed   - store the current catalog as handled and exit' \
		'  make check  - run fmt, lint, and test' \
		'  make clean  - remove local build outputs'

fmt:
	gofmt -w $(GOFILES)

lint:
	go vet ./...

test:
	go test ./...

tidy:
	go mod tidy

build:
	go build $(GO_BUILD_FLAGS) -o $(BINARY) ./cmd/martie

run:
	go run $(GO_BUILD_FLAGS) ./cmd/martie

seed:
	go run $(GO_BUILD_FLAGS) ./cmd/martie seed

check: fmt lint test

clean:
	rm -f $(BINARY) martie-*
