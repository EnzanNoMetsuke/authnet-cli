# authnet-cli

`authnet-cli` is an Authorize.Net operations CLI. The executable is `authnet`.

The project is in a pre-release `0.1.0` read-first alpha. The current goal is a safe operator-first command line tool for inspecting Authorize.Net state across sandbox and production profiles, with stable JSON output for automation and agents.

This is not an SDK wrapper, raw API tunnel, MCP server, or compliance product.

## Current Boundary

The read-first alpha allows explicit production reads and excludes production writes. Sandbox helper commands may contact the real Authorize.Net sandbox gateway for known test-card scenarios, but production mutation commands are not in v1.

Output is redacted by default. The CLI must not write sensitive payment data or customer PII through CLI-controlled persistence.

See the full [project specification](docs/project-spec.md) and [safety model](docs/safety-model.md) before adding new command surfaces.

## Installation State

There is not yet a published package or release artifact. Build from the repository while the alpha is under active development:

```sh
make build
./bin/authnet version
```

GitHub Releases are the canonical release source for v1, with checksums and an initial Homebrew tap path. Until the first tag is published, local builds are the supported install path. See the [release process](docs/release.md) for artifact, checksum, changelog, and Homebrew tap details.

## V1 Command Scope

Canonical v1 command surfaces:

```text
authnet --version
authnet version
authnet paths
authnet config validate
authnet auth test
authnet profile list
authnet profile setup
authnet profile remove
authnet transaction get
authnet transaction list
authnet transaction unsettled list
authnet customer-profile get
authnet customer-profile list
authnet response-code explain
authnet sandbox charge approved
authnet sandbox charge declined
authnet sandbox charge avs
authnet sandbox charge cvv
authnet sandbox charge duplicate
authnet completion ...
```

Production reads must use an explicit production profile. A default profile may exist only for sandbox-classified profiles.

## User Preferences

Non-secret user preferences live in `config.yaml` under the resolved `authnet-cli` config directory shown by `authnet paths`. Supported preferences include default color behavior and transaction list sorting:

```yaml
preferences:
  color: auto
  transaction_list:
    sort_by: timestamp
    sort_order: descending
```

Supported color values are `auto`, `always`, and `never`. Transaction lists support `sort_by` values `timestamp`, `transaction_id`, and `amount`, plus `sort_order` values `ascending` and `descending`. Explicit flags such as `--color=always`, `--no-color`, `--sort-by`, and `--sort-order` override persisted preferences for one invocation. `AUTHNET_TX_SORT_BY` and `AUTHNET_TX_SORT_ORDER` override transaction list preferences when flags are not provided. `--automation` always forces JSON output with no color.

Raw gateway response output is sandbox-only and explicit. Supported commands are:

```sh
authnet --raw-response --profile sandbox-main auth test
authnet --raw-response --profile sandbox-main transaction get TRANSACTION_ID
```

Production profiles and unsupported commands are denied before raw gateway output is emitted.

Sandbox charge helpers are sandbox-only and use test-card aliases instead of raw card-number entry:

```sh
authnet sandbox charge approved --card visa --amount 12.34
authnet sandbox charge avs --variant no-match --amount 12.34
authnet sandbox charge cvv --variant no-match --amount 12.34
authnet sandbox charge duplicate --amount 12.34 --window 120
```

## Local Development

Install the pinned linter and Git hooks:

```sh
make golangci-lint-install
make install-git-hooks
```

Run the full local verification set:

```sh
GOTMPDIR="$PWD/.cache/go-tmp" make verify
```

Run opt-in live sandbox integration tests only with sandbox credentials:

```sh
AUTHNET_API_LOGIN_ID="<sandbox-api-login-id>" \
AUTHNET_TRANSACTION_KEY="<sandbox-transaction-key>" \
GOTMPDIR="$PWD/.cache/go-tmp" \
make test-sandbox-integration
```

Contributor expectations are documented in [CONTRIBUTING.md](CONTRIBUTING.md). Agent and automation usage is documented in [docs/agent-usage.md](docs/agent-usage.md). Sandbox integration CI is documented in [docs/sandbox-integration-ci.md](docs/sandbox-integration-ci.md). Release workflow details are documented in [docs/release.md](docs/release.md). Response-code reference maintenance is documented in [docs/response-code-reference.md](docs/response-code-reference.md).
