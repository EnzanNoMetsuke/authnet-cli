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

Durable user preferences are non-secret config only. Supported preferences in this release include output-mode preferences, `preferences.color`, and nested transaction list sorting and filtering preferences in `config.yaml`:

```yaml
preferences:
  json: never
  automation: never
  color: auto
  transaction_list:
    sort_by: timestamp
    sort_order: descending
    filter:
      status: declined
      amount: 1.23
      payment: Visa XXXX1111
```

`preferences.json` and `preferences.automation` support `always` or `never`. `preferences.json: always` selects the JSON envelope by default. `preferences.automation: always` selects automation mode by default, including JSON output, no color, no prompts, structured failures on stdout, and stable exit codes. If both durable output preferences are `always`, automation takes precedence at the preference layer and each preference-reading command emits a structured warning until one preference is removed.

`preferences.color` supports `auto`, `always`, or `never`. `preferences.transaction_list.sort_by` supports `timestamp`, `transaction_id`, or `amount`. `preferences.transaction_list.sort_order` supports `ascending` or `descending`. Transaction list filters are exact-match only: status must be an Authorize.Net `transactionStatusEnum` value, amount must be a positive integer or decimal with up to two decimal places, and payment must be a canonical redacted card summary such as `Visa XXXX1111`.

Precedence is: explicit command-line flags, automation safety overrides where applicable, environment overrides such as `AUTHNET_JSON`, `AUTHNET_AUTOMATION`, `AUTHNET_COLOR`, `AUTHNET_TX_SORT_BY`, `AUTHNET_TX_SORT_ORDER`, `AUTHNET_TX_FILTER_STATUS`, `AUTHNET_TX_FILTER_AMOUNT`, and `AUTHNET_TX_FILTER_PAYMENT`, durable preferences, then built-in defaults. Automation should still prefer `--automation`; it forces JSON output and no color regardless of persisted preferences.

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
authnet config migrate
authnet paths
```

If a legacy `profiles.json` file is still active, run `authnet --automation config migrate` for a structured migration report. Treat `data.result` as the stable status token, `data.message` as display text, `data.active_config` as the current active config file when present, and `data.migrated_path` as the file written by a completed migration or recovery. The `active_config`, `original_path`, `migrated_path`, and `backup_path` fields are always present in automation output and use an empty string when not applicable. If only `DEPRECATED-profiles.json` remains and `config.yaml` is absent, rerun with `authnet --automation --yes config migrate` to approve recovery from the retained backup.

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
authnet --automation --raw-response --profile sandbox-main auth test
authnet --automation --raw-response --profile sandbox-main transaction get TRANSACTION_ID
authnet --automation --raw-response --profile sandbox-main transaction unsettled list --limit 25 --page 1
```

Those are the only supported raw-response commands. Other commands fail clearly when `--raw-response` is provided. Raw unsettled transaction list output returns one selected gateway page; `--page` chooses the gateway page number and `--limit` chooses that page's gateway page size. If another raw page exists, bare raw-response mode writes the indication to stderr, while JSON or automation mode carries it as a structured warning.

Raw unsettled mode supports gateway-native `--sort-by timestamp|transaction_id`, `--sort-order ascending|descending`, and `--status any|pendingApproval`. Amount sorting, exact transaction-status filtering, amount filtering, and payment filtering are available in normalized output only. If raw response mode runs while `preferences.json: never` or `preferences.automation: never` is configured, the CLI warns that those durable preferences are ignored so the gateway JSON can be presented accurately. Bare raw-response mode keeps stdout as the raw gateway JSON and writes warnings to stderr; JSON or automation raw-response mode carries warnings in the envelope.

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
