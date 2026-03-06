.PHONY: build race test lint fmt vet clean help

GOFLAGS := -trimpath
LDFLAGS := -s -w

## build: Build the race binary
build:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/race ./cmd/race

## race: Run a race with default config
race:
	go run ./cmd/race --config race.yaml

## race-config: Run a race with custom config (usage: make race-config CONFIG=my-race.yaml)
race-config:
	go run ./cmd/race --config $(CONFIG)

## test: Run all tests
test:
	go test -race -count=1 ./...

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## fmt: Format code
fmt:
	gofumpt -w .

## vet: Run go vet
vet:
	go vet ./...

## clean: Remove build artifacts and results
clean:
	rm -rf bin/ results/ coverage.out coverage.html

## help: Show this help
help:
	@grep -E '^##' Makefile | sed 's/## //'
