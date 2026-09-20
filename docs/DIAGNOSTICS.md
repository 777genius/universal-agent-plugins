# Diagnostics Contract

This document records the current user-facing diagnostics contract for shipped `plugin-kit-ai` surfaces.

## Goal

Stabilize failure **families** and their high-signal phrasing without freezing every incidental detail of stderr output.

## Runtime Failure Families

- unknown invocation resolution
- unknown registered handler
- decode failures
- encode failures
- middleware/handler errors surfaced as returned error strings

Contract-sensitive examples:

- `unknown invocation "..."` from runtime resolver failures
- `decode codex notify input: ...` for malformed or missing Codex notify payloads

## Validate Failure Families

`cli/internal/validate` exposes stable failure kinds for:

- `unknown_platform`
- `cannot_infer_platform`
- `required_file_missing`
- `forbidden_file_present`
- `build_failed`

The exact build tool output may vary, but the failure kind and the leading `go build <target>:` framing are contract-sensitive.

## Install Failure Families

`plugininstall/domain.ExitCode` is the stable CLI-facing class surface:

- `ExitUsage`
- `ExitRelease`
- `ExitNetwork`
- `ExitChecksum`
- `ExitFS`
- `ExitAmbiguous`

Detailed text may evolve, but the exit code family and the core reason category are part of the contract.

High-signal install diagnostics intentionally cover:

- `--tag` / `--latest` selection misuse
- prerelease refusal without `--pre`
- no matching asset for the requested GOOS/GOARCH
- missing `checksums.txt`
- checksum mismatch
- destination already exists
- destination path or install dir is invalid or not writable

Install success output is also part of the stable CLI contract at a high level:

- first line identifies the final installed file path
- subsequent lines identify release ref/source, asset, and target GOOS/GOARCH
- overwrite status is printed only when an existing file was replaced

This contract covers verified installation of third-party plugin binaries only. It does not imply a self-update or auto-update subsystem for the `plugin-kit-ai` CLI itself.
Supported and refused release layouts are documented in [INSTALL_COMPATIBILITY.md](./INSTALL_COMPATIBILITY.md).

## Non-Contract Debug Data

These are intentionally **not** public contract:

- full stack traces
- transport-layer retry wording
- verbose external CLI logs
- `PLUGIN_KIT_AI_E2E_TRACE` contents used by repository tests

They may be used for troubleshooting and E2E assertions, but changes to them do not require compatibility handling.

## Stable-Candidate Review Notes

The declared `v1` candidate set is reviewed against this diagnostics policy as follows:

- SDK root runtime surface:
  - reviewed through runtime resolver/decode/handler regression tests
  - stable failure families are limited to invocation resolution, decode/encode, and handler-facing errors
- Claude and Codex event surfaces:
  - reviewed through platform-specific runtime tests plus real CLI smoke where available
  - hook traces remain repository-only debug data, not public contract
- CLI command set:
  - `validate` and `install` expose the meaningful stable failure families
  - `init`, `capabilities`, and `version` are only reviewed for success-shape and deterministic output expectations in the current pre-`v1` contract
