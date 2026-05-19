> **Historical research note:** This document captures an early planning snapshot and is not an up-to-date source of current project guidance. Use `README.md`, `CONTEXT.md`, `CONTRIBUTING.md`, `docs/project-spec.md`, and the other current docs for normative behavior, command names, release policy, and implementation structure.

Yes — for this project, I’d build the CLI in **Go**.

One important update first: while checking current sources, I found that the official Authorize.Net GitHub org now lists a public `authorize-net-mcp` repo, updated August 21, 2025, with very low visible adoption. I could not fetch its README, so I would not treat it as a mature, widely adopted MCP tool yet, but it does mean the earlier “no MCP at all” premise is now less clean. A CLI-first strategy still makes sense. ([GitHub][1])

## 1. Best language: Go

**Recommendation: Go, not Rust or TypeScript.**

For a cross-platform professional CLI, Go hits the best balance of performance, portability, implementation speed, install simplicity, and operational boringness — which is exactly what you want for payment tooling. Go’s toolchain supports macOS, Linux, and Windows targets across common CPU architectures, and its `GOOS`/`GOARCH` model makes multi-platform release builds straightforward. ([Go.dev][2])

Why Go is the best fit here:

| Criterion                   |                        Go |                           Rust |                          TypeScript/Node |
| --------------------------- | ------------------------: | -----------------------------: | ---------------------------------------: |
| Runtime dependency          |              Usually none |                           None |             Requires Node unless bundled |
| Cross-platform release UX   |                 Excellent |          Good, but more fiddly | Good via npm, less good as native binary |
| CLI ecosystem               |                 Excellent |                      Excellent |                                Excellent |
| Performance                 |          More than enough |                           Best |                                     Fine |
| Dev velocity                |                      High |                         Medium |                                     High |
| Binary size                 |                  Moderate |         Usually smaller/faster |                        Larger if bundled |
| Payment/security ergonomics |                      Good | Excellent, but slower to build | Good, but supply-chain surface is larger |
| Agent compatibility         | Excellent via JSON output |      Excellent via JSON output |                Excellent via JSON output |

The only material downside: **Authorize.Net does not appear to publish an official Go SDK**. Their docs and official GitHub org emphasize SDKs/sample code in languages like PHP, .NET/C#, Ruby, Node.js, Java, Python, Swift, TypeScript, and mobile SDKs — but not Go. ([developer.authorize.net][3])

That is not a dealbreaker. For this CLI, I would **not** wrap an SDK anyway. I would call the current Authorize.Net API directly over HTTP and model the requests/responses yourself. Authorize.Net’s current API supports **XML and JSON**, and their legacy SOAP/AIM/SIM/DPM surfaces are deprecated or superseded, so a new tool should be JSON-first. ([developer.authorize.net][4])

My suggested project shape:

```text
authnet-cli/
  cmd/authnet/              # CLI entrypoint
  internal/authnet/         # API client
  internal/config/          # profiles, env vars, config file
  internal/output/          # JSON/table output
  internal/safety/          # prod-write guards, confirmations
  internal/testsupport/     # sandbox fixtures, VCR-style mocks
  skills/
    authorize-net/SKILL.md  # agent guidance
```

Core design rule: **human-friendly commands, machine-stable JSON**.

Example:

```bash
authnet transaction get 123456789 --profile sandbox --json
authnet transaction list --from 2026-05-01 --to 2026-05-17 --json
authnet customer-profile get CUST123 --json
authnet sandbox charge-test-card --amount 12.34 --card visa --json
```

For agent compatibility, every command should support:

```bash
--json
--no-color
--profile sandbox
--dry-run
--confirm
--idempotency-key <key>
--trace-id <id>
```

I would make **sandbox the default profile** until the user explicitly configures production. Payment CLIs should be paranoid. That is not overengineering; that is avoiding “the robot refunded all the things.”

## 2. Can you test without incurring costs?

**Yes, for normal development/testing, you can use the Authorize.Net sandbox without real payment processing.**

Authorize.Net says the sandbox is separate from production and uses separate credentials. Sandbox transactions are not submitted to financial institutions and “will never actually process a payment.” They also provide test card numbers and response triggers for declines, AVS responses, CVV responses, partial authorization scenarios, and other cases. ([developer.authorize.net][5])

The practical implication:

| Test type                               |                    Possible in sandbox? |     Real money involved? |
| --------------------------------------- | --------------------------------------: | -----------------------: |
| Auth/capture test card                  |                                     Yes |                       No |
| Decline simulation                      |                                     Yes |                       No |
| AVS/CVV simulation                      |                                     Yes |                       No |
| Customer profiles                       |                                     Yes |           No real charge |
| Reporting APIs                          |               Yes, against sandbox data |                       No |
| Webhook handling                        | Likely yes, but requires endpoint setup |   No direct payment cost |
| Production merchant settlement behavior |                          Only simulated | Not fully representative |

Caveats:

Authorize.Net’s sandbox behaves similarly to the live gateway, but it is not a full substitute for production. Some processor-specific behavior, settlement timing, fraud settings, account updater flows, and merchant-account quirks may differ. For example, their testing guide says Account Updater in sandbox runs on the same monthly frequency as production and cannot be manually triggered. ([developer.authorize.net][5])

Also, if your CLI ever accepts raw card numbers, even in a sandbox, you need to be very careful about logs, shell history, telemetry, crash reports, and local config files. Authorize.Net’s docs explicitly call out PCI-DSS considerations for payment integrations. ([developer.authorize.net][4])

