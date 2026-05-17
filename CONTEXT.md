# Authorize.Net Operations CLI

This context defines the product and domain language for an Authorize.Net command-line tool. It exists to keep scope and terminology precise as the project specification develops.

## Language

**Authorize.Net operations CLI**:
A command-line product for inspecting and safely operating Authorize.Net merchant data and gateway actions across sandbox and production environments.
_Avoid_: Authorize.Net automation toolkit, SDK wrapper, MCP server

**authnet**:
The executable name for the Authorize.Net operations CLI.
_Avoid_: authnet-cli executable

**authnet-cli**:
The repository and project name for the Authorize.Net operations CLI.
_Avoid_: authnet executable

**Project license**:
The open-source license governing use and distribution of the project.
_Avoid_: Unlicensed source

**Operator**:
A person using the CLI to inspect or perform Authorize.Net gateway actions with direct accountability for the result.
_Avoid_: End user, consumer, agent

**Profile**:
A named set of Authorize.Net access details classified as either sandbox or production.
_Avoid_: Environment, account, credentials

**Profile name**:
An operator-defined non-secret label for a profile.
_Avoid_: Customer name, merchant secret, sensitive label

**Credential source**:
The place a profile uses to retrieve Authorize.Net secrets without storing them in plaintext profile data by default.
_Avoid_: Plaintext profile secret

**Plaintext credential storage**:
CLI-controlled persistence of Authorize.Net secrets in unencrypted form.
_Avoid_: Credential source, environment variable injection

**Environment classification**:
The sandbox-or-production designation attached to a profile.
_Avoid_: Profile name, mode, flag

**Default profile**:
A sandbox-classified profile used when an operator does not explicitly choose a profile.
_Avoid_: Production default, implicit production profile

**Profile setup**:
Workflows for creating or updating profiles and their credential-source references.
_Avoid_: Credential storage, authentication test

**Profile removal**:
A local workflow for removing saved profile metadata and optionally its secure local credential reference.
_Avoid_: Gateway deletion, environment variable mutation

**Profile portability**:
Future workflows for exporting or importing non-secret profile configuration across environments.
_Avoid_: v1 profile setup, credential export

**Profile listing**:
A command surface for showing saved profiles using non-secret metadata.
_Avoid_: Credential display, merchant detail

**Production marker**:
A clear indication that a profile or command result is production-classified.
_Avoid_: Color-only warning, implicit profile naming

**Profile config**:
Non-secret per-user configuration that stores profiles, environment classifications, and credential-source references.
_Avoid_: Credential source, plaintext credential storage

**Config directory**:
The per-user directory where CLI-controlled non-secret configuration is stored.
_Avoid_: Project directory, credential source

**Path inspection**:
A command surface that shows operators where the CLI stores or reads local state.
_Avoid_: Hidden path, documentation-only path

**authnet paths**:
The canonical path-inspection command.
_Avoid_: authnet config path

**Version reporting**:
A local flag and command surface that reports CLI and contract version information without contacting external services.
_Avoid_: Update check

**Man page**:
A packaged manual page for shell-level command documentation.
_Avoid_: v1 help surface

**Shell completion**:
Generated command-line completion metadata for discovering commands and flags.
_Avoid_: Secret-bearing completion, sensitive profile disclosure

**Dynamic profile completion**:
Shell completion that suggests saved profile names.
_Avoid_: Static command completion

**Environment override**:
A process-level setting that temporarily overrides profile configuration for automation or one-off use.
_Avoid_: Profile config, default profile

**Interactive setup**:
Guided profile setup for human operators.
_Avoid_: Required setup path, automation setup

**Non-interactive setup**:
Deterministic profile setup suitable for scripts, CI, and agents.
_Avoid_: Interactive wizard

**Read-first release**:
A release whose production-facing behavior is limited to inspecting Authorize.Net state, while write-like workflows are confined to sandbox helpers.
_Avoid_: Write-capable v1, full operations release

**Production write**:
Any command that mutates production Authorize.Net state.
_Avoid_: Money-moving action, financial write

**Dry run**:
A future mode that previews a planned mutation without performing it.
_Avoid_: Read command, sandbox test helper

**Production read**:
Any command that inspects production Authorize.Net state without mutating it.
_Avoid_: Safe command, harmless command

**Sensitive payment data**:
Payment or cardholder data whose storage or logging would create PCI or customer privacy risk.
_Avoid_: Debug data, raw response

**Compliance guarantee**:
A claim that using the CLI satisfies a regulatory or payment-industry compliance obligation.
_Avoid_: Risk-reduction posture, safety model

**Customer PII**:
Customer-identifying information whose storage or logging would create privacy risk.
_Avoid_: Debug data, raw response

**CLI-controlled persistence**:
Any file, cache, trace, fixture, diagnostic bundle, or crash artifact written by the CLI.
_Avoid_: Log file only, debug trace only

**Diagnostic log**:
A CLI-created record of diagnostic execution details written for later inspection.
_Avoid_: Terminal diagnostic output, JSON command result

**External network call**:
A network request to any service other than the selected Authorize.Net environment.
_Avoid_: Authorize.Net API call

**Current JSON API**:
The supported Authorize.Net API surface used by the CLI for gateway communication.
_Avoid_: Legacy SOAP, AIM, SIM, DPM

**Telemetry**:
Collection or transmission of CLI usage, behavior, diagnostic, or crash information.
_Avoid_: Command output, local terminal diagnostics

