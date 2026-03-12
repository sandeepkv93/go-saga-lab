# go-saga-lab PRD

## 1. Document Control
- Product: `go-saga-lab`
- Version: `v1.0`
- Date: `2026-03-09`
- Author: `Codex + sandeepkv93`
- Status: `Draft for implementation`

## 2. Product Summary
`go-saga-lab` is a production-style saga orchestration platform in Go that helps engineers design, execute, debug, and validate distributed transactions with compensations. It provides:
- A Saga Orchestrator API (create/start/cancel/retry)
- Deterministic workflow execution with retries, backoff, and timeouts
- Compensation-first failure handling
- Outbox pattern for reliable event publishing
- Web UI timeline for state transitions and step-level diagnostics
- OpenTelemetry-native observability (traces, metrics, logs)

The goal is to bridge the gap between toy saga demos and real implementation patterns used in production systems.

## 3. Problem Statement
Most public saga examples are too simplistic:
- They skip idempotency and duplicate delivery handling.
- They do not model compensation failures and retry policies clearly.
- They lack visibility into execution timeline and causality.
- They rarely expose clear operational controls (pause/resume/replay/cancel).

Teams need a reference implementation that is deep enough to teach production behavior, but scoped enough to run locally.

## 4. Goals and Non-Goals
### 4.1 Goals
1. Provide a reusable orchestration engine for multi-step sagas.
2. Support both HTTP and async command execution (pluggable transports).
3. Ensure deterministic state transitions and recoverability from crashes.
4. Offer complete observability for each saga instance.
5. Ship local-first developer experience (`docker compose up` and run).

### 4.2 Non-Goals (v1)
1. Multi-region active-active execution.
2. BPMN graphical builder.
3. Full workflow DSL parser from user-supplied scripts.
4. Exactly-once global guarantee across external systems.
5. Enterprise auth/SSO (basic API key auth only in v1).

## 5. Target Users
1. Backend engineers learning/implementing orchestration patterns.
2. Platform/SRE engineers validating failure recovery and compensations.
3. Technical interview/system design learners needing realistic artifacts.
4. Teams needing a bootstrap reference for internal workflow engines.

## 6. Core Use Cases
1. **Order flow simulation**: Reserve inventory -> authorize payment -> create shipment; rollback when shipment step fails.
2. **Fault injection**: Trigger failure at step `N` and inspect compensation chain.
3. **Timeout and retry tuning**: Compare fixed vs exponential backoff and max attempts.
4. **Crash recovery**: Restart orchestrator mid-execution and verify resume behavior.
5. **Outbox replay**: Re-deliver events and prove idempotent consumers.

## 7. Functional Requirements
### 7.1 Saga Definition and Lifecycle
1. API to register saga templates with ordered steps.
2. Each step must define:
- `action` endpoint/handler
- `compensation` endpoint/handler
- retry policy (`maxAttempts`, `backoff`, `jitter`)
- timeout policy
3. Lifecycle states:
- Saga: `created`, `running`, `compensating`, `completed`, `failed`, `cancelled`
- Step: `pending`, `in_progress`, `succeeded`, `failed`, `compensated`, `compensation_failed`
4. Start saga with an input payload and idempotency key.
5. Cancel behavior:
- If not started: cancel directly.
- If running: transition to compensation path where applicable.

### 7.2 Execution Semantics
1. Sequential step execution for v1.
2. Retries on transient failures according to policy.
3. Durable execution state stored in Postgres.
4. Worker lease/heartbeat to recover in-flight executions.
5. Deterministic transition guards to prevent illegal state jumps.

### 7.3 Compensation and Failure Handling
1. Compensation executes in reverse order for successful prior steps.
2. Compensation retries independently from forward-action retries.
3. Terminal failure states include reason code + error envelope.
4. Support manual operator actions:
- retry failed step
- force compensate
- mark step as manual-success (audited)

### 7.4 Outbox and Eventing
1. Write domain event + state change atomically via outbox table.
2. Background publisher reads outbox and emits to broker abstraction.
3. Ensure at-least-once delivery with deduplication key.
4. Provide replay endpoint for failed deliveries.

### 7.5 APIs (v1)
1. `POST /v1/sagas/templates`
2. `POST /v1/sagas`
3. `GET /v1/sagas/{id}`
4. `GET /v1/sagas/{id}/timeline`
5. `POST /v1/sagas/{id}/cancel`
6. `POST /v1/sagas/{id}/retry`
7. `GET /v1/outbox/failures`
8. `POST /v1/outbox/replay/{event_id}`
9. `GET /healthz`, `GET /readyz`, `GET /metrics`

