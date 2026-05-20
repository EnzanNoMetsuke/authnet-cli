# Release Process

GitHub Releases are the canonical release source for the `0.1.0` read-first alpha.

Release artifacts are built with GoReleaser from signed or otherwise approved Git tags. The v1 release baseline produces native binaries, archives, checksums, and GitHub release notes. It does not require artifact signatures or SBOMs; those are post-v1 release attestations.

This document covers the release workflow for the local repository, the GitHub
release, and the Homebrew tap. It also names the files that control release
metadata so version, schema, and contract-status changes are made deliberately.

## Relevant Files

- `docs/release.md`: release workflow and operator-facing release checklist
- `docs/homebrew-tap.md`: Homebrew tap setup and token configuration
- `.goreleaser.yaml`: tagged release build, archive, GitHub release, and Homebrew cask configuration
- `Makefile`: local build defaults, release dry-run targets, and verification targets
- `internal/cli/build.go`: fallback version metadata used when no linker flags are injected
- `cmd/authnet/main.go`: package-level build metadata variables populated by linker flags
- `internal/cli/commands.go`: `authnet version` output shape
- `internal/cli/root_test.go`: version and JSON output expectations
- `.github/workflows/release.yml`: tag-triggered and manual release workflow, if present

If a listed file does not exist in the checkout, treat that as a release-readiness
gap before publishing. In particular, the GitHub Actions workflow is required
for automated tagged releases, while local `make release-snapshot` can still
exercise GoReleaser configuration without publishing.

## Artifacts