**Redacted output**:
Command output that hides sensitive fields while preserving enough non-sensitive context for operations.
_Avoid_: Full response, raw output

**Raw response mode**:
A sandbox-only output mode that returns the unredacted gateway response for debugging.
_Avoid_: Production raw output, normal output

**JSON envelope**:
A stable machine-readable response wrapper that carries common command metadata and command-specific data.
_Avoid_: Bespoke JSON shape, raw API response

**Schema version**:
The version identifier for the stable JSON envelope and data contract.
_Avoid_: CLI version

**Human-readable output**:
Default interactive command output optimized for operator scanning rather than automation.
_Avoid_: Stable machine contract

**Stable JSON contract**:
The versioned machine interface exposed through JSON output.
_Avoid_: Human-readable output, table output

**Explicit JSON output**:
Machine-readable command output selected by an explicit operator or automation request.
_Avoid_: TTY auto-detection, implicit JSON

**Structured failure**:
A machine-readable command failure represented inside the JSON envelope.
_Avoid_: Human diagnostic, panic output

**Structured warning**:
A machine-readable non-fatal command caveat represented inside the JSON envelope.
_Avoid_: Human-only warning, unstructured notice

**Automation mode**:
A global execution mode that makes command behavior predictable for scripts, CI, and agents.
_Avoid_: Interactive mode, TUI mode

**Exit-code taxonomy**:
A small fixed set of process exit codes that categorizes command outcomes for automation.
_Avoid_: Single failure code, unbounded exit code set

**Agent usage guide**:
Repo-local guidance that tells automation and agents how to use the CLI safely.
_Avoid_: Installable agent skill, command reference

**Safety model document**:
Operator-facing documentation that explains the CLI's safety and privacy boundaries.
_Avoid_: Formal compliance report, implementation-only notes

**Contribution guide**:
Repo documentation that tells contributors how to work safely on the project.
_Avoid_: Heavy governance, code of conduct replacement

**Operator README**:
The repository entry document that explains the product promise, safety posture, and current release status to operators.
_Avoid_: Internal-only README, generic SDK wrapper description

**Non-goal**:
A capability that is outside the intended product boundary rather than merely deferred to a future version.
_Avoid_: Future scope, deferred release surface

**Installable agent skill**:
A versioned agent capability package that encodes stable CLI commands, safety rules, and JSON schemas.
_Avoid_: Agent usage guide

**Canonical release**:
The authoritative published build of the CLI with versioned artifacts and integrity metadata.
_Avoid_: Source-only release, package-manager mirror

**Release checksum**:
Integrity metadata that lets an operator verify a release artifact has not changed.
_Avoid_: Artifact signature, SBOM

**Future release attestation**:
Deferred release metadata such as artifact signatures or software bills of materials.
_Avoid_: v1 release requirement

**Convenience install channel**:
A package-manager distribution path derived from a canonical release.
_Avoid_: Canonical release

**Homebrew tap**:
A separate package-index repository used as the initial convenience install channel.
_Avoid_: In-repository formula

**Curated command**:
A supported CLI operation with defined behavior, redaction, safety handling, and output schema.
_Avoid_: Raw API passthrough, generic gateway tunnel

**Authentication test**:
A curated command that validates profile access and confirms safe profile identity context.
_Avoid_: Connectivity ping, secret display

**Config validation**:
A local command that checks non-secret profile configuration and credential-source availability without contacting Authorize.Net.
_Avoid_: Authentication test, gateway validation

**authnet config validate**:
The canonical config-validation command.
_Avoid_: authnet doctor

**authnet doctor**:
A broad diagnostic command reserved for a future version if concrete support workflows justify it.
_Avoid_: v1 diagnostic surface

**Canonical command name**:
The documented command wording that uses formal Authorize.Net terminology.
_Avoid_: Shorthand alias, ambiguous abbreviation

**Transaction command**:
A curated command for inspecting Authorize.Net transaction state.
_Avoid_: Production write

**Settled transaction history**:
Transaction records available through historical reporting over a selected time range.
_Avoid_: Unsettled transaction set, current batch

**Unsettled transaction set**:
Transaction records that have not yet settled and belong to the current settlement flow.
_Avoid_: Settled transaction history, historical report

**Bounded pagination**:
List-command behavior that returns a conservative result set by default and requires explicit expansion for more data.
_Avoid_: Unbounded auto-pagination, raw API page only

**Time range**:
An absolute or relative operator-selected interval used to filter time-based gateway records.
_Avoid_: Implicit reporting window

**Resolved timestamp**:
An explicit timestamp emitted after parsing operator-supplied time input.
_Avoid_: Relative timestamp, local-only timestamp

**Operator-local time**:
The local timezone context used to interpret operator-supplied date or relative time input when no timezone is specified.
_Avoid_: Merchant timezone default, UTC-only input

**Normalized transaction view**:
A stable transaction representation that exposes common operational fields plus allowlisted redacted gateway details.
_Avoid_: Raw transaction response, gateway dump

**Safe normalized result**:
A command result shaped for output after sensitive gateway fields have been removed or redacted.
_Avoid_: Raw gateway response, renderer-only redaction

**Customer profile command**:
A curated command for inspecting Authorize.Net customer profile state.
_Avoid_: Customer profile mutation

**Customer profile metadata**:
The non-sensitive identifying and summary fields of an Authorize.Net customer profile.
_Avoid_: Nested payment profile, shipping address detail

