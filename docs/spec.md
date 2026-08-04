# Paper Specification

Paper gives local coding agents a durable, multi-project record of completed work and avoidable friction. Markdown is the authoritative data store so a human can inspect and edit records with ordinary tools, while strict parsing keeps malformed state from entering command behavior.

## Goals

Paper must make logging cheaper than silently pushing through a problem. It must answer what work was completed today, preserve enough context to act on agent complaints, attribute records without per-project setup, and remain safe when several agents write concurrently.

Paper is local-only in this version. GitHub publishing, background synchronization, project-local stores, generated daily notes, attachments, and automatic hooks are outside the scope.

## Record Model

Every record has a timestamp-based ID, creation time, summary, project, project root, invocation directory, optional Git state, optional agent, and tags. IDs have the form `YYYYMMDDTHHMMSS-summary-slug-xxxx`, where the suffix contains two cryptographically random bytes.

A work record has optional details. A work record cannot contain cut state or resolution metadata.

An open cut has a controlled kind and a required description. Attempted action, impact, workaround, evidence, and suggested fix are optional.

A resolved cut has all open-cut data plus a resolution timestamp and non-empty resolution. Open and resolved cuts are distinct domain types after parsing.

Supported cut kinds are:

```text
tool-call
broken-link
command
documentation
environment
workflow
other
```

## Markdown Format

One record occupies one Markdown file. Typed metadata lives in YAML frontmatter, while narrative content uses known level-two sections.

```markdown
---
id: 20260804T144503-documentation-points-to-a-removed-api-f19d
type: cut
created_at: 2026-08-04T14:45:03-04:00
status: open
summary: Documentation points to a removed API page
kind: broken-link
tags:
  - paper/cut
project:
  name: paper
  key: github.com/gabrielchantayan/paper
  root: /Users/gabe/Programming/paper
  remote: git@github.com:gabrielchantayan/paper.git
directory: internal/store
git:
  branch: main
  commit: abc1234
agent: opencode
---

## Description

The referenced page returns 404 and no replacement is linked.

## Attempted

Searched the documentation site.

## Workaround

Read the package source locally.
```

The parser rejects unknown frontmatter fields, duplicate structural sections, invalid timestamps and IDs, unknown cut kinds, empty required sections, multiple YAML documents, and fields that conflict with the record variant. Narrative Markdown headings are escaped when they would otherwise collide with Paper's structural headings and restored during parsing. Parsing completes before any command mutates state.

Resolution preserves the file path and replaces the complete record atomically. The frontmatter status becomes `resolved`, `resolved_at` is added, and a `Resolution` section is appended.

## Storage

The default vault is `~/.paper`. `paper init --vault PATH` stores an absolute override in `$XDG_CONFIG_HOME/paper/config.yaml` or `~/.config/paper/config.yaml`.

Vault selection uses command option, environment, saved configuration, then default order. `paper init` and every store-opening command create the required work and cuts directories when absent.

Work records are partitioned by creation year and month under `work/`. Cuts remain under `cuts/` because their state can change. New records are written to a temporary file, flushed, then linked into their final unique path without overwriting an existing record. Resolution writes and flushes a complete replacement before renaming it over the original.

Listing fails when any Markdown record in the relevant tree fails to parse. This prevents a partial result from presenting corrupted storage as complete.

## Project Context

Paper invokes Git directly without a shell. In a worktree, `git rev-parse --show-toplevel` supplies the root. The origin remote becomes the canonical key after protocol, user information, and `.git` are removed. The raw remote is retained.

Paper records the symbolic branch when attached and the current commit when one exists. Missing remotes, unborn branches, and detached heads are valid and omit unavailable fields.

Outside Git, the current absolute directory is both root and canonical key. Its basename is the project name. `--project` changes only the displayed name.

## Command Behavior

All write commands require named fields and reject unexpected positional arguments. `show` and `resolve` require exactly one positional ID and support flags before or after it. `-` reads one narrative field from standard input; a cut rejects attempts to read multiple fields from the same stream.

Text output is concise and intended for terminals. JSON output is an explicit stable projection of the parsed domain record, not serialization of Go implementation structs. JSON commands emit one value and no diagnostic text to standard output.

`paper cuts` returns open cuts globally by default. `--all` includes resolved cuts. Project filtering matches the exact project name or canonical key.

`paper history` returns records from the current local day by default. `--since` and `--until` accept inclusive calendar dates. `--all` cannot be combined with date bounds.

`paper show` and `paper resolve` accept a unique ID prefix. Missing and ambiguous prefixes fail without mutation. Resolving work or an already resolved cut also fails.

## Agent Integration

`paper agent install` writes a managed `paper/SKILL.md` under the user-level Claude Code, OpenCode, Kimi Code, or Pi skill root. `all` operates on every supported target. Environment-specific configuration roots are respected through `CLAUDE_CONFIG_DIR`, `OPENCODE_CONFIG_DIR`, `XDG_CONFIG_HOME`, `KIMI_CODE_HOME`, and `PI_CODING_AGENT_DIR`.

The installed skill instructs agents to inspect open cuts before work, record actionable friction immediately, continue the original task, and record one concise outcome after meaningful completion. Claude Code uses `--agent claude-code`, OpenCode uses `--agent opencode`, Kimi Code uses `--agent kimi-code`, and Pi uses `--agent pi`.

Each installation includes a manifest containing Paper ownership and the expected SHA-256 digest. Repeated installation is idempotent. Paper refuses to overwrite unmanaged, modified, or outdated content; upgrading requires an explicit uninstall followed by install. Uninstall removes only the managed skill and manifest.

## Verification

The implementation is accepted when these commands succeed:

```sh
go test ./...
go vet ./...
go build ./cmd/paper
```

Tests cover record round trips, malformed and incomplete records, remote normalization, concurrent creation, lookup and resolution, whole-scan failure, stdin input, JSON lifecycle behavior, required-field failure before storage, idempotent skill installation, and protection of unmanaged skills.
