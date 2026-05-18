# Homebrew Tap Setup

Use this guide to configure the `Exigentix/homebrew-tap` repository for `authnet-cli` releases.

The source repository can remain under `EnzanNoMetsuke/authnet-cli`. GoReleaser publishes GitHub release artifacts to the source repository and writes the Homebrew formula to the separate Exigentix tap using the `HOMEBREW_TAP_GITHUB_TOKEN` Actions secret.

## 1. Create the tap repository

1. Open `https://github.com/organizations/Exigentix/repositories/new`.
2. Set Owner to `Exigentix`.
3. Set Repository name to `homebrew-tap`.
4. Set Visibility to `Public` unless this is intentionally internal-only.
5. Initialize with a README if desired.
6. Create the repository.

The expected repository URL is:

```text
https://github.com/Exigentix/homebrew-tap
```

The expected Homebrew tap command is:

```sh
brew tap Exigentix/tap
```

## 2. Create the publisher token

1. Open `https://github.com/settings/personal-access-tokens`.
2. Click Generate new token.
3. Use this token name:

```text
authnet-cli Homebrew tap publisher
```

4. Set Resource owner to `Exigentix`.
5. Set Repository access to Only select repositories.
6. Select `Exigentix/homebrew-tap`.
7. Set Repository permissions:
   - Contents: Read and write
   - Metadata: Read-only, automatic
8. Generate the token.
9. If Exigentix requires token approval or SSO authorization, complete that approval before using the token.

## 3. Add the Actions secret

1. Open `https://github.com/EnzanNoMetsuke/authnet-cli/settings/secrets/actions`.
2. Click New repository secret.
3. Set Name to:

```text
HOMEBREW_TAP_GITHUB_TOKEN
```

4. Paste the fine-grained token as the Secret value.
5. Click Add secret.

## 4. Verify local configuration

From the `authnet-cli` repository:

```sh
rg -n 'owner: Exigentix|name: homebrew-tap|HOMEBREW_TAP_GITHUB_TOKEN' .goreleaser.yaml .github/workflows/release.yml
```

Expected matches:

- `.goreleaser.yaml` sets the Homebrew repository owner to `Exigentix`.
- `.goreleaser.yaml` sets the Homebrew repository name to `homebrew-tap`.
- `.github/workflows/release.yml` exports `HOMEBREW_TAP_GITHUB_TOKEN` for the GoReleaser publish step.

## 5. Verify GitHub access

Confirm the tap exists:

```sh
gh repo view Exigentix/homebrew-tap
```

Confirm the source repository has the secret:

```sh
gh secret list --repo EnzanNoMetsuke/authnet-cli | grep HOMEBREW_TAP_GITHUB_TOKEN
```

## 6. Expected release behavior

On a tagged release, GoReleaser will:

1. Publish release artifacts to `EnzanNoMetsuke/authnet-cli`.
2. Generate checksums and release notes.
3. Commit or update `Formula/authnet.rb` in `Exigentix/homebrew-tap`.
4. Use `HOMEBREW_TAP_GITHUB_TOKEN` for the cross-repository tap write.
