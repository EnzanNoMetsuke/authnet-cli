# Linting

This project uses `golangci-lint` for Go static analysis.

## Setup

Install the pinned project linter binary into `./bin`:

```sh
make golangci-lint-install
```

Install the tracked Git hooks:

```sh
make install-git-hooks
```

The hook installer sets `core.hooksPath` to `.githooks`. The pre-commit hook runs:

```sh
make golangci-lint-full
```

## Commands

Run the full lint suite:

```sh
make golangci-lint-full
```

Run the standard local verification set:

```sh
make verify
```

## Enabled Linters

The lint configuration enables:

- `govet`
- `staticcheck`
- `revive`
- `errcheck`
- `ineffassign`
- `unused`
- `gosec`
