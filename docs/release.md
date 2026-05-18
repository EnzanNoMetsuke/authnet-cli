# Release Process

GitHub Releases are the canonical release source for the `0.1.0` read-first alpha.

Release artifacts are built with GoReleaser from signed or otherwise approved Git tags. The v1 release baseline produces native binaries, archives, checksums, and GitHub release notes. It does not require artifact signatures or SBOMs; those are post-v1 release attestations.

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

Verify the metadata with:

```sh
make build
./bin/authnet --version
./bin/authnet version
./bin/authnet --json version
```

Version reporting must not perform update checks or any other network calls.

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

Before the first published release, create `Exigentix/homebrew-tap` and add a `HOMEBREW_TAP_GITHUB_TOKEN` Actions secret to this repository with permission to write to the tap. GoReleaser will then publish `Formula/authnet.rb` during tagged releases.

After the first release, install with:

```sh
brew tap Exigentix/tap
brew install authnet
authnet --version
```
