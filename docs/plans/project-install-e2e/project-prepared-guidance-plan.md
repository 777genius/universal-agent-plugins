# Prepared project activation guidance — pre-code plan

Root assigned this shared component after the actual composed Grok fixture
exposed a misleading deps-status action. Two independent plan GOs are required
before implementation, followed by two independent code GOs and fresh tests.

Current dependencyNextActions appends attest_activation/--activation-complete
for every activation_manual/prepared fact. Project Grok, Cline MCP and other
prepared project lanes reject that promotion in the composed provider; status
must not recommend it. This change is presentation only: preserve all provider,
attestation, native discovery, failed/disabled state and readiness gates.

Add an internal nonserialized Scope field to dependencyStatusFact. Seed it from
App.scope for desired and retained/orphan environment facts; where inspected
bindings supply scope, preserve project classification if EITHER context is
project (never downgrade an app-project fact to user because malformed metadata
says user). Empty/ordinary user contexts retain existing user behavior.

For project-scoped manual/prepared activation, emit a review_project_activation
next action explaining that the selected project's client instructions and
native loading need review/verification; do not emit --activation-complete or a
user-scope command without the selected project context. No speculative client-ID
switch or new core policy port is needed: all project-prepared guidance can be
conservative. Preserve explicit SDK next actions and existing authentication
next-action behavior; this slice changes only the automatic status activation
suggestion. Do not manufacture active/installed/attested state, launch a client,
mutate saved state or run any native/trust/auth/model command.

Propagate scope before both appendBindingStatusFacts and appendReceiptStatus
compute actions, covering desired locked bindings and retained/orphan receipts.
Keep JSON wire schema unchanged (Scope json:"-"); the existing outer status
scope remains authoritative to the user. Add focused status-helper tests for
user compatibility, project prepared/manual, both activation/auth facts, desired
and orphan binding paths, inconsistent project/user metadata, missing binding
and SDK action preservation. Add a composed project deps-status assertion where
available to prove no activation-complete suggestion, while provider refusal
remains tested on client leaves.

Run focused agentpluginscli tests and relevant lint; no broad native or platform
suite is necessary for string guidance. Checkpoint immediately after approved
plan records and source increments through internal expected-head publication;
root merges this shared change into the project feature, never main/release.
