# Sandbox Integration CI

Sandbox integration CI verifies live Authorize.Net sandbox behavior without making those tests part of the default pull-request gate.

The workflow is defined in `.github/workflows/sandbox-integration.yml`. It runs only from `workflow_dispatch` or the scheduled workflow event. It does not run on pull requests or regular pushes.

## Required GitHub Configuration

Add these repository Actions secrets:

```text
AUTHNET_API_LOGIN_ID
AUTHNET_TRANSACTION_KEY
```

Use sandbox credentials only. Do not use production credentials for this workflow.

For scheduled runs, also add this repository Actions variable:

```text
AUTHNET_SANDBOX_ENABLED=true
```

The scheduled workflow skips successfully unless that variable is set to `true` and both credential secrets are present.

## Manual Runs

To run the workflow manually:

1. Open GitHub Actions.
2. Select `sandbox-integration`.
3. Choose `Run workflow`.
4. Set `run_sandbox_integration` to `true`.
5. Start the workflow.

Manual runs skip successfully unless `run_sandbox_integration` is `true` and both credential secrets are present.

## Local Runs

Run the same focused integration target locally with sandbox credentials:

```sh
AUTHNET_API_LOGIN_ID="<sandbox-api-login-id>" \
AUTHNET_TRANSACTION_KEY="<sandbox-transaction-key>" \
GOTMPDIR="$PWD/.cache/go-tmp" \
make test-sandbox-integration
```

The target sets `AUTHNET_SANDBOX_INTEGRATION=1` and runs the focused live test:

```sh
go test ./internal/cli -run 'TestSandboxAuthAndChargeApprovedIntegration' -count=1
```

## Coverage

The integration test creates a temporary sandbox-classified profile, then verifies:

- `authnet auth test`
- `authnet sandbox charge approved --amount 1.23`

The test asserts redacted output and must not print or persist sandbox credential values, card numbers, card codes, or customer PII.

## Failure Categories

Failures generally map to one of these areas:

- Configuration: missing secrets, disabled manual input, disabled scheduled variable, or invalid profile setup.
- Authentication: invalid sandbox API login ID or transaction key.
- Gateway: Authorize.Net sandbox returned a structured gateway error.
- Network: DNS, TLS, timeout, or other connectivity failure reaching the sandbox gateway.
