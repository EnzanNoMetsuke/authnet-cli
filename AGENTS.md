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

### Upstream MCP access

Use the `mcpproxy` MCP server to discover and call upstream MCP servers. First call `retrieve_tools` for the upstream capability you need, then call the exact returned tool name with `call_tool_read`, `call_tool_write`, or `call_tool_destructive` according to the operation. Include a clear intent reason and data-sensitivity classification.

Do not use the `mcpproxy` CLI for upstream MCP access unless the `mcpproxy` MCP tools are completely unavailable in the current session.

### Triage labels

Use the default five-label triage vocabulary: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, and `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

This is a single-context repo with root `CONTEXT.md` and root `docs/adr/`. See `docs/agents/domain.md`.
