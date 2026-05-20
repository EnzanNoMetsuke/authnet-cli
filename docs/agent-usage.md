# Agent Usage Guide

This guide is for automation, CI, and coding agents using `authnet`.

Agents should treat Authorize.Net output as sensitive operational data even when the CLI redacts known sensitive fields. Do not persist command output that may contain customer or payment context unless the human operator explicitly instructs you to and the destination is approved for that data.

## Default Invocation

Prefer automation mode:

```sh
authnet --automation version
```

`--automation` implies:

- JSON output
- No color
- No prompts
- No interactive behavior
- Structured failures on stdout
- Stable exit codes

Use the JSON envelope and exit-code taxonomy for control flow. Do not parse human-readable output in automation.

## Preferences

Durable user preferences are non-secret config only. The supported preference in this release is `preferences.color` in `config.yaml`, with values `auto`, `always`, or `never`.

Precedence is: explicit command-line flags, automation safety overrides, environment overrides such as `AUTHNET_COLOR`, durable preferences, then built-in defaults. Automation should still prefer `--automation`; it forces JSON output and no color regardless of persisted preferences.

## Profiles

Use explicit production profiles:

```sh
authnet --automation --profile prod-main transaction get 1234567890
```

Never rely on a default profile for production. A default profile may exist only for sandbox-classified profiles.

Do not put secrets, customer PII, client names, or sensitive merchant labels in profile names. Profile names are visible in normal output and JSON envelopes.

Useful profile and config commands:

```text
authnet profile setup
authnet profile list
authnet profile remove
authnet config validate
authnet paths
```

## Production Reads

Production reads are permitted in v1 only through explicit production profiles. Treat their output as sensitive even when redacted.

Examples of production-read command surfaces:

```text
authnet transaction get
authnet transaction list
authnet transaction unsettled list
authnet customer-profile get
authnet customer-profile list
authnet auth test
```

Avoid broad production reads. Use bounded pagination and narrow time ranges where command options allow it.

## Raw Output

Never request raw production output. Raw response mode is sandbox-only:

**NOTE:** The current CLI enforces the sandbox-only `--raw-response` safety gate, but command-specific raw gateway response emission has not landed yet.

```sh
authnet --automation --raw-response --profile sandbox-main version
```

Sandbox raw output can still contain sensitive-looking test data. Do not save it unless that is necessary for the task and the destination is appropriate.

## Local And Reference Commands

Local-only commands:

```text
authnet --version
authnet version
authnet paths
authnet config validate
authnet response-code explain
authnet completion ...
```

`authnet response-code explain` uses the checked-in curated reference. It does not fetch live documentation. See [docs/response-code-reference.md](response-code-reference.md).

`authnet sandbox charge ...` commands are sandbox helper surfaces. They contact the real Authorize.Net sandbox gateway and must use sandbox-classified profiles.

Sandbox charge helpers use test-card aliases instead of raw card numbers:

```sh
authnet sandbox charge approved --card visa --amount 12.34
authnet sandbox charge declined --card visa --amount 12.34
authnet sandbox charge avs --variant no-match --amount 12.34
authnet sandbox charge cvv --variant no-match --amount 12.34
authnet sandbox charge duplicate --amount 12.34 --window 120
```

Agents must not persist full request payloads from sandbox charge helpers because request construction includes sandbox card numbers and CVV/card-code trigger values.

## Exit Codes

Use these exit-code categories:

```text
0 success
1 general failure
2 usage or configuration error
3 authentication or authorization failure
4 gateway or API failure
5 safety policy denied
6 not found
7 unavailable or network timeout
```

Structured failures are represented inside the JSON envelope in JSON and automation modes.
