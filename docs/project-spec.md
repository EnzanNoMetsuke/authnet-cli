# authnet-cli Project Specification

## Summary

`authnet-cli` is an Authorize.Net operations CLI. The executable is `authnet`.

The product is a safe, operator-first command-line tool for inspecting and carefully operating Authorize.Net merchant data and gateway actions across sandbox and production environments. It is not a generic SDK wrapper, raw API tunnel, MCP server, or compliance product.

v1 is a read-first release. It allows explicit production reads, excludes production writes, and includes sandbox-only test helpers that contact the real Authorize.Net sandbox gateway.

## Product Boundary

The canonical product is an **Authorize.Net operations CLI**.

The primary v1 user is the **Operator**: a person using the CLI to inspect or perform Authorize.Net gateway actions with direct accountability for the result. Automation, CI, and agents are supported through stable machine-readable output and predictable execution modes, but the command model should remain understandable to human operators first.

The repository/project name is `authnet-cli`. The executable name is `authnet`.

The project license is MIT.

## V1 Scope

v1 is a **Read-first release**:

- Production-facing commands may inspect Authorize.Net state.
- Production writes are excluded.
- Sandbox writes are limited to test helpers for known card transaction scenarios.
- Full sandbox resource management is deferred to the immediate follow-up surface.

V1 command families:

- Authentication testing
- Profile setup, listing, removal, and local config validation
- Path inspection
- Version reporting
- Transaction inspection
- Customer profile inspection
- Response-code explanation
- Sandbox card test helpers
- Shell completion

V1 documentation and release deliverables:

- Operator-facing `README.md`
- `docs/project-spec.md`
- `docs/safety-model.md`
- Safety-focused `CONTRIBUTING.md`
- Agent usage guide
- Canonical GitHub release artifacts with checksums
- Initial Homebrew tap as the first convenience install channel

## Implementation Decisions

The CLI will be implemented in Go. See [ADR 0001](./adr/0001-use-go-for-the-cli.md).

Use Cobra for command structure. Use the current Authorize.Net JSON API directly over HTTP with typed request and response models. Do not use legacy SOAP, AIM, SIM, or DPM APIs.

Use the Charmbracelet ecosystem for output and future TUI readiness:

- Use `lipgloss/table` for simple non-interactive tables.
- Structure output code so `bubbletea` and `bubbles/table` can support future interactive TUI workflows.
- Do not commit to a general interactive TUI in v1 unless a specific v1 workflow proves it needs interaction.

Curated command output must be built from safe normalized result models, not raw gateway response maps. Redaction must not live only in renderers.

## Profiles And Credentials

Profiles are named access targets. Every profile has:

- A visible, operator-defined profile name
- Exactly one environment classification: `sandbox` or `production`
- A credential source

Profile names are visible in normal output and JSON envelopes. Documentation must warn operators not to put secrets, customer PII, client names, or sensitive merchant labels in profile names.

Credentials:

- Humans may use environment variables or secure local storage.
- Automation may use environment variables or injected secrets.
- The CLI must not create plaintext credential storage.
- Profile config stores non-secret metadata and credential-source references only.

Profile config:

- Store non-secret profile config in the OS user config directory.
- Use `authnet-cli` as the subdirectory under `os.UserConfigDir()`.
- Provide `authnet paths` to surface the resolved config directory.
- Allow environment overrides for automation and one-off use.

Defaults:

- A default profile may exist only if it is sandbox-classified.
- Production profiles must always be selected explicitly.
- Automation mode may use a sandbox default profile, but must not silently inherit production.

Profile commands:

- Include profile setup.
- Support both interactive setup and non-interactive setup.
- Include profile listing with non-secret metadata.
- Include local profile removal.
- Exclude profile export/import from v1.

Production profiles must be clearly marked with a textual production marker. Color may reinforce the marker but must not be the only cue.

## Safety And Privacy Model

V1 allows explicit production reads and excludes production writes.

A **production write** is any command that mutates production Authorize.Net state. This includes money-moving actions and non-financial mutations such as customer profile changes, webhook configuration changes, fraud-review actions, and subscription changes.

The CLI must never write sensitive payment data or customer PII through CLI-controlled persistence. This applies to logs, caches, traces, fixtures, diagnostic bundles, crash artifacts, and any file written by the CLI.

V1 must not create diagnostic logs.

V1 must make no network calls other than requests to the selected Authorize.Net environment. No update checks, docs fetches, telemetry, crash reporting, or external error reporting are allowed in v1.

V1 includes no telemetry.

The CLI does not make a PCI-DSS or other compliance guarantee. It is designed to reduce payment-data and PII handling risk, but operators remain responsible for their compliance obligations.

Raw response mode:

- Available only for sandbox-classified profiles.
- Explicit only.
- Never available for production profiles.

Dry-run mode:

- Excluded from v1.
- Deferred until mutation commands exist.

Future production-write safety:

- A safety policy decides whether a production write is allowed.
- Confirmation acknowledges an allowed high-risk action in interactive use.
- Non-interactive execution of high-risk actions requires safety-policy permission, not just a confirmation flag.
- The concrete safety policy mechanism is deferred until production writes are designed.

## Output Contract

The CLI defaults to human-readable output for interactive operators.

Automation relies on the stable JSON contract selected by explicit JSON output. Do not auto-switch to JSON based on stdout TTY detection.

Global output flags:

- `--json` selects the stable JSON contract.
- `--automation` implies JSON output, no color, no prompts, no TUI or wizard behavior, structured failures on stdout, and the exit-code taxonomy.
- `--color=auto|always|never` controls ANSI color.
- `--no-color` is an alias for `--color=never`.

JSON is uncolored by default. Human operators may explicitly request colored JSON, for example with `--json --color=always`. Automation mode always disables color.

JSON envelope requirements:

- Every JSON response uses a common envelope.
- Every envelope includes `schema_version`.
- Structured failures are written to stdout in JSON mode.
- Exit codes still indicate success or failure.
- Structured warnings are first-class envelope fields.
- Command-specific data lives under a command-specific data field.
- The envelope must indicate profile name, environment classification, command identity, redaction status where relevant, warnings, and normalized errors.

Suggested exit-code taxonomy:

- `0`: success
- `1`: general failure
- `2`: usage or configuration error
- `3`: authentication or authorization failure
- `4`: gateway or API failure
- `5`: safety policy denied
- `6`: not found
- `7`: unavailable or network timeout

## Command Surface

Canonical command names should use formal Authorize.Net terminology. Shorthand aliases may be considered later, but v1 docs should use canonical names.

### Core Commands