**Nested customer profile detail**:
Payment profile or shipping address information contained inside a customer profile.
_Avoid_: Customer profile metadata

**Response-code explanation**:
A curated command that explains Authorize.Net gateway and API codes for operator debugging.
_Avoid_: Raw documentation lookup

**Code family**:
The category that gives an Authorize.Net code its correct meaning, such as transaction response, API message, validation, AVS, or CVV.
_Avoid_: Undifferentiated code

**Recurring billing**:
Future workflows for inspecting and eventually operating Authorize.Net subscription-based payment schedules.
_Avoid_: v1 transaction command, one-time payment

**Subscription inspection**:
Read-only recurring billing workflows for understanding subscription state without changing it.
_Avoid_: Subscription mutation

**Subscription mutation**:
Recurring billing workflows that create, update, cancel, or otherwise change a subscription.
_Avoid_: Subscription inspection

**Subscription**:
An Authorize.Net recurring billing plan that generates payments according to a payment schedule.
_Avoid_: Transaction, customer profile

**Payment schedule**:
The start date, interval, occurrence count, and trial-period terms that define when a subscription generates payments.
_Avoid_: Settlement schedule, transaction date

**Subscription status**:
The lifecycle state of a subscription, such as active, expired, suspended, canceled, or terminated.
_Avoid_: Transaction status, response code

**Subscription payment**:
A transaction generated by a subscription according to its payment schedule.
_Avoid_: Subscription, payment schedule

**Curated response-code reference**:
A locally available response-code reference maintained against official Authorize.Net sources.
_Avoid_: Live documentation scrape, stale lookup table

**Source meaning**:
The official or source-derived meaning of an Authorize.Net code.
_Avoid_: Recommended next step, interpretation

**Recommended next step**:
Project-authored guidance for what an operator should do after seeing a code.
_Avoid_: Source meaning, official documentation

**Safety policy**:
A rule set that determines whether a command is allowed for a profile, command class, and execution mode.
_Avoid_: Confirmation prompt, warning

**Confirmation**:
An operator acknowledgement required before an allowed high-risk action proceeds in interactive use.
_Avoid_: Safety policy, permission

**Non-interactive execution**:
Command execution that cannot ask an operator for runtime confirmation.
_Avoid_: Interactive confirmation

**Sandbox test helper**:
A sandbox-only workflow that creates known test transactions or response scenarios for credential verification and debugging.
_Avoid_: Sandbox resource management, fixture management

**Card test scenario**:
A sandbox test helper scenario for card authorization/capture success, decline, AVS/CVV behavior, or duplicate-window behavior.
_Avoid_: eCheck test scenario, partial authorization scenario

**Future payment test scenario**:
A deferred sandbox test helper scenario for payment capabilities outside v1 card coverage.
_Avoid_: v1 card test scenario

**Duplicate protection**:
Behavior that reduces the risk of accidentally submitting the same payment action more than once.
_Avoid_: Sandbox-only test, optional safety

**Offline simulation**:
A development or test-only substitute for gateway behavior that does not contact Authorize.Net.
_Avoid_: Sandbox test helper, gateway result

**Sandbox integration test**:
An opt-in test that contacts the real Authorize.Net sandbox gateway.
_Avoid_: Default unit test, offline simulation

**Sandbox integration CI**:
A manual or scheduled verification workflow that runs sandbox integration tests with explicit credentials.
_Avoid_: Required pull-request gate

**JSON golden test**:
A regression test that verifies the stable JSON contract for a command result.
_Avoid_: Human-output snapshot

**Human-output test**:
A focused regression test that verifies important human-readable output behavior without freezing all formatting.
_Avoid_: Stable JSON contract test

**Redaction test**:
A regression test that proves synthetic sensitive sentinel values do not appear where they are forbidden.
_Avoid_: Real merchant fixture, incidental output test

**Sandbox resource management**:
Sandbox-only workflows for creating, updating, listing, and cleaning up persisted Authorize.Net test resources.
_Avoid_: Test transaction shortcut

**Webhook tooling**:
Workflows for receiving, testing, or validating event notifications outside the Authorize.Net gateway resource lifecycle.
_Avoid_: Sandbox resource management

## Relationships