### 7.6 UI Requirements
1. Saga list view with filters (status, template, date range).
2. Saga detail timeline (step transitions, duration, attempts).
3. Error panel with root-cause and last 3 attempts.
4. Operator controls for retry/cancel/manual override.
5. Trace linkout for each step execution.

### 7.7 Security and Audit
1. API key auth for control-plane endpoints.
2. Structured audit log for all operator actions.
3. Redact secrets from logs/traces.
4. Input payload size limits and schema validation.

## 8. Non-Functional Requirements
1. **Reliability**: recoverable after process crash without state corruption.
2. **Performance**: p95 saga transition write < 100ms under local load (200 concurrent sagas).
3. **Scalability**: at least 1k active saga instances in local benchmark profile.
4. **Observability**: 100% saga/step transitions produce logs + metrics + traces.
5. **Testability**: deterministic integration tests with fault injection.
6. **Operability**: local stack via Docker Compose in under 5 minutes.

## 9. Success Metrics
1. `>= 95%` integration test coverage of state machine transitions.
2. Zero illegal state transitions in chaos/fault test suite.
3. Crash-recovery test pass rate `100%` across 50 seeded runs.
4. Outbox replay success rate `>= 99.9%` in local stress tests.
5. Developer setup time median `< 15 minutes` (cold machine).

## 10. Scope (v1 / v1.1 / v2)
### v1
- Core orchestrator engine
- Postgres persistence + outbox
- REST APIs
- Basic UI timeline
- OTEL instrumentation
- Compose-based local environment

### v1.1
- Broker adapters (RabbitMQ, NATS)
- Concurrent branch support (limited DAG)
- CLI for scenario execution

### v2
- Policy engine for retry/compensation strategies
- Multi-tenant isolation
- RBAC and OIDC auth

## 11. High-Level Architecture
1. `api-gateway` (Go): request validation, auth, orchestration commands.
2. `orchestrator-core` (Go): state machine + scheduler + transition guards.
3. `worker` (Go): step action/compensation execution.
4. `store` (Postgres): saga_state, saga_steps, outbox, audit_log.
5. `publisher` (Go): outbox dispatcher to broker abstraction.
6. `ui` (TypeScript/React optional, or server-rendered HTMX for v1 simplicity).
7. `otel-collector` + `prometheus` + `grafana` + `tempo/loki` for telemetry stack.

## 12. Data Model (Initial)
1. `saga_instances`
- `id`, `template_id`, `status`, `input_json`, `idempotency_key`, `created_at`, `updated_at`
2. `saga_step_executions`
- `id`, `saga_id`, `step_name`, `attempt`, `phase(action|compensation)`, `status`, `error_code`, `error_message`, `started_at`, `finished_at`
3. `outbox_events`
- `id`, `aggregate_type`, `aggregate_id`, `event_type`, `payload_json`, `dedupe_key`, `status`, `next_attempt_at`, `attempts`
4. `audit_events`
- `id`, `actor`, `action`, `target_type`, `target_id`, `metadata_json`, `created_at`

## 13. Risks and Mitigations
1. **State machine complexity drift**
- Mitigation: explicit transition table + generated tests from table.
2. **Compensation side effects not idempotent**
- Mitigation: idempotency key contract and contract tests for handlers.
3. **Outbox backlog growth**
- Mitigation: retry budgets + dead-letter status + replay tooling.
4. **Observability overhead**
- Mitigation: sampling controls + benchmark profiles.
5. **Scope creep**
- Mitigation: strict v1 exit criteria and v1.1 backlog gate.

## 14. Dependencies
1. Go `1.24+`
2. Postgres `16+`
3. Docker + Docker Compose
4. OpenTelemetry SDK and Collector
5. Optional broker: RabbitMQ or NATS (feature-flagged)

## 15. Acceptance Criteria (v1 Release)
1. Can run 3 reference sagas end-to-end locally.
2. Injected failure triggers correct reverse-order compensation.
3. Crash during execution resumes without duplicate terminal transitions.
4. Outbox events are eventually published or moved to explicit failed state.
5. UI accurately renders timeline and operator actions.
6. End-to-end traces show saga and step span hierarchy.
7. Documentation includes architecture, quickstart, and troubleshooting.

## 16. Open Questions
1. UI stack choice for v1: React SPA vs server-rendered Go templates.
2. Broker default in v1.1: RabbitMQ vs NATS.
3. Should manual override be available by default or feature-flag only?
4. Preferred license (`MIT` vs `Apache-2.0`).