Required v1 command surfaces:

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
authnet sandbox ...
authnet completion ...
```

`authnet doctor` is excluded from v1 and reserved for a future version if concrete support workflows justify it.

`authnet config validate`:

- Runs locally.
- Does not contact Authorize.Net.
- Checks non-secret profile config shape.
- Checks credential-source availability.
- Does not print secret values.

`authnet auth test`:

- Contacts the selected Authorize.Net environment.
- Validates profile access.
- Confirms safe profile identity context.
- Must not display secrets, raw auth responses, or sensitive account details.

`authnet paths`:

- Shows the resolved config directory.
- For v1, should explicitly communicate that the CLI has no sensitive-data persistence paths.

`authnet version` and `authnet --version`:

- Must be local only.
- Must not check for updates.
- The command form should support richer human output and JSON.
- The flag form may print a concise version string.

### Transactions

Transaction commands distinguish:

- Settled transaction history
- Unsettled transaction set

Transaction lookup returns a normalized transaction view:

- Stable operational fields
- Safe payment summary
- Status and response state
- Settlement state where available
- Customer/profile references where safe
- Allowlisted redacted gateway-specific detail sections

List-style transaction commands use bounded pagination. They should not fetch every matching production record by default.

Time-based transaction lists:

- Accept absolute time ranges.
- Accept relative ranges such as `--last 7d`.
- Interpret date-only or relative input using operator-local time unless a timezone is supplied.
- Emit resolved timestamps with explicit timezone offsets in JSON.

### Customer Profiles

Customer profile commands are read-only in v1.

Default customer-profile output should show customer profile metadata only. Nested payment profiles and shipping address details require explicit selection or expansion and still must be redacted.

### Response-Code Explanation

`authnet response-code explain` explains Authorize.Net gateway and API codes for operator debugging.

It should cover code families such as:

- Transaction response
- API message
- Validation
- AVS
- CVV

Use a local curated response-code reference for v1. Do not fetch docs live. The reference must include a maintenance story for keeping it current against official Authorize.Net sources.

Output must separate:

- Code family
- Source meaning
- Likely causes
- Project-authored recommended next steps
- Source links
- Reference version or review date

### Sandbox Test Helpers

V1 sandbox test helpers contact the real Authorize.Net sandbox gateway. Offline simulation is test-only and must not appear as product-facing gateway behavior.

V1 sandbox-helper coverage:

- Card authorization/capture success
- Card authorization/capture decline
- AVS variants
- CVV variants
- Duplicate-window behavior

V1 does not include eCheck or partial authorization helpers, but both are future payment test scenarios.

Sandbox helpers should use documented test-card aliases rather than encouraging operators to type raw card numbers.

V1 sandbox helper command surface:

```text
authnet sandbox charge approved
authnet sandbox charge declined
authnet sandbox charge avs --variant <variant>
authnet sandbox charge cvv --variant <variant>
authnet sandbox charge duplicate
```

All sandbox charge scenarios accept `--card <alias>` and `--amount <decimal>`. Supported card aliases are `visa`, `mastercard`, `amex`, and `discover`; the default alias is `visa` unless a scenario requires a different compatible card. The CLI must not expose a normal product-facing raw card-number flag.

Supported AVS variants are `match`, `no-match`, `zip-match`, `address-match`, and `unavailable`. Supported CVV variants are `match`, `no-match`, `not-processed`, `should-be-present`, and `issuer-unavailable`.

Duplicate-window testing uses `authnet sandbox charge duplicate --window <seconds>`, submits two equivalent sandbox `authCaptureTransaction` requests with the Authorize.Net `duplicateWindow` transaction setting, and reports both attempts safely.

## Release And Distribution

GitHub Releases are the canonical release source.

V1 canonical release artifacts:

- Cross-platform native binaries
- Checksums
- Changelog or release notes

V1 initial convenience install channel:

- Separate Homebrew tap repository: `Exigentix/homebrew-tap`

Post-v1 release attestations:

- Artifact signatures
- SBOMs

Additional future channels may include Scoop, WinGet, `.deb`, `.rpm`, Docker, npm wrapper, and MCP registry distribution if later surfaces justify them.

## Testing Strategy

Default tests:

- Unit and mocked tests only.
- No required Authorize.Net network access.
- No real merchant data.

Sandbox integration tests:

- Opt-in only.
- Contact the real Authorize.Net sandbox gateway.
- Require explicit credentials.
- Not required pull-request gates.
- May run manually or on a schedule in CI.

Required regression coverage:

- JSON golden tests for the stable JSON contract
- Selective human-output tests for important operator-facing behavior
- Mandatory redaction tests with synthetic sentinel values

Redaction tests must prove synthetic sensitive values do not appear in forbidden surfaces, including human output, JSON output, warnings, errors, and CLI-controlled persistence boundaries.

Fixtures must be synthetic only. Do not use real merchant responses, customer records, cardholder data, or production payloads.

## Documentation Requirements

V1 docs must include:

- Operator-facing README with pre-release status when appropriate
- Full project specification
- Safety model document
- Agent usage guide
- Safety-focused contribution guide
- Command help via Cobra

V1 docs must make these constraints clear:

- Production writes are out of v1.
- Production reads are explicit and redacted by default.
- Sensitive payment data and customer PII are never written through CLI-controlled persistence.
- No diagnostic logs in v1.
- No telemetry or crash reporting in v1.
- No external network calls other than selected Authorize.Net environment requests.
- Raw response mode is sandbox-only.
- Automation mode behavior is predictable and non-interactive.
- The CLI reduces handling risk but does not guarantee PCI-DSS or other compliance.

V1 does not include man pages.

## Agent Support

V1 includes a repo-local agent usage guide, not a full installable agent skill.

The agent usage guide must tell automation and agents to:

- Prefer `--automation`.
- Use explicit production profiles.
- Treat production reads as sensitive.
- Never request raw production output.
- Never persist CLI output containing customer or payment data.
- Rely on the JSON envelope and exit-code taxonomy.

A full installable agent skill follows after the stable JSON contract settles.

## Future Scope

These are deferred surfaces, not non-goals:

- Full sandbox resource management
- Production writes
- Concrete safety policy mechanism
- Dry-run for mutation commands
- Webhook tooling
- Recurring billing support
- Subscription mutation
- eCheck sandbox helpers
- Partial authorization sandbox helpers
- Telemetry, if explicitly opt-in and approved later
- Update checks, if explicitly opt-in and approved later
- Profile export/import
- `authnet doctor`
- Man pages
- Dynamic profile completion
- Interactive TUI workflows
- Installable agent skill
- Release signatures and SBOMs
- Additional package channels

Full sandbox resource management should be the immediate follow-up surface after v1. It covers gateway-side sandbox objects such as customer profiles, payment profiles, subscriptions, and generated transaction fixtures. Local webhook receiver/testing infrastructure remains separate webhook tooling.

Recurring billing future vocabulary:

- Recurring billing
- Subscription
- Payment schedule
- Subscription status
- Subscription payment

Recurring billing support should start with read-only subscription inspection before subscription mutation.

## Non-Goals

True non-goals are capabilities outside the intended product boundary:

- Generic Authorize.Net SDK wrapper behavior
- Raw production gateway passthrough
- Legacy Authorize.Net SOAP, AIM, SIM, or DPM protocol support
- PCI-DSS or other compliance guarantee

Do not list deferred future surfaces as non-goals.

## Open Questions

These should be resolved during implementation planning:

- Exact JSON envelope field names and schema-versioning policy
- Exact config file format
- Exact keychain backend/library
- Exact response-code reference source/update workflow
- Exact bounded-pagination defaults and maximums
- Exact profile setup command syntax
- Exact CI workflow layout for sandbox integration tests
