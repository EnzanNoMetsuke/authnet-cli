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

### Triage labels

Use the default five-label triage vocabulary: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, and `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

This is a single-context repo with root `CONTEXT.md` and root `docs/adr/`. See `docs/agents/domain.md`.
