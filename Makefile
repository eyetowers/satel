.PHONY: all build lint lint-fix test

all: build lint test

build:
	go build ./...

test:
	go test ./...

lint:
	go tool golangci-lint run --timeout=180s

lint-fix:
	go tool golangci-lint run --fix --timeout=180s
