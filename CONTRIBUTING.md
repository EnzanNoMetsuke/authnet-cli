# Contributing

`authnet-cli` is a payment-operations project. Changes should preserve the read-first v1 boundary, avoid unnecessary complexity, and keep operator safety visible in code, tests, and docs.

Read [docs/project-spec.md](docs/project-spec.md), [docs/safety-model.md](docs/safety-model.md), and [CONTEXT.md](CONTEXT.md) before changing command behavior.

## Verification

Install local tooling:

```sh
make golangci-lint-install
make install-git-hooks
```

Run the full verification set before committing:

```sh
GOTMPDIR="$PWD/.cache/go-tmp" make verify
```

`make verify` runs formatting, tests, vet, build, docs checks, and the pinned `golangci-lint` configuration.

## Fixtures And Test Data

Use synthetic fixtures only. Do not commit real merchant responses, customer records, cardholder data, production payloads, or data derived from them.

Tests for sensitive surfaces must use synthetic sentinel values and prove those values do not appear in forbidden output. Redaction coverage should include human output, JSON output, warnings, errors, and CLI-controlled persistence boundaries.

## Safety Requirements

Production writes are out of v1. Any future network or persistence expansion requires explicit review against the safety model before implementation.

Review is required for changes that add or modify:

- Authorize.Net network calls
- Non-Authorize.Net network calls
- CLI-controlled persistence
- Raw response behavior
- Redaction behavior
- Production profile handling
- Transaction, customer profile, or sandbox helper output

V1 must not add Diagnostic logs, Telemetry, crash reporting, update checks, live docs fetching, or other external network calls outside the selected Authorize.Net environment.

## Command Surfaces

Use canonical glossary vocabulary from [CONTEXT.md](CONTEXT.md). Document new command references in the Operator README and relevant docs.

Current v1 command surfaces include:

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

Response-code reference updates must follow [docs/response-code-reference.md](docs/response-code-reference.md).
