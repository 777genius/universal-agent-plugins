# Project installation: owner scope clarification

Accepted owner instruction, 2026-10-23. This document records the scope of the
current project-installation program; it does not change release availability.

## Supported clients and the narrow VS Code exception

The owner explicitly requested project installation for **all supported agents**.
The program therefore covers the current supported client set: Claude, Codex,
Cursor, Gemini, OpenCode, Copilot, Kiro, Grok, Qoder, Cline, Windsurf, Kimi,
ChatGPT, and VS Code. Each client receives its own implementation PR stacked
on the project-installation base PR.

The repository's 2026-10-16 VS Code freeze required a new explicit owner
instruction before reopening work. The current all-supported-agents request is
that instruction for **project-scope installation only**. It authorizes the
narrow adapter, configuration, protocol, fixture, and test changes necessary to
implement and verify that project-installation path. Preserve unrelated VS Code
implementation, behavior, and source; the rest of the freeze remains in force.
This exception does not independently authorize new VS Code hooks, unrelated
profile features, publishing, or a release.

## Implementation and review gates

Use the existing dependency solver, manifest and lock formats, immutable
package store, digest/security checks, state receipts, and transaction engine.
Keep project mutable ownership separate from the existing user environment.
Preserve the user profile as user authority; a directory inside a project must
not be disguised as a new HOME or native user profile.

Before coding each slice, perform two rounds of independent plan review and
fixes. Review implementation independently, address findings, and commit/push
incrementally. A fixture proves its contract; real-client discovery, startup,
MCP operation, authentication, and release evidence require their own observed
proof. A client without a native project registry must expose its supported
component or explicit preparation path and state the remaining limitation.

For this program, the owner's current model selection is **GPT-6.1 with high or
xhigh reasoning** for workers and reviewers. This supersedes older repository
model instructions for this program only. It does not relax review, evidence,
filesystem authority, or release gates.

Preserve useful legacy implementation and tests under
[the capability-preservation contract](AUTHORING_CAPABILITY_PRESERVATION.md).
Existing source, historical releases, current executable availability, and
future qualification remain distinct.

## Registry security providers

The same owner update defers Gen Agent Trust Hub, Socket, and Snyk registry
provider integration. The concrete deferred work and acceptance criteria are
recorded in [Registry security providers backlog](REGISTRY_SECURITY_PROVIDERS_BACKLOG.md).
The current project program does not add provider dependencies, provider CI
jobs, or third-party scans.
