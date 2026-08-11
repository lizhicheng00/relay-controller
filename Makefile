.PHONY: build test check race run

GO ?= go

build:
	$(GO) build -trimpath -ldflags "-s -w -X main.version=1.0.0" -o bin/relay-controller ./cmd/relay-controller

test:
	$(GO) test ./...

check:
	$(GO) test ./...
	$(GO) vet ./...

race:
	CGO_ENABLED=1 $(GO) test -race ./...

run:
	$(GO) run ./cmd/relay-controller
