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

`authnet sandbox ...` commands are sandbox helper surfaces. They may contact the real Authorize.Net sandbox gateway when implemented.

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
