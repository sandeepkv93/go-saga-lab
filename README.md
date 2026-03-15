# go-saga-lab

A Go project that combines:
- saga lifecycle orchestration with explicit state transitions
- PostgreSQL-backed saga persistence with built-in SQL migrations
- durable outbox writes for saga creation events
- an in-memory development mode for zero-setup local runs
- a reusable outbox dispatcher and publisher process for draining pending events

## Flow Diagram

```mermaid
flowchart LR
    C[Client] -->|POST /v1/sagas| API[Go HTTP API]
    API -->|Create saga status=created| S[Saga Repository]
    S -->|Atomic write| DB[(PostgreSQL)]
    S -->|Insert saga.created event| O[(Outbox)]
    C -->|POST /v1/sagas/:id/start| API
    API -->|Transition created -> running| R[Runtime Service]
    R -->|Persist status update| S
    C -->|POST /v1/sagas/:id/step-result| API
    API -->|Transition running -> completed or compensating| R
    C -->|POST /v1/sagas/:id/compensation-result| API
    API -->|Transition compensating -> cancelled or failed| R
    PUB[cmd/publisher] -->|Poll pending events| D[Outbox Dispatcher]
    D -->|Load pending events| O
    D -->|Publish event| P[Publisher Interface]
    D -->|Mark published or failed| O
    C -->|GET /v1/sagas/:id| API
    API -->|Read latest saga state| S
    API -->|Return JSON state| C
```

## Component Diagram

```mermaid
flowchart TB
    subgraph App
        API[cmd/api]
        PUB[cmd/publisher]
        HTTP[internal/api/httpapi]
        RT[internal/orchestrator/runtime]
        SM[internal/orchestrator/statemachine]
        OB[internal/outbox]
    end

    subgraph Storage
        MEM[internal/store/memory]
        PG[internal/store/postgres]
        SQL[(PostgreSQL)]
        OUT[(outbox_events)]
    end

    API --> HTTP
    PUB --> OB
    HTTP --> RT
    HTTP --> MEM
    HTTP --> PG
    RT --> SM
    RT --> MEM
    RT --> PG
    PG --> SQL
    PG --> OUT
    OB --> MEM
    OB --> PG
```

## Why this example is interesting

The project focuses on the hard edges that most saga examples skip:
- deterministic state transitions with illegal-transition rejection
- local-first development mode without forcing infrastructure on day one
- PostgreSQL startup checks with automatic `*.up.sql` migration application
- durable outbox capture for `saga.created`
- a dispatcher abstraction that can advance outbox state independently from API handling

This makes the repo useful as both a learning reference and a bootstrap for a more production-like orchestration service.

## Current scope

Implemented now:
- saga create, get, start, cancel, step-result, and compensation-result endpoints
- in-memory and PostgreSQL-backed repositories
- built-in migration runner for PostgreSQL startup
- atomic saga + outbox write on creation for Postgres
- outbox dispatcher and publisher process with success/failure status updates

Not implemented yet:
- concurrent step execution

## Project structure

- `cmd/api`: HTTP process entrypoint
- `cmd/publisher`: polling outbox publisher process
- `internal/api/httpapi`: request handlers and route wiring
- `internal/config`: environment-driven configuration
- `internal/domain`: saga and outbox domain types
- `internal/orchestrator/runtime`: lifecycle transition coordinator
- `internal/orchestrator/statemachine`: legal state transition table
- `internal/outbox`: dispatcher and publisher contract
- `internal/store/memory`: in-memory repository for local runs and tests
- `internal/store/postgres`: PostgreSQL repository, migrations, and startup migration runner
- `migrations`: SQL schema files applied on Postgres startup

## Run locally (memory mode)

```bash
go run ./cmd/api
```

Default behavior:
- storage backend is `memory`
- no external services are required
- HTTP server listens on `:8080` unless `PORT` is set

Run the publisher once against the selected backend:

```bash
PUBLISHER_RUN_ONCE=true go run ./cmd/publisher
```

Metrics endpoints:
- API: `http://localhost:8080/metrics`
- Publisher: `http://localhost:9091/metrics`

## Run with PostgreSQL

1. Start dependencies:

```bash
docker compose up -d postgres
```

2. Run the API with Postgres enabled:

```bash
DATABASE_URL='postgres://go_saga_lab:go_saga_lab@localhost:5432/go_saga_lab?sslmode=disable' \
STORAGE_BACKEND=postgres \
go run ./cmd/api
```

Postgres startup behavior:
- the app pings the database before serving traffic
- the app applies all `*.up.sql` files from `MIGRATIONS_DIR` (default `migrations`)
- saga creation writes `saga_instances` and `outbox_events` in one transaction

