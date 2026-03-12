GO ?= go

.PHONY: test fmt run-api up down

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

run-api:
	$(GO) run ./cmd/api

up:
	docker compose up -d

down:
	docker compose down
