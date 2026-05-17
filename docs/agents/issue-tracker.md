# Issue Tracker: Linear

Issues and implementation work for this repo live in Linear.

## Workflow

- Use Linear as the source of truth for issues, tasks, and triage state.
- Do not create GitHub, GitLab, or local markdown issues for this repo unless the user explicitly asks.
- When a skill says "publish to the issue tracker", create or update the appropriate Linear issue.
- When a skill says "fetch the relevant ticket", read the referenced Linear issue.

## Expected Issue Content

Linear issues should include enough context for an AFK agent or human implementer to act without reconstructing the whole conversation:

- Clear title using the project's domain vocabulary
- Problem or goal
- Scope and non-scope
- Acceptance criteria
- Relevant docs, ADRs, or files
- Current triage label or state

## Tooling

Prefer the available Linear connector or Linear skill when operating on issues. If Linear tooling is unavailable in a session, ask the user how they want to proceed rather than silently falling back to another tracker.