Observability stack:
- `docker compose up -d prometheus grafana`
- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000`
- provisioned dashboard: `go-saga-lab Overview`

## Storage mode matrix

| `STORAGE_BACKEND` | Saga persistence | Outbox persistence | Intended usage |
|---|---|---|---|
| `memory` | in-process map | in-process slice | local development, tests, quick demos |
| `postgres` | PostgreSQL | PostgreSQL `outbox_events` table | durable local/dev or production-like setup |

Backend selection behavior:
- if `DATABASE_URL` is set and `STORAGE_BACKEND` is unset, the app promotes to `postgres`
- otherwise the default remains `memory`

## API usage

Create a saga:

```bash
curl -sS -X POST http://localhost:8080/v1/sagas \
  -H 'Content-Type: application/json' \
  -d '{
    "template_id": "order-flow",
    "idempotency_key": "idem-1001",
    "input": {
      "order_id": "order-1001",
      "customer_id": "cust-42"
    }
  }'
```

Start the saga:

```bash
curl -sS -X POST http://localhost:8080/v1/sagas/<saga_id>/start
```

Report a successful forward step:

```bash
curl -sS -X POST http://localhost:8080/v1/sagas/<saga_id>/step-result \
  -H 'Content-Type: application/json' \
  -d '{"succeeded": true}'
```

Report a failed forward step:

```bash
curl -sS -X POST http://localhost:8080/v1/sagas/<saga_id>/step-result \
  -H 'Content-Type: application/json' \
  -d '{"succeeded": false}'
```

Finish compensation:

```bash
curl -sS -X POST http://localhost:8080/v1/sagas/<saga_id>/compensation-result \
  -H 'Content-Type: application/json' \
  -d '{"succeeded": true}'
```

Fetch current saga state:

```bash
curl -sS http://localhost:8080/v1/sagas/<saga_id>
```

Health check:

```bash
curl -sS http://localhost:8080/healthz
```

## Lifecycle walkthrough

Happy path:
- `created`
- `running`
- `completed`

Compensation path:
- `created`
- `running`
- `compensating`
- `cancelled` or `failed`

Current transition triggers:
- `start`
- `cancel`
- `step-result` with `succeeded=true|false`
- `compensation-result` with `succeeded=true|false`

## Outbox behavior

Current outbox event flow:
- saga creation emits `saga.created`
- repository stores the event with status `pending`
- `cmd/publisher` runs the dispatcher loop
- dispatcher claims eligible events with a lease owner and lease TTL, then invokes a publisher implementation
- successful publish marks the row `published`
- failed publish marks the row `failed`, increments attempt count, and schedules `next_attempt_at` with exponential backoff
- trace IDs flow from API ingress into outbox payloads, publisher logs, and RabbitMQ message headers

Current publisher backends:
- `log`: emits publish events to process logs
- `rabbitmq`: declares exchange + demo queue binding and publishes JSON messages to the configured exchange

Current limitation:
- failed events do not yet store failure reason
- there is no heartbeat or lease renewal flow for long-running publish attempts
- there is no timeout engine for saga step execution itself, only for outbox publish attempts

## Publisher transport variables

- `PUBLISHER_BACKEND` (`log|rabbitmq`, default `log`)
- `AMQP_URL` (default `amqp://guest:guest@localhost:5672/`)
- `AMQP_EXCHANGE` (default `go_saga_lab.events`)
- `AMQP_EXCHANGE_TYPE` (default `topic`)
- `AMQP_QUEUE` (default `go_saga_lab.events.demo`)
- `AMQP_ROUTING_KEY_PREFIX` (default `saga`)

## Environment variables

- `PORT` (default `8080`)
- `STORAGE_BACKEND` (`memory|postgres`, default `memory`)
- `DATABASE_URL` (optional; enables Postgres when set)
- `MIGRATIONS_DIR` (default `migrations`)
- `PUBLISHER_POLL_INTERVAL_MS` (default `2000`)
- `PUBLISHER_RETRY_BASE_MS` (default `1000`)
- `PUBLISHER_RETRY_MAX_MS` (default `30000`)
- `PUBLISHER_LEASE_TTL_MS` (default `10000`)
- `PUBLISHER_LEASE_OWNER` (default hostname-based value)
- `PUBLISHER_TIMEOUT_MS` (default `5000`)
- `PUBLISHER_METRICS_ADDR` (default `:9091`)
- `PUBLISHER_RUN_ONCE` (default `false`)

## Troubleshooting

- API starts but sagas disappear after restart:
  - you are in `memory` mode; use `DATABASE_URL` or `STORAGE_BACKEND=postgres` for durable state
- Postgres mode fails during startup:
  - verify `DATABASE_URL` connectivity and confirm the target database exists
- Saga create works but no outbox rows are published:
  - verify `cmd/publisher` is running against the same backend and storage configuration
- RabbitMQ publisher fails on startup:
  - verify `AMQP_URL`, exchange configuration, and broker reachability
- `409 Conflict` on lifecycle endpoint:
  - the requested transition is illegal for the current saga state

## Quality checks

```bash
go test ./...
go run ./cmd/api
```
