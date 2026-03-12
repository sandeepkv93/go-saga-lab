# go-saga-lab

`go-saga-lab` is a production-style saga orchestration reference in Go. The initial scaffold focuses on deterministic state transitions, local-first developer workflow, and a clear path toward persistence, outbox delivery, and observability.

## Current Scope
- Repository bootstrap and local development ergonomics
- Initial saga domain model and transition rules
- Postgres repository boundary and first migration set
- Runtime service that drives lifecycle transitions through the state machine
- Table-driven tests for legal and illegal transitions
- Compose stack for Postgres and telemetry collector

## Quickstart
1. Copy `.env.example` to `.env` if you want local overrides.
2. Start dependencies with `make up`.
3. Apply migrations with your preferred tool against the SQL in `migrations/`.
4. Run tests with `make test`.
5. Start the API placeholder with `make run-api`.

## Repository Layout
```text
cmd/
  api/
internal/
  domain/
  orchestrator/statemachine/
deployments/
  docker/
docs/
  adr/
  runbooks/
migrations/
tests/
web/
```

## Current Commands
- `make test`: run the Go test suite
- `make fmt`: format Go code
- `make run-api`: start the placeholder API process
- `make up`: start local dependencies
- `make down`: stop local dependencies

## API Status
- Default backend is in-memory for zero-setup local runs.
- Set `DATABASE_URL` or `STORAGE_BACKEND=postgres` to switch the binary to Postgres.
- On Postgres startup, the binary pings the database and applies `*.up.sql` files from `MIGRATIONS_DIR` (default: `migrations`).
- Saga creation now emits a `saga.created` outbox event. On Postgres this write is atomic with saga persistence.
- The `internal/outbox` package now includes a dispatcher that marks pending events as `published` or `failed`.
- Available lifecycle endpoints:
- `POST /v1/sagas`
- `GET /v1/sagas/{id}`
- `POST /v1/sagas/{id}/start`
- `POST /v1/sagas/{id}/cancel`
- `POST /v1/sagas/{id}/step-result`
- `POST /v1/sagas/{id}/compensation-result`

## Near-Term Build Order
1. Implement orchestration runtime with retries and compensation.
2. Add lifecycle APIs and reference saga scenarios.
3. Wire telemetry and dashboards.
4. Add outbox publisher and replay flow.

See [PRD.md](/home/dev/workspace/go-saga-lab/PRD.md) and [PLAN.md](/home/dev/workspace/go-saga-lab/PLAN.md) for full product definition and execution sequencing.
