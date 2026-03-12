# go-saga-lab Execution Plan

## 1. Mission Objective
Deliver `go-saga-lab` v1 as a local-first, production-style saga orchestration reference in Go with deterministic execution, compensation, outbox reliability, and deep observability.

## 2. Scope and Constraints
- Scope: new repo scaffold and v1 implementation plan.
- Constraints:
  - Local runnable via Docker Compose.
  - Production patterns over toy shortcuts.
  - Preserve simple onboarding (`<15 min` setup).
  - Prioritize correctness and recoverability over feature breadth.

## 3. Success Criteria
1. Core state machine + persistence implemented and tested.
2. Public API usable for create/start/get/cancel/retry.
3. At least one full reference saga with failure injection.
4. Telemetry dashboards available out-of-box.
5. CI gates for lint/test/integration and basic load test.

## 4. Work Breakdown (DAG)
### T1. Repository Bootstrap
- Goal: initialize structure and build system.
- Owner: executor
- Dependencies: none
- File ownership:
  - `/cmd/*`, `/internal/*`, `/web/*`, `/deployments/*`, `/Makefile`
- Risk tier: 0
- Primary risk: folder churn and weak conventions.
- Mitigation: enforce standard Go layout + ADR-001.
- Rollback: remove scaffold commit.
- Validation required: `go test ./...` (empty/smoke), `make lint`.

### T2. Domain and State Machine
- Goal: implement saga domain model and legal transition table.
- Owner: executor
- Dependencies: T1
- File ownership:
  - `/internal/domain/*`, `/internal/orchestrator/statemachine/*`
- Risk tier: 1
- Primary risk: illegal transitions under retries/restarts.
- Mitigation: transition map + table-driven tests + fuzz checks.
- Rollback: revert statemachine package to prior stable tag.
- Validation required: domain unit tests, transition invariant tests.

### T3. Persistence Layer
- Goal: implement Postgres repositories and migration scripts.
- Owner: executor
- Dependencies: T1, T2
- File ownership:
  - `/internal/store/postgres/*`, `/migrations/*`
- Risk tier: 1
- Primary risk: inconsistent transaction boundaries.
- Mitigation: explicit Tx boundary for state + outbox writes.
- Rollback: migration rollback scripts and feature-flagged writes.
- Validation required: repository integration tests with testcontainers.

### T4. Orchestrator Runtime + Worker
- Goal: scheduler loop, leasing, retries, timeout, compensation logic.
- Owner: executor
- Dependencies: T2, T3
- File ownership:
  - `/internal/orchestrator/runtime/*`, `/internal/worker/*`
- Risk tier: 2
- Primary risk: duplicate execution after crash.
- Mitigation: lease token, heartbeat timeout, idempotent attempt keys.
- Rollback: disable async worker and run single-thread safe mode.
- Validation required: crash-recovery and duplicate-prevention tests.

### T5. API Layer
- Goal: REST endpoints for lifecycle and operator controls.
- Owner: executor
- Dependencies: T3, T4
- File ownership:
  - `/internal/api/http/*`, `/openapi/*`
- Risk tier: 1
- Primary risk: weak validation causing runtime faults.
- Mitigation: request schema validation + typed errors.
- Rollback: keep endpoints read-only behind feature flags.
- Validation required: API contract tests and negative-path tests.

### T6. Outbox Publisher
- Goal: reliable outbox dispatcher and replay operations.
- Owner: executor
- Dependencies: T3, T4
- File ownership:
  - `/internal/outbox/*`
- Risk tier: 2
- Primary risk: runaway retries / duplicate emits.
- Mitigation: capped retry strategy + dedupe key + terminal failure state.
- Rollback: pause publisher loop; preserve outbox rows.
- Validation required: delivery tests, replay tests, failure tests.

### T7. Observability Stack
- Goal: traces, metrics, structured logs, baseline dashboards.
- Owner: executor
- Dependencies: T4, T5, T6
- File ownership:
  - `/internal/telemetry/*`, `/deployments/otel/*`, `/deployments/grafana/*`
- Risk tier: 1
- Primary risk: missing trace correlation.
- Mitigation: mandatory saga/step correlation IDs propagated everywhere.
- Rollback: disable non-critical exporters, retain stdout telemetry.
- Validation required: trace continuity tests + metrics assertions.

### T8. UI Timeline
- Goal: provide saga timeline and operator controls.
- Owner: executor
- Dependencies: T5
- File ownership:
  - `/web/*`