My advice: for v1, avoid commands that encourage users to type card numbers directly. Prefer sandbox-only test-card aliases:

```bash
authnet sandbox charge --amount 25.00 --card visa
authnet sandbox charge --amount 25.00 --card amex
authnet sandbox decline --reason avs
```

Then internally map those to Authorize.Net’s documented test values.

## 3. Main implementation challenges

The hard part is not the HTTP client. The hard part is building a payment CLI that agents can use safely.

The biggest challenges:

**Production write safety.**
Void, capture, refund, subscription changes, customer profile updates, and fraud review actions can have financial impact. Authorize.Net supports transaction types like authorization/capture, auth-only, prior-auth capture, void, and refund; those need explicit guardrails. ([developer.authorize.net][4])

**Duplicate transaction prevention.**
Authorize.Net supports a `duplicateWindow` transaction setting to help prevent accidental duplicate billing. Your CLI should expose that concept and probably set a conservative default for write operations. ([developer.authorize.net][4])

**Sandbox/production isolation.**
Do not merely use a `--sandbox` flag. Use named profiles with separate credential stores, visually obvious environment labels, and hard confirmation for production writes.

**Credential storage.**
Support env vars first for CI/agents, then OS keychain/keyring for humans. Avoid plaintext config for API transaction keys unless the user explicitly opts into it.

**Agent-safe output schemas.**
Tables are for humans. Agents need stable JSON with predictable keys, normalized errors, and exit codes.

**API coverage creep.**
Authorize.Net covers payments, customer profiles, recurring billing, eCheck, Accept products, OAuth, webhooks, fraud management, and reporting. Their developer guide groups these into payment fundamentals, platform essentials, digital acceptance, and risk/reporting. ([developer.authorize.net][6])

I would scope v1 like this:

```text
v0.1:
  auth test
  transaction get/list
  customer profile get/list
  sandbox charge/decline helpers
  raw API passthrough for unsupported calls

v0.2:
  webhooks list/test/verify-signature helper
  transaction reporting helpers
  response-code explainer

v0.3:
  guarded writes: void, capture, refund
  recurring billing reads
  production write confirmation policy

v1.0:
  stable JSON schema
  full docs
  install packages
  agent skill
  integration test suite
```

## 4. Best distribution channels

Use **multiple channels**, but make **GitHub Releases the canonical source of truth**.

Recommended distribution stack:

| Channel         |             Priority | Purpose                                          |
| --------------- | -------------------: | ------------------------------------------------ |
| GitHub Releases |            Must-have | Canonical binaries, checksums, SBOMs, changelog  |
| Homebrew tap    |            Must-have | Best macOS/Linux developer UX                    |
| Scoop           | Strongly recommended | Windows developer-friendly install               |
| WinGet          | Strongly recommended | Mainstream Windows package manager               |
| `.deb` / `.rpm` |          Recommended | Linux servers and CI images                      |
| Docker image    |             Optional | CI/automation use                                |
| npm wrapper     |             Optional | Easy `npx authnet-cli` usage for agent workflows |
| MCP Registry    |                Later | If/when you ship MCP                             |

GoReleaser is a strong fit here because it can create GitHub/GitLab/Gitea releases, upload artifacts, generate changelogs, generate Homebrew casks, Scoop manifests, Winget manifests, and Linux packages via nFPM. ([GoReleaser][7])

My preferred release setup:

```text
Git tag pushed:
  v0.1.0

GitHub Actions:
  run tests
  build macOS arm64/amd64
  build Linux arm64/amd64
  build Windows arm64/amd64
  generate checksums
  generate SBOM
  sign artifacts
  publish GitHub Release
  update Homebrew tap
  update Scoop bucket
  submit/update WinGet manifest
  publish .deb/.rpm
```

For the **skill**, I would initially distribute it in the same repo:

```text
skills/authorize-net/SKILL.md
skills/authorize-net/examples/
skills/authorize-net/install.sh
```

Then document install paths for the ecosystems you care about, for example Codex-style local skills, Claude-style project instructions, and generic “agent usage guide” markdown. Keep the skill coupled to the CLI version, because agent instructions will need to track command names and JSON schemas.

## Bottom line

Build it in **Go**. Use the Authorize.Net **sandbox** for normal testing without real payment costs. Start distribution with **GitHub Releases + Homebrew + Scoop + WinGet**, then add `.deb`/`.rpm`, Docker, and possibly an npm wrapper once the CLI stabilizes.

The winning product angle is not “Authorize.Net API wrapper.” That is boring. The winning angle is:

> A safe, JSON-first Authorize.Net operations CLI designed for humans, CI, and agents — with sandbox-first workflows and production write guardrails.

That is a much sharper wedge.

[1]: <https://github.com/authorizenet> "Authorize.Net · GitHub"
[2]: <https://go.dev/doc/install/source> "Installing Go from source - The Go Programming Language"
[3]: <https://developer.authorize.net/hello_world.html> "Hello world sandbox | Authorize.net Developer Center"
[4]: <https://developer.authorize.net/api/reference/features/payment_transactions.html> "Authorize.net API Documentation - Payment Transactions"
[5]: <https://developer.authorize.net/hello_world/testing_guide.html> "Testing guide | Authorize.net Developer Center"
[6]: <https://developer.authorize.net/api.html> "Developer guides | Authorize.net Developer Center"
[7]: <https://goreleaser.com/customization/release/> "Releases – GoReleaser"