The release workflow builds `authnet` for:

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`
- `windows/amd64`
- `windows/arm64`

Each release includes:

- Platform archives containing the `authnet` binary and core project docs
- `checksums.txt`
- GitHub release notes generated from commit history

## Version Metadata

Release builds set local-only version metadata through linker flags:

- CLI version from the Git tag
- Git commit
- Build date
- JSON schema version
- Contract status

The CLI version, schema version, and contract status are related but distinct:

- CLI version answers "what build or release is this?"
- JSON schema version answers "what output contract schema is this?"
- Contract status answers "how stable should operators treat that contract?"

For example, a release can report CLI version `0.1.0`, schema version `0.1.0`,
and contract status `pre-release`. Do not infer contract status from the Git
tag or semantic version. Change it explicitly as release policy.

Metadata sources:

- `.goreleaser.yaml` injects release metadata with `-X main.version={{ .Version }}`, `-X main.commit={{ .Commit }}`, `-X main.date={{ .Date }}`, `-X main.schemaVersion=...`, and `-X main.contractStatus=...`.
- `Makefile` injects local build metadata through `VERSION`, `COMMIT`, `BUILD_DATE`, `SCHEMA_VERSION`, and `CONTRACT_STATUS`.
- `internal/cli/build.go` provides fallback defaults when a binary is built without linker flags.
- `cmd/authnet/main.go` defines the variables that linker flags populate and passes them into the CLI package.

Current local defaults are controlled by:

```make
VERSION ?= 0.0.0-dev
SCHEMA_VERSION ?= 0.2.0
CONTRACT_STATUS ?= alpha
```

GoReleaser release builds derive `main.version` from the pushed Git tag through
`{{ .Version }}`. The local `Makefile` default version and the no-linker
fallback in `internal/cli/build.go` are intentionally `0.0.0-dev` unless
overridden, so non-release builds remain visibly separate from tagged release
builds. The schema version and contract status are currently hardcoded in
`.goreleaser.yaml`, so changing either one for a release requires a code/config
change before tagging.

Verify the metadata with:

```sh
make build
./bin/authnet --version
./bin/authnet version
./bin/authnet --json version
```

Version reporting must not perform update checks or any other network calls.

## Changing Contract Status

Contract status is a manual release-policy value. It is not derived from the Git
tag. For example, to change the contract status from `pre-release` to `alpha`,
update the release config and matching local/fallback defaults together.

Required release-build change in `.goreleaser.yaml`:

```yaml
- -X main.contractStatus=alpha
```

Recommended local-build change in `Makefile`:

```make
CONTRACT_STATUS ?= alpha
```

Recommended no-linker fallback change in `internal/cli/build.go`:

```go
defaultContractStatus = "alpha"
```

After changing contract status, verify all version output forms:

```sh
GOTMPDIR="$PWD/.cache/go-tmp" make verify
make release-check
make release-snapshot
./bin/authnet --version
./bin/authnet version
./bin/authnet --json version
```

If the contract status appears in tests, update the expectations in
`internal/cli/root_test.go` in the same change. Keep release metadata changes in
the same commit as their tests and release-doc updates.

## Changing Schema Version

Schema version is also explicit release metadata. Change it only when the JSON
contract changes in a way that should be visible to automation consumers.

Update all relevant locations together:

- `.goreleaser.yaml`: `-X main.schemaVersion=...`
- `Makefile`: `SCHEMA_VERSION ?= ...`
- `internal/cli/build.go`: `defaultSchemaVersion = "..."`
- `internal/cli/root_test.go`: version JSON and text-output expectations
- Any docs that describe the JSON envelope or schema contract

Do not change schema version just because the CLI binary version changes. A bug
fix or packaging-only release may keep the same schema version.

## Dry Run

Validate the release configuration before tagging:

```sh
make release-check
make release-snapshot
```

The GitHub Actions release workflow also supports a manual dry run through `workflow_dispatch`.

## Publishing

Publish by pushing a version tag:

```sh
git tag v0.1.0
git push origin v0.1.0
```

Do not push tags from an agent workflow unless explicitly instructed.

## Homebrew Tap

The initial convenience install channel is a separate Homebrew tap repository:

```text
Exigentix/homebrew-tap
```

The source repository can remain under `EnzanNoMetsuke/authnet-cli`; only the tap repository needs to live under the Exigentix organization.

Before the first published release, create `Exigentix/homebrew-tap` and add a `HOMEBREW_TAP_GITHUB_TOKEN` Actions secret to this repository with permission to write to the tap. See [Homebrew Tap Setup](homebrew-tap.md) for the step-by-step configuration guide.

GoReleaser will then publish a Homebrew cask at `Casks/authnet.rb` during tagged releases. This matches GoReleaser's current guidance for binary releases.

After the first release, install with:

```sh
brew tap Exigentix/tap
brew install --cask authnet
authnet --version
```

If the tap ever contains an older `Formula/authnet.rb`, remove that formula and add a root-level `tap_migrations.json` entry before publishing the cask:

```json
{
  "authnet": "authnet"
}
```

No formula-to-cask cleanup is required before the first published release if the tap is still empty.

## Full Release Workflow

1. Confirm the release scope.
   Review the Linear `authnet-cli` project, the target milestone, and the
   intended release type. Confirm whether the release is still `pre-release` or
   whether the contract status should change, such as to `alpha`.

2. Confirm the working tree and branch.
   Run:

   ```sh
   git status --short
   git branch --show-current
   ```

   Do not publish from a dirty or unexpected branch. Preserve unrelated local
   changes and resolve them before release work.

3. Review release metadata policy.
   Decide the CLI version tag, schema version, and contract status separately.
   For a `v0.1.0` tag, GoReleaser injects CLI version `0.1.0`; it does not
   decide schema version or contract status. Confirm `Makefile` and
   `internal/cli/build.go` still use the intended local/fallback version policy,
   usually `0.0.0-dev`, unless a release-prep change explicitly overrides it.

4. Update metadata files when policy changes.
   If contract status or schema version changes, update `.goreleaser.yaml`,
   `Makefile`, `internal/cli/build.go`, and relevant tests/docs together. If
   only the CLI release version changes, normally do not edit source files; use
   the Git tag for the release version while keeping local/fallback builds on
   `0.0.0-dev`.

5. Confirm the Homebrew tap setup before the first release.
   Ensure `Exigentix/homebrew-tap` exists and this repository has a
   `HOMEBREW_TAP_GITHUB_TOKEN` Actions secret with write permission to that tap.
   Follow `docs/homebrew-tap.md` for the complete setup.

6. Run full local verification with repo-local caches.
   Run:

   ```sh
   GOTMPDIR="$PWD/.cache/go-tmp" make verify
   ```

   The `verify` target formats, tests, vets, builds, checks docs, and runs the
   configured linter. Fix failures before continuing.

7. Run GoReleaser configuration checks.
   Run:

   ```sh
   make release-check
   ```

   This validates the GoReleaser configuration without building or publishing a
   release.

8. Build a local snapshot release.
   Run:

   ```sh
   make release-snapshot
   ```

   Inspect `dist/` for expected platform archives and `checksums.txt`. Snapshot
   builds are local validation artifacts, not the canonical release.

9. Verify local version output.
   Run:

   ```sh
   ./bin/authnet --version
   ./bin/authnet version
   ./bin/authnet --json version
   ```

   Confirm the CLI version, schema version, contract status, commit, and build
   date are expected for the local build. Version commands must stay local-only
   and must not perform update checks or network calls.

10. Commit release-prep changes, if any.
    If metadata, docs, tests, or release configuration changed, commit them
    before tagging. Use a concise conventional commit and include related Linear
    issue IDs when applicable.

11. Create the version tag locally.
    Use an approved signed or otherwise approved tag:

    ```sh
    git tag v0.1.0
    ```

    Replace `v0.1.0` with the intended release version. Confirm the tag points
    at the exact commit that passed verification.

12. Publish the tag only when ready.
    Run:

    ```sh
    git push origin v0.1.0
    ```

    Do not push tags from an agent workflow unless explicitly instructed. The
    pushed tag is the release trigger.

13. Monitor the GitHub Actions release workflow.
    Confirm GoReleaser builds the expected targets, publishes the GitHub
    Release, uploads checksums, and updates the Homebrew cask. If the workflow
    fails after a tag push, fix the cause before replacing or recreating release
    artifacts.

14. Validate the published GitHub Release.
    Confirm the release contains all expected platform archives, `checksums.txt`,
    and generated release notes. Download at least one archive and run
    `authnet --version` and `authnet version` against the published binary.

15. Validate Homebrew installation.
    After the tap update lands, run:

    ```sh
    brew tap Exigentix/tap
    brew install --cask authnet
    authnet --version
    ```

    Confirm the installed binary reports the release version and expected
    contract metadata.

16. Update tracker and release notes.
    Update the related Linear issue or milestone with the published release URL,
    verification notes, and any manual installation checks. Mark any follow-up
    release-attestation work, such as signatures or SBOMs, as post-v1 unless the
    release scope changed.
