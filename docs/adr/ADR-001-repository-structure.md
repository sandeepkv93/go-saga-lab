# ADR-001 Repository Structure

## Status
Accepted

## Context
The project needs a repository layout that supports a clean Go service boundary now, while leaving room for multiple binaries, a web UI, migrations, and deployment assets.

## Decision
Use a standard Go-oriented structure:
- `cmd/` for binaries
- `internal/` for application code
- `deployments/` for Docker and telemetry assets
- `docs/adr/` for architecture decisions
- `migrations/` for database evolution

## Consequences
- The initial codebase stays small and familiar.
- Future services can be added without reshaping the repository.
- Internal packages remain encapsulated from external consumers.
