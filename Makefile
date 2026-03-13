GO ?= go

.PHONY: test fmt run-api run-publisher up down

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

run-api:
	$(GO) run ./cmd/api

run-publisher:
	$(GO) run ./cmd/publisher

up:
	docker compose up -d

down:
	docker compose down
