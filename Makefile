.PHONY: all build lint lint-fix test

all: build lint test

build:
	go build ./...

test:
	go test ./...

lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint run --timeout=180s

lint-fix:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint run --fix --timeout=180s
