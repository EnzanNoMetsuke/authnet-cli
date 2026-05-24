# Safety Model

This document defines the v1 safety and privacy boundary for `authnet`.

`authnet` reduces payment-data and customer-data handling risk, but it does not provide a PCI-DSS or other compliance guarantee. Operators remain responsible for their own compliance obligations.

## Production Behavior

V1 is a read-first release:

- Production reads are allowed only when the operator selects a production-classified profile explicitly.
- Production writes are excluded from v1.
- A production write is any command that mutates production Authorize.Net state, including transaction actions, customer profile changes, subscription changes, webhook changes, fraud-review actions, and other gateway-side mutations.
- Sandbox writes are limited to sandbox test helpers for known card transaction scenarios.

Color can reinforce production markers, but it must never be the only cue. Production-classified output must have textual production markers where profile context is shown.

## Redaction And Persistence

Redacted output is the default in every environment. Curated command output must be built from safe normalized results rather than raw gateway responses.

The CLI must never write sensitive payment data or customer PII through CLI-controlled persistence. This includes:

- Logs
- Caches
- Traces
- Fixtures
- Diagnostic bundles
- Crash artifacts
- Any other file written by the CLI

V1 has no Diagnostic logs. `authnet paths` must communicate that v1 has no CLI-controlled sensitive-data persistence paths.

## Network Boundary

V1 must make no network calls other than requests to the selected Authorize.Net environment.

The CLI must not perform:

- Telemetry
- Crash reporting
- Update checks
- Live documentation fetching
- External error reporting
- Other non-Authorize.Net network calls

`authnet version`, `authnet --version`, `authnet paths`, and `authnet config validate` are local-only commands.

## Raw Response Mode

Raw response mode is sandbox-only and explicit. It is never available for production-classified profiles.

Operators and agents must not request raw production output. If a command would expose raw production gateway data, it must fail before contacting the production environment.

Supported raw-response commands:

```text
authnet auth test
authnet transaction get TRANSACTION_ID
```

Commands outside that list fail clearly when `--raw-response` is provided. JSON raw-response output keeps the standard envelope, sets `redacted` to `false`, and places the unredacted gateway object in `data.raw_gateway_response`.

If raw response mode runs while `preferences.json: never` or `preferences.automation: never` is configured, the CLI warns that those durable preferences are ignored so the raw gateway JSON can be presented accurately. Bare raw-response mode keeps stdout as the raw gateway JSON and writes warnings to stderr.

## Automation Boundary

`--automation` implies JSON output, no color, no prompts, no TUI or wizard behavior, structured failures on stdout, and the exit-code taxonomy.

Automation should rely on the JSON envelope rather than human-readable text. See [docs/agent-usage.md](agent-usage.md) for automation guidance.
