# Agent Instructions

## Project identity

When a human contributor says "this project", treat it as the `authnet-cli` Linear project unless they explicitly mean the local repository.

- Linear project: `authnet-cli`
- Linear project ID: `e43aaf38-b126-4fc2-8c49-d89d67154ac6`
- Linear URL: https://linear.app/lynyx/project/authnet-cli-55182454a0e9
- Linear team: `Lynyx` (`LYN`)
- Local repo/project name: `authnet-cli`
- CLI executable name: `authnet`

## Agent skills

### Issue tracker

Issues live in Linear; use the repo's Linear workflow rather than GitHub, GitLab, or local markdown. See `docs/agents/issue-tracker.md`.

### Project workflow

Follow the mandatory issue, verification, commit, and tracker-update workflow in `docs/agents/workflow.md`.

### Linting

Use the project linting workflow in `docs/agents/linting.md`.

### Go test execution

Run Go tests with repo-local caches from the start. Do not run bare `go test` in this repo.

Use the Makefile targets when possible:

```sh
GOTMPDIR="$PWD/.cache/go-tmp" make verify
make test-sandbox-integration
```

For focused `go test` commands, use repo-local caches explicitly:

```sh
GOTMPDIR="$PWD/.cache/go-tmp" GOCACHE="$PWD/.cache/go-build" GOMODCACHE="$PWD/.cache/go-mod" go test ./internal/cli -run '<test-pattern>'
```

Tests that use `httptest` or live sandbox integration require local networking. Run those with the same repo-local cache settings and request sandbox/network escalation immediately instead of first trying a restricted run.

### Upstream MCP access

Use the `mcpproxy` MCP server to discover and call upstream MCP servers. First call `retrieve_tools` for the upstream capability you need, then call the exact returned tool name with `call_tool_read`, `call_tool_write`, or `call_tool_destructive` according to the operation. Include a clear intent reason and data-sensitivity classification.

Do not use the `mcpproxy` CLI for upstream MCP access unless the `mcpproxy` MCP tools are completely unavailable in the current session.

### Triage labels

Use the default five-label triage vocabulary: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, and `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

This is a single-context repo with root `CONTEXT.md` and root `docs/adr/`. See `docs/agents/domain.md`.
