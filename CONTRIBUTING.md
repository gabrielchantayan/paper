# Contributing

Paper is intentionally small: one local binary, one Markdown vault, and no service dependency. Changes should preserve that shape unless the requirement cannot be expressed cleanly within it.

## Development

Paper requires Go 1.25 or newer.

```sh
go test -race ./...
go vet ./...
go build ./cmd/paper
```

Tests that inspect user-level configuration must set an isolated `HOME`, configuration root, and `PAPER_VAULT`. Never write into a developer's actual vault or agent directories.

## Pull Requests

Explain the observable behavior, why the chosen design belongs in Paper, and the commands used to verify it. Include regression tests for fixes and update adjacent documentation when command behavior or record formats change.