- Risk tier: 1
- Primary risk: stale UI state vs backend truth.
- Mitigation: poll + ETag/updatedAt checks.
- Rollback: serve read-only timeline without controls.
- Validation required: UI e2e smoke tests.

### T9. Quality Gates and CI
- Goal: enforce CI pipeline with lint/test/integration.
- Owner: executor
- Dependencies: T2-T8
- File ownership:
  - `/.github/workflows/*`, `/scripts/ci/*`
- Risk tier: 1
- Primary risk: slow/flaky CI.
- Mitigation: split fast and slow lanes, deterministic fixtures.
- Rollback: temporarily relax non-critical checks with explicit TODO expiry.
- Validation required: CI green across PR branches.

### T10. Documentation and Release Pack
- Goal: complete README, ADRs, runbooks, troubleshooting, demo scripts.
- Owner: executor
- Dependencies: T1-T9
- File ownership:
  - `/README.md`, `/docs/*`
- Risk tier: 0
- Primary risk: docs drift from behavior.
- Mitigation: doc tests + script-backed examples.
- Rollback: mark incomplete sections clearly and block release.
- Validation required: quickstart dry run from clean environment.

## 5. Milestones and Timeline (Suggested)
1. Week 1: T1-T3 (scaffold, domain, persistence)
2. Week 2: T4-T6 (runtime, API, outbox)
3. Week 3: T7-T8 (observability, UI)
4. Week 4: T9-T10 (CI hardening, docs, release candidate)

## 6. Validation Matrix
1. Unit tests: domain transitions, retry policies, compensation ordering.
2. Integration tests: Postgres repos, outbox dispatch, API behavior.
3. Failure-path tests: handler failures, timeout, crash-restart recovery.
4. E2E tests: reference order saga successful and failed runs.
5. Performance smoke: 200 concurrent saga start requests, p95 latency capture.
6. Security checks: static analysis (`gosec`), dependency scan (`govulncheck`).

## 7. Reference Test Scenarios
1. `happy_path_order_flow`
2. `shipment_failure_triggers_compensation`
3. `payment_timeout_with_retry_then_success`
4. `worker_crash_mid_step_resume_once`
5. `outbox_publish_fail_then_replay`
6. `manual_override_audited`

## 8. Repo Structure Proposal
```text
go-saga-lab/
  cmd/
    api/
    worker/
    publisher/
  internal/
    domain/
    orchestrator/
    store/
    outbox/
    api/
    telemetry/
  migrations/
  web/
  deployments/
    docker/
    otel/
    grafana/
  docs/
    adr/
    runbooks/
  scripts/
  tests/
```

## 9. Execution Mode and Governance
- Execution mode: `single-agent` until T6, then re-evaluate for parallel work (`T7` and `T8`).
- Checkpoint cadence: every 1-2 implementation sessions.
- Branching strategy:
  - `main`: protected
  - `feat/<milestone>-<scope>` branches
- PR policy:
  - Require 1 reviewer
  - Require green CI
  - Block merge if failure-path tests are missing for Tier 1+ changes

## 10. Risk Register
1. **R1 Duplicate side effects** (Tier 2)
- Detection: repeated external call signatures per saga step.
- Mitigation: idempotency keys + dedupe storage.
2. **R2 Compensation failure loops** (Tier 2)
- Detection: compensation attempts exceed threshold.
- Mitigation: dead-letter compensation + operator intervention path.
3. **R3 Transition race conditions** (Tier 2)
- Detection: optimistic lock conflict spikes.
- Mitigation: versioned rows and compare-and-swap writes.
4. **R4 Local setup friction** (Tier 1)
- Detection: onboarding failures from test users.
- Mitigation: one-command bootstrap script.

## 11. Rollout Strategy
1. Alpha: local dev only with demo saga.
2. Beta: stability focus with fault-injection suite and performance smoke.
3. v1.0: docs-complete release with tagged examples and dashboards.

## 12. Done Definition
1. All v1 acceptance criteria in PRD are met.
2. CI is green on default branch.
3. Crash recovery and compensation tests pass reliably.
4. Quickstart succeeds from clean machine.
5. Release notes and architecture docs published.

## 13. Immediate Next Steps
1. Initialize repo and commit scaffold (`T1`).
2. Implement transition table and tests (`T2`).
3. Add Postgres schema and repository integration tests (`T3`).
