# Paper

Paper is a local-first work and friction log for coding agents. It records what agents completed with `paper log`, captures avoidable friction with `paper cut`, and keeps every record as readable Markdown in one multi-project vault.

Paper does not publish issues or run background services. Agents make explicit, fast CLI calls while the files remain yours to search, sync, back up, or open in Obsidian.

[Website](https://gabrielchantayan.github.io/paper/) · [Releases](https://github.com/gabrielchantayan/paper/releases) · [Agent setup contract](SETUP.md)

## Install

On macOS or Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/gabrielchantayan/paper/main/install.sh | sh
```

On Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/gabrielchantayan/paper/main/install.ps1 | iex
```

The installers resolve one tagged release, verify its archive against that release's `checksums.txt`, and install to `~/.local/bin` by default. Set `PAPER_INSTALL_DIR` to choose another directory or pass a version such as `v1.2.3` to pin the installation.

With Go 1.25 or newer:

```sh
go install github.com/gabrielchantayan/paper/cmd/paper@latest
```

From a local checkout:

```sh
go build -o paper ./cmd/paper
```

## Agent Quick Setup

Paste this into Claude Code, OpenCode, Kimi Code, or Pi:

```text
Read https://gabrielchantayan.github.io/paper/setup.md and set up Paper for this environment.
```

The setup contract installs Paper, preserves existing configuration, installs only the current agent's managed skill, verifies the result, and stops rather than overwriting unmanaged files.

## Quick Start

Paper uses `~/.paper` by default. Initialize another vault when desired:

```sh
paper init --vault ~/Documents/Paper
```

Record completed work:

```sh
paper log \
  --summary "Added strict parsing for Paper records" \
  --details "Covered malformed frontmatter and resolved-cut invariants." \
  --agent opencode
```

Record friction when it happens:

```sh
paper cut \
  --summary "Documentation points to a removed API page" \
  --description "The setup link returns 404 and no replacement is linked." \
  --kind broken-link \
  --attempted "Searched the documentation site." \
  --workaround "Read the package source locally." \
  --agent opencode
```

Review and resolve cuts:

```sh
paper cuts
paper show 20260804T144503-broken-doc
paper resolve 20260804T144503-broken-doc --resolution "Updated the documentation link."
```

See what got done today:

```sh
paper history
```

## Agent Skills

Paper can install a managed user-level skill for Claude Code, OpenCode, Kimi Code, Pi, or every supported agent:

```sh
paper agent install all
paper agent doctor
paper agent uninstall opencode
```

The skill tells agents to check existing cuts, record new friction immediately, continue the original task, and write one concise outcome after meaningful work. Paper refuses to overwrite or remove an unmanaged or manually modified skill.

## Commands

```text
paper init [--vault PATH]

paper log --summary TEXT
          [--details TEXT|-]
          [--tag TAG]
          [--agent NAME]
          [--project NAME]

paper cut --summary TEXT
          --description TEXT|-
          [--kind KIND]
          [--attempted TEXT]
          [--impact TEXT]
          [--workaround TEXT]
          [--evidence TEXT]
          [--suggested-fix TEXT]
          [--tag TAG]
          [--agent NAME]
          [--project NAME]

paper cuts [--all] [--project PROJECT] [--format text|json]
paper history [--since YYYY-MM-DD] [--until YYYY-MM-DD] [--all]
paper show ID [--format text|json]
paper resolve ID --resolution TEXT|-

paper agent install|uninstall claude|opencode|kimi|pi|all
paper agent doctor [claude|opencode|kimi|pi|all]
```

The native user-level skill locations are:

```text
Claude Code  ${CLAUDE_CONFIG_DIR:-~/.claude}/skills/paper/SKILL.md
OpenCode     ${OPENCODE_CONFIG_DIR:-${XDG_CONFIG_HOME:-~/.config}/opencode}/skills/paper/SKILL.md
Kimi Code    ${KIMI_CODE_HOME:-~/.kimi-code}/skills/paper/SKILL.md
Pi           ${PI_CODING_AGENT_DIR:-~/.pi/agent}/skills/paper/SKILL.md
```

Run `paper agent` or `paper agent --help` for target descriptions and examples. `paper agent doctor` reports every target's installation status, resolved path, and logging identity.

Use `-` for one narrative field to read it from standard input. Mutating and read commands support `--vault`; record-producing commands and lists support `--format text|json`.

Cut kinds are `tool-call`, `broken-link`, `command`, `documentation`, `environment`, `workflow`, and `other`.

Bare `paper cuts` lists unresolved cuts across every project. Bare `paper history` lists work created today in the local timezone. Record IDs accept any unique prefix in `show` and `resolve`.

## Project Attribution

Inside a Git worktree, Paper records the repository root, normalized origin identity, raw origin URL, branch, commit, and invocation directory relative to the root. Worktrees sharing an origin therefore share a project identity while retaining their exact local context.

Outside Git, Paper uses the current directory name and absolute path. `--project` overrides the displayed project name without discarding the inferred stable identity.

## Vault

The vault is selected in this order:

1. The command's `--vault` option.
2. `PAPER_VAULT`.
3. The path saved by `paper init` in the operating system configuration directory.
4. `~/.paper`.

Records use one file each so parallel agents do not contend over a journal file:

```text
~/.paper/
  work/
    2026/
      08/
        20260804T143012-added-record-parsing-a7c2.md
  cuts/
    20260804T144503-broken-doc-link-f19d.md
```

Each note has typed YAML frontmatter for Obsidian properties and Markdown sections for narrative content. A malformed note fails the whole relevant listing instead of disappearing silently.

The complete behavioral contract is in [`docs/spec.md`](docs/spec.md).

## Development

```sh
go test ./...
go vet ./...
go build ./cmd/paper
```

Tags matching stable semantic versions, such as `v1.0.0`, trigger cross-platform builds for macOS, Linux, and Windows on amd64 and arm64. The release workflow publishes archives, checksums, and generated release notes. GitHub Pages deploys the contents of `site/` from `main`.
