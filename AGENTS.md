# Agent Guidelines

## Purpose

Paper records completed work and actionable friction for coding agents. Preserve its local-first, Markdown-authoritative design and keep agent-facing commands deterministic and non-interactive.

## Workflow

- Run `paper cuts --format json` before substantial work when Paper is available.
- Record actionable tooling, documentation, environment, or workflow friction when it occurs. Do not log routine typos or expected exploration.
- Record one concise `paper log` outcome after completing meaningful work.
- Never modify records in a user's real vault during automated tests. Use `PAPER_VAULT` with a temporary directory.

## Engineering

- Parse external input completely into domain types before acting on it.
- Keep open and resolved cuts as distinct domain states.
- Refuse malformed records instead of silently skipping them.
- Preserve unmanaged or user-modified agent skills.
- Keep JSON output stable and diagnostics on standard error.
- Add deterministic regression coverage for every bug fix.

## Verification

Run:

```sh
go test -race ./...
go vet ./...
go build ./cmd/paper
```

For installer or workflow changes, exercise the built binary and agent installation against an isolated `HOME`.