- The **Authorize.Net operations CLI** operates against Authorize.Net environments.
- **authnet** is the executable for the **authnet-cli** project.
- The **Project license** is MIT.
- An **Operator** is the primary v1 user of the **Authorize.Net operations CLI**.
- A **Profile** has a **Profile name** intended to be visible in normal output.
- A **Profile** references a **Credential source**.
- A **Profile** has exactly one **Environment classification**.
- **Profile listing** includes production profiles using non-secret metadata and a textual **Production marker**.
- The **Authorize.Net operations CLI** must not create **Plaintext credential storage**.
- A **Default profile** must be sandbox-classified.
- **Profile setup** supports **Interactive setup** and **Non-interactive setup**.
- **Profile removal** affects local profile state, not Authorize.Net gateway state.
- The **Read-first release** excludes **Profile portability**.
- **Profile config** stores non-secret profile data, while **Environment overrides** can adjust behavior at process time.
- **Profile config** lives under the **Config directory**.
- **Path inspection** exposes the **Config directory** to operators.
- **authnet paths** is the canonical **Path inspection** command.
- The **Read-first release** includes **Version reporting**.
- The **Read-first release** includes **Shell completion**.
- The **Read-first release** excludes **Man pages**.
- The **Read-first release** excludes **Dynamic profile completion**.
- The first public version of the **Authorize.Net operations CLI** is a **Read-first release**.
- A **Read-first release** allows explicit **Production reads**.
- A **Read-first release** excludes **Production writes**.
- The **Read-first release** excludes **Dry run**.
- The **Authorize.Net operations CLI** must never write **Sensitive payment data** or **Customer PII** through **CLI-controlled persistence**.
- The **Authorize.Net operations CLI** does not make a **Compliance guarantee**.
- The **Read-first release** does not create **Diagnostic logs**.
- The **Read-first release** makes no **External network calls**.
- The **Read-first release** includes no **Telemetry**.
- The **Authorize.Net operations CLI** uses the **Current JSON API**.
- The **Authorize.Net operations CLI** returns **Redacted output** by default.
- Machine-readable command output uses a **JSON envelope**.
- Every **JSON envelope** includes a **Schema version**.
- The **Authorize.Net operations CLI** defaults to **Human-readable output** for interactive use.
- Automation relies on the **Stable JSON contract**.
- The **Stable JSON contract** is exposed through **Explicit JSON output**.
- **Explicit JSON output** writes **Structured failures** to stdout.
- A **JSON envelope** can include **Structured warnings**.
- **Automation mode** implies **Explicit JSON output** and disables interactive behavior.
- **Automation mode** may use a sandbox **Default profile**, but production profiles must be selected explicitly.
- The **Authorize.Net operations CLI** uses an **Exit-code taxonomy**.
- The **Read-first release** includes an **Agent usage guide**.
- The **Read-first release** includes a **Safety model document**.
- The **Read-first release** includes a safety-focused **Contribution guide**.
- The project includes an **Operator README** from the start.
- The project specification distinguishes **Non-goals** from future scope.
- **Non-goals** include generic SDK wrapper behavior, raw production gateway passthrough, legacy Authorize.Net protocol support, and compliance guarantees.
- An **Installable agent skill** follows after the **Stable JSON contract** settles.
- The **Read-first release** includes a **Canonical release** and an initial **Convenience install channel**.
- A **Canonical release** includes **Release checksums**.
- **Future release attestations** are planned after the **Read-first release**.
- The initial **Convenience install channel** is a **Homebrew tap**.
- The **Read-first release** contains **Curated commands** rather than raw API passthrough.
- **Curated commands** use **Canonical command names**.
- The **Read-first release** includes **Authentication test**.
- The **Read-first release** includes **Config validation**.
- **authnet config validate** is the canonical **Config validation** command.
- The **Read-first release** excludes **authnet doctor**.
- The **Read-first release** includes **Transaction commands**, **Customer profile commands**, **Response-code explanation**, and **Sandbox test helpers**.
- **Transaction commands** distinguish **Settled transaction history** from the **Unsettled transaction set**.
- List-style commands use **Bounded pagination**.
- Time-based list commands accept **Time ranges** and emit **Resolved timestamps**.
- Time input without an explicit timezone uses **Operator-local time**.
- Curated command output is built from **Safe normalized results**.
- Transaction lookup returns a **Normalized transaction view**.
- **Customer profile commands** return **Customer profile metadata** by default and require explicit selection for **Nested customer profile detail**.
- **Response-code explanation** uses a **Curated response-code reference**.
- **Response-code explanation** preserves the relevant **Code family**.
- **Response-code explanation** separates **Source meaning** from **Recommended next steps**.
- **Recurring billing** is outside the **Read-first release** and is required for a future version.
- **Recurring billing** includes **Subscriptions**, **Payment schedules**, **Subscription statuses**, and **Subscription payments**.
- Future **Recurring billing** support starts with **Subscription inspection** before **Subscription mutation**.
- **Raw response mode** is unavailable for production-classified profiles.
- A **Safety policy** determines whether a **Production write** is allowed before any **Confirmation** is considered.
- **Non-interactive execution** of high-risk actions requires **Safety policy** permission rather than confirmation flags.
- A **Read-first release** includes **Sandbox test helpers** but not **Sandbox resource management**.
- **Sandbox test helpers** in the **Read-first release** cover **Card test scenarios**.
- eCheck and partial authorization are **Future payment test scenarios**.
- **Duplicate protection** is expected for future write-capable commands.
- **Sandbox test helpers** contact the real sandbox gateway; **Offline simulation** is not product-facing behavior.
- **Sandbox integration tests** are opt-in and contact the real sandbox gateway.
- **Sandbox integration CI** is not a required pull-request gate.
- **JSON golden tests** cover the **Stable JSON contract**.
- **Human-output tests** cover important operator-facing output behavior selectively.
- **Redaction tests** are mandatory for sensitive output and persistence boundaries.
- **Sandbox resource management** is planned as the immediate follow-up surface after the **Read-first release**.
- **Webhook tooling** is separate from **Sandbox resource management**.
- **Webhook tooling** is outside the **Read-first release**.

## Example dialogue

