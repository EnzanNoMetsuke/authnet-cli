# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Layout

This is a single-context repo.

Before exploring, read:

- `CONTEXT.md` at the repo root
- Relevant ADRs under `docs/adr/`

If either path does not exist in a future checkout, proceed silently. Do not suggest creating domain docs upfront. The producer skill (`grill-with-docs`) creates them lazily when terms or decisions actually get resolved.

## Use The Glossary's Vocabulary

When output names a domain concept in an issue title, refactor proposal, hypothesis, test name, or implementation plan, use the term as defined in `CONTEXT.md`.

Do not drift to synonyms the glossary explicitly avoids.

If the concept you need is not in the glossary yet, either reconsider whether you are inventing language the project does not use, or note the gap for `grill-with-docs`.

## ADRs

Read ADRs that touch the area you are about to work in.

If your output contradicts an existing ADR, surface the conflict explicitly rather than silently overriding it.