> **Dev:** "Should this project include an MCP server in the first product boundary?"
> **Domain expert:** "No — the product is the **Authorize.Net operations CLI** first; other integration surfaces can follow only after the CLI contract is stable."
>
> **Dev:** "Should operators type `authnet-cli transaction get`?"
> **Domain expert:** "No — **authnet** is the executable; **authnet-cli** is the repository and project name."
>
> **Dev:** "What license should the project use?"
> **Domain expert:** "Use MIT as the **Project license**."
>
> **Dev:** "Are we optimizing the command model for agents first?"
> **Domain expert:** "No — design for the **Operator** first, while keeping automation and agent usage stable through JSON output, non-interactive modes, and predictable failures."
>
> **Dev:** "Can we use a `--sandbox` flag as the environment model?"
> **Domain expert:** "No — use a **Profile** with an explicit **Environment classification** so safety does not depend on naming convention."
>
> **Dev:** "Do human operators have to use the OS keychain?"
> **Domain expert:** "No — a **Profile** can use an environment-variable or secure local-storage **Credential source**, but plaintext profile secrets are not the default."
>
> **Dev:** "Are profile names redacted from normal output?"
> **Domain expert:** "No — a **Profile name** is visible, but operators should not put secrets, customer PII, or sensitive merchant labels in it."
>
> **Dev:** "Should profile listing hide production profiles?"
> **Domain expert:** "No — **Profile listing** includes production profiles using non-secret metadata and a clear textual **Production marker**."
>
> **Dev:** "Can color be the only production warning?"
> **Domain expert:** "No — a **Production marker** must be textual; color can only reinforce it."
>
> **Dev:** "Can the CLI store API transaction keys in plaintext for throwaway containers?"
> **Domain expert:** "No — use environment injection instead; the CLI must not create **Plaintext credential storage**."
>
> **Dev:** "Can production be the default profile if the operator chooses it?"
> **Domain expert:** "No — a **Default profile** is always sandbox-classified; production must be selected explicitly."
>
> **Dev:** "Is profile setup only an interactive wizard?"
> **Domain expert:** "No — **Profile setup** supports both **Interactive setup** for operators and **Non-interactive setup** for automation."
>
> **Dev:** "Does removing a profile delete anything in Authorize.Net?"
> **Domain expert:** "No — **Profile removal** affects local profile state, not gateway state."
>
> **Dev:** "Can v1 export or import profiles?"
> **Domain expert:** "No — **Profile portability** is outside the **Read-first release**."
>
> **Dev:** "Can profile files contain API transaction keys?"
> **Domain expert:** "No — **Profile config** stores non-secret profile data and credential-source references only."
>
> **Dev:** "Should operators have to infer where profile config lives?"
> **Domain expert:** "No — **Path inspection** exposes the **Config directory**."
>
> **Dev:** "Should the path inspection command be `authnet config path`?"
> **Domain expert:** "No — use **authnet paths**."
>
> **Dev:** "Can v1 check GitHub for newer versions?"
> **Domain expert:** "No — v1 includes local **Version reporting**, not update checks."
>
> **Dev:** "Should only `authnet version` exist?"
> **Domain expert:** "No — **Version reporting** includes both `authnet --version` and `authnet version`."
>
> **Dev:** "Does v1 need packaged manual pages?"
> **Domain expert:** "No — **Man pages** are outside the **Read-first release**."
>
> **Dev:** "Should v1 skip shell completion until later?"
> **Domain expert:** "No — the **Read-first release** includes **Shell completion** without sensitive values."
>
> **Dev:** "Can v1 shell completion suggest saved profile names?"
> **Domain expert:** "No — the **Read-first release** excludes **Dynamic profile completion**."
>
> **Dev:** "Can v1 refund or void production transactions?"
> **Domain expert:** "No — v1 is a **Read-first release**; production writes wait until the safety model is proven."
>
> **Dev:** "Does v1 need a global dry-run flag?"
> **Domain expert:** "No — **Dry run** is a future mode for planned mutations, not a **Read-first release** feature."
>
> **Dev:** "Does changing a production customer profile count as a production write if no money moves?"
> **Domain expert:** "Yes — any production mutation is a **Production write**."
>
> **Dev:** "Can v1 inspect production transactions?"
> **Domain expert:** "Yes, explicit **Production reads** are in scope, but **Sensitive payment data** and **Customer PII** must never be logged to disk."
>
> **Dev:** "Can debug traces or generated fixtures store real customer responses?"
> **Domain expert:** "No — **CLI-controlled persistence** must never contain **Sensitive payment data** or **Customer PII**."
>
> **Dev:** "Does the safety model mean the CLI is PCI-DSS compliant?"
> **Domain expert:** "No — the CLI reduces handling risk but does not make a **Compliance guarantee**."
>
> **Dev:** "Can v1 write redacted diagnostic logs to disk?"
> **Domain expert:** "No — the **Read-first release** does not create **Diagnostic logs**."
>
> **Dev:** "Can v1 check GitHub for updates or fetch docs live?"
> **Domain expert:** "No — the **Read-first release** makes no **External network calls**."
>
> **Dev:** "Should the CLI support legacy SOAP, AIM, SIM, or DPM integrations?"
> **Domain expert:** "No — the **Authorize.Net operations CLI** uses the **Current JSON API**."
>
> **Dev:** "Can v1 include disabled-by-default crash reporting?"
> **Domain expert:** "No — the **Read-first release** includes no **Telemetry**."
>
> **Dev:** "Should a production transaction read print the raw gateway response by default?"
> **Domain expert:** "No — **Production reads** return **Redacted output** by default."
>
> **Dev:** "Can sandbox commands print full customer-like data by default because it is test data?"
> **Domain expert:** "No — the **Authorize.Net operations CLI** returns **Redacted output** by default in every environment."
>
> **Dev:** "Can an operator request the raw gateway response in production for debugging?"
> **Domain expert:** "No — **Raw response mode** is sandbox-only."
>
> **Dev:** "Can each command invent its own top-level JSON shape?"
> **Domain expert:** "No — machine-readable output uses a **JSON envelope** with command-specific data inside it."
>
> **Dev:** "Can JSON consumers infer the schema from the CLI version?"
> **Domain expert:** "No — every **JSON envelope** includes a **Schema version**."
>
> **Dev:** "Is table output the automation contract?"
> **Domain expert:** "No — table output is **Human-readable output**; automation relies on the **Stable JSON contract**."
>
> **Dev:** "Can stdout TTY detection decide whether output is JSON?"
> **Domain expert:** "No — the **Stable JSON contract** is exposed through **Explicit JSON output**."
>
> **Dev:** "Should JSON failures be split onto stderr?"
> **Domain expert:** "No — **Explicit JSON output** writes **Structured failures** to stdout; exit codes still indicate failure."
>
> **Dev:** "Can warnings exist only as human-readable text?"
> **Domain expert:** "No — a **JSON envelope** can include **Structured warnings**."
>
> **Dev:** "Should CI have to combine several flags to avoid prompts and TUI behavior?"
> **Domain expert:** "No — **Automation mode** implies **Explicit JSON output** and disables interactive behavior."
>
> **Dev:** "Can automation silently inherit a production profile?"
> **Domain expert:** "No — **Automation mode** may use a sandbox **Default profile**, but production profiles must be selected explicitly."
>
> **Dev:** "Can all command failures exit with code 1?"
> **Domain expert:** "No — use an **Exit-code taxonomy** so automation can distinguish failure classes."
>
> **Dev:** "Should v1 ship a full installable agent skill?"
> **Domain expert:** "No — v1 includes an **Agent usage guide**; an **Installable agent skill** follows after the **Stable JSON contract** settles."
>
> **Dev:** "Can safety rules live only in scattered command docs?"
> **Domain expert:** "No — the **Read-first release** includes a **Safety model document**."
>
> **Dev:** "Can contribution rules wait until the project has many contributors?"
> **Domain expert:** "No — the **Read-first release** includes a safety-focused **Contribution guide**."
>
> **Dev:** "Can the README stay internal until implementation exists?"
> **Domain expert:** "No — the project includes an **Operator README** from the start, with pre-release status clearly marked."
>
> **Dev:** "Should deferred features be listed as non-goals?"
> **Domain expert:** "No — a **Non-goal** is outside the intended product boundary, not merely future scope."
>
> **Dev:** "Are production writes and webhook tooling non-goals?"
> **Domain expert:** "No — they are future scope. True **Non-goals** include generic SDK wrapper behavior, raw production gateway passthrough, legacy protocol support, and compliance guarantees."
>
> **Dev:** "Can v1 be source-build only?"
> **Domain expert:** "No — the **Read-first release** includes a **Canonical release** and an initial **Convenience install channel**."
>
> **Dev:** "Should the Homebrew formula live in the CLI source repository?"
> **Domain expert:** "No — use a separate **Homebrew tap**."
>
> **Dev:** "Are signatures and SBOMs required before v1 can ship?"
> **Domain expert:** "No — v1 requires **Release checksums**; signatures and SBOMs are **Future release attestations**."
>
> **Dev:** "Can v1 include a raw API passthrough for unsupported endpoints?"
> **Domain expert:** "No — v1 uses **Curated commands** so redaction, safety handling, and output schemas remain enforceable."
>
> **Dev:** "Should the docs use `txn` as the primary transaction command?"
> **Domain expert:** "No — **Canonical command names** use formal Authorize.Net terminology; shorthand aliases can be considered later."
>
> **Dev:** "Is `auth test` just a network ping?"
> **Domain expert:** "No — **Authentication test** validates profile access and confirms safe profile identity context."
>
> **Dev:** "Can config validation and auth testing be one command?"
> **Domain expert:** "No — **Config validation** is local and checks credential-source availability without contacting Authorize.Net; **Authentication test** validates gateway access."
>
> **Dev:** "Can config validation print credential values?"
> **Domain expert:** "No — **Config validation** checks credential-source availability, not secret contents."
>
> **Dev:** "Should local config validation be called `authnet doctor`?"
> **Domain expert:** "No — use **authnet config validate**."
>
> **Dev:** "Should v1 include a broad diagnostic command?"
> **Domain expert:** "No — **authnet doctor** is reserved for a future version if support workflows justify it."
>
> **Dev:** "Are customer profiles out of scope until write support exists?"
> **Domain expert:** "No — read-only **Customer profile commands** are part of the **Read-first release**."
>
> **Dev:** "Can `transaction list` mean both historical reporting and current unsettled transactions?"
> **Domain expert:** "No — **Settled transaction history** and the **Unsettled transaction set** are distinct concepts."
>
> **Dev:** "Should list commands automatically fetch every matching production record?"
> **Domain expert:** "No — list-style commands use **Bounded pagination**."
>
> **Dev:** "Can JSON output say a query used `last 7d` without resolving it?"
> **Domain expert:** "No — time-based list commands accept **Time ranges** and emit **Resolved timestamps**."
>
> **Dev:** "Should date-only input default to the merchant timezone?"
> **Domain expert:** "No — time input without an explicit timezone uses **Operator-local time**, while output uses explicit offsets."
>
> **Dev:** "Should transaction lookup dump the raw gateway object?"
> **Domain expert:** "No — transaction lookup returns a **Normalized transaction view**."
>
> **Dev:** "Can redaction live only inside table and JSON renderers?"
> **Domain expert:** "No — curated command output is built from **Safe normalized results**."
>
> **Dev:** "Should customer profile lookup include payment profiles and shipping addresses by default?"
> **Domain expert:** "No — **Customer profile metadata** is the default; **Nested customer profile detail** requires explicit selection."
>
> **Dev:** "Should response-code explanation depend on fetching live documentation?"
> **Domain expert:** "No — use a **Curated response-code reference** with a maintenance path back to official sources."
>
> **Dev:** "Should response-code explanation only handle transaction response codes?"
> **Domain expert:** "No — explain gateway and API codes while preserving the **Code family**."
>
> **Dev:** "Can response-code explanations include advice written by this project?"
> **Domain expert:** "Yes, but separate **Source meaning** from **Recommended next steps**."
>
> **Dev:** "Does v1 include recurring billing commands?"
> **Domain expert:** "No — **Recurring billing** is required for a future version, using **Subscription**, **Payment schedule**, **Subscription status**, and **Subscription payment** as the core vocabulary."
>
> **Dev:** "Should recurring billing support start with creating and canceling subscriptions?"
> **Domain expert:** "No — future **Recurring billing** starts with **Subscription inspection** before **Subscription mutation**."
>
> **Dev:** "Is a confirmation prompt enough to make production writes safe?"
> **Domain expert:** "No — a **Safety policy** decides whether the write is allowed; **Confirmation** only acknowledges an allowed high-risk action."
>
> **Dev:** "Can automation satisfy a confirmation prompt with `--confirm`?"
> **Domain expert:** "No — **Non-interactive execution** of high-risk actions requires **Safety policy** permission."
>
> **Dev:** "Do sandbox writes include creating reusable customer profiles and subscriptions?"
> **Domain expert:** "Not in v1 — use **Sandbox test helpers** for known transaction scenarios, then add **Sandbox resource management** as the next release surface."
>
> **Dev:** "Can sandbox helper commands return fake offline responses?"
> **Domain expert:** "No — **Sandbox test helpers** contact the real sandbox gateway; **Offline simulation** is for tests and development only."
>
> **Dev:** "Do normal tests have to contact Authorize.Net?"
> **Domain expert:** "No — **Sandbox integration tests** are opt-in; default tests use local doubles or fixtures."
>
> **Dev:** "Should every pull request depend on Authorize.Net sandbox availability?"
> **Domain expert:** "No — **Sandbox integration CI** is manual or scheduled, not a required pull-request gate."
>
> **Dev:** "Should table formatting snapshots be as strict as JSON schemas?"
> **Domain expert:** "No — **JSON golden tests** cover the stable contract; **Human-output tests** are selective."
>
> **Dev:** "Can redaction safety rely on code review alone?"
> **Domain expert:** "No — **Redaction tests** are mandatory and use synthetic sentinel values."
>
> **Dev:** "Do v1 sandbox helpers cover eCheck and partial authorization?"
> **Domain expert:** "No — v1 covers **Card test scenarios**; eCheck and partial authorization are **Future payment test scenarios**."
>
> **Dev:** "Is duplicate-window behavior only a sandbox helper concern?"
> **Domain expert:** "No — **Duplicate protection** is expected for future write-capable commands."
>
> **Dev:** "Should a local webhook receiver be part of sandbox resource management?"
> **Domain expert:** "No — **Sandbox resource management** covers Authorize.Net sandbox objects; local receiving and validation belong to **Webhook tooling**."
>
> **Dev:** "Does v1 include webhook commands?"
> **Domain expert:** "No — **Webhook tooling** is outside the **Read-first release**."

## Flagged ambiguities

- "toolkit" implies a broader set of surfaces than the current product boundary — resolved: the canonical product is **Authorize.Net operations CLI**.
- "authnet-cli" could be mistaken for the executable name — resolved: **authnet** is the executable and **authnet-cli** is the project name.
- "user" is too broad for the primary v1 audience — resolved: the canonical human actor is **Operator**.
- "profile name" could be treated as a secret field — resolved: **Profile name** is visible and must not contain sensitive labels.
- "credentials" could imply secrets embedded in profile files — resolved: profiles reference a **Credential source** and do not store plaintext secrets by default.
- "production profile" could be easy to miss in profile output — resolved: production profiles require a textual **Production marker**.
- "plaintext mode" could be treated as a convenience feature — resolved: the CLI must not create **Plaintext credential storage**.
- "environment" and "profile" can be conflated — resolved: a **Profile** is the named access target, while **Environment classification** is its sandbox-or-production designation.
- "default profile" could imply any saved target — resolved: a **Default profile** is sandbox-only.
- "setup" could imply a wizard-only workflow — resolved: **Profile setup** supports **Interactive setup** and **Non-interactive setup**.
- "profile config" could imply secret storage — resolved: **Profile config** stores non-secret profile data only.
- "config path" could be hidden in documentation — resolved: **Path inspection** exposes the **Config directory**.
- "version command" could imply update checking — resolved: v1 includes local **Version reporting**, not update checks.
- "shell completion" could leak profile names — resolved: v1 excludes **Dynamic profile completion**.
- "operations" can imply financial mutation — resolved: v1 is a **Read-first release** with sandbox-only write helpers.
- "dry run" could be added as a generic v1 safety flag — resolved: **Dry run** is deferred until mutation commands exist.
- "write" could be limited to money movement — resolved: a **Production write** is any production mutation.
- "logged to disk" could be read too narrowly — resolved: **CLI-controlled persistence** must never contain **Sensitive payment data** or **Customer PII**.
- "PCI-safe defaults" could be mistaken for a compliance claim — resolved: the CLI does not make a **Compliance guarantee**.
- "redacted diagnostic log" could appear safe enough for v1 — resolved: the **Read-first release** does not create **Diagnostic logs**.
- "CLI network access" could include update checks or docs fetches — resolved: the **Read-first release** makes no **External network calls**.
- "disabled telemetry" could be treated as harmless — resolved: the **Read-first release** includes no **Telemetry**.
- "Authorize.Net support" could imply legacy protocol support — resolved: the CLI uses the **Current JSON API** only.
- "read-only" could imply no meaningful risk — resolved: **Production reads** are allowed in v1, but must not persist **Sensitive payment data** or **Customer PII**.
- "JSON output" could imply raw API output — resolved: command output is **Redacted output** by default.
- "JSON output" could also imply bespoke per-command structures — resolved: machine-readable output uses a **JSON envelope**.
- "JSON output" could be selected implicitly by TTY detection — resolved: JSON requires **Explicit JSON output**.
- "JSON failure" could be treated as a stderr diagnostic — resolved: **Structured failures** are part of **Explicit JSON output** on stdout.
- "warning" could be human-only text — resolved: a **JSON envelope** can include **Structured warnings**.
- "automation usage" could rely on remembering several flags — resolved: **Automation mode** provides predictable script, CI, and agent behavior.
- "automation profile selection" could inherit production accidentally — resolved: **Automation mode** may use a sandbox **Default profile**, but production must be explicit.
- "failure" could collapse into a single exit code — resolved: the CLI uses an **Exit-code taxonomy**.
- "human output" and "JSON output" could be treated as equal contracts — resolved: **Human-readable output** is for operators, while the **Stable JSON contract** is for automation.
- "agent support" could imply a full skill package in v1 — resolved: v1 includes an **Agent usage guide**, while an **Installable agent skill** follows later.
- "release" could mean source-only availability — resolved: v1 includes a **Canonical release** and an initial **Convenience install channel**.
- "integrity metadata" could imply signatures and SBOMs are v1 blockers — resolved: v1 requires **Release checksums**, while signatures and SBOMs are **Future release attestations**.
- "raw output" could be treated as a general debugging escape hatch — resolved: **Raw response mode** is sandbox-only.
- "passthrough" could bypass product safety — resolved: v1 exposes **Curated commands**, not raw API passthrough.
- "command naming" could drift into shorthand — resolved: **Canonical command names** use formal Authorize.Net terminology.
- "transaction list" could mean historical or unsettled transactions — resolved: **Settled transaction history** and **Unsettled transaction set** are distinct concepts.
- "list" could imply unbounded fetching — resolved: list-style commands use **Bounded pagination**.
- "relative date" could make automation ambiguous — resolved: **Time ranges** are emitted as **Resolved timestamps**.
- "timezone default" could imply merchant timezone or UTC — resolved: unspecified time input uses **Operator-local time** and output includes explicit offsets.
- "redaction" could be treated as renderer-only — resolved: curated command output is built from **Safe normalized results**.
- "customer profile" could imply all nested customer data — resolved: **Customer profile metadata** is distinct from **Nested customer profile detail**.
- "subscription" could be confused with a transaction or customer profile — resolved: a **Subscription** is a recurring billing plan that generates **Subscription payments** according to a **Payment schedule**.
- "recurring billing support" could imply mutation from day one — resolved: future support starts with **Subscription inspection** before **Subscription mutation**.
- "response-code explanation" could imply live docs scraping — resolved: use a **Curated response-code reference** that must be kept current with official sources.
- "response code" could imply only transaction response codes — resolved: **Response-code explanation** covers gateway and API codes with explicit **Code family**.
- "recommended next step" could be mistaken for official documentation — resolved: **Response-code explanation** separates **Source meaning** from **Recommended next steps**.
- "confirmation" could be mistaken for authorization — resolved: **Safety policy** permits or denies the action, while **Confirmation** acknowledges an allowed high-risk action.
- "automation confirmation" could bypass prompts with a flag — resolved: **Non-interactive execution** of high-risk actions requires **Safety policy** permission.
- "sandbox helper" can mean either quick test transactions or full persisted-resource lifecycle management — resolved: v1 includes **Sandbox test helpers**, while **Sandbox resource management** follows immediately after.
- "sandbox helper" could mean fake local responses — resolved: **Sandbox test helpers** contact the real sandbox gateway; **Offline simulation** is test-only.
- "integration test" could imply mandatory networked tests — resolved: **Sandbox integration tests** are opt-in.
- "sandbox CI" could imply required PR checks — resolved: **Sandbox integration CI** is manual or scheduled.
- "golden test" could freeze every human table detail — resolved: **JSON golden tests** cover the stable contract, while **Human-output tests** are selective.
- "redaction" could rely on review only — resolved: **Redaction tests** are mandatory.
- "sandbox payment coverage" could expand indefinitely — resolved: v1 covers **Card test scenarios**, while eCheck and partial authorization are **Future payment test scenarios**.
- "resource management" could include local webhook infrastructure — resolved: **Sandbox resource management** covers gateway-side sandbox objects, while **Webhook tooling** is separate.
