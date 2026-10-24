# Eight-client integration seam — independent R2

## Decision and scope

**GO** for root's merge-only production integration `e18b94a68ea280414e199a286142dd335fdfdf7d`, plus documentation through `d194b7f00e5cb2cff764a1d7a1ddeb74992afda4` (including `bbc0473ae554491eafac5d9b8d50be6b9465bbde`). Reviewer codex_claude, 2026-10-23. This review covers the requested merge resolutions, preservation of independently reviewed leaf code/goldens, and scope documentation. It does not replace final leaf reviews, establish all 14 clients complete, or establish native/runtime/release qualification. No production code changed or native process executed by this review.

## Production seam

The nativeconfig conflict resolution preserves `CodecCopilot` and `CodecCodex` in both the declarations and `supportedCodec` union, alongside every previously supported codec. Copilot's MCP projection path remains present; the Codex-specific TOML collection, authentication-option fencing and project projection from the other parent remain present. The merge does not silently route either client through another dialect.

The domain resolution retains both Cursor and Copilot `withProjectScope(..., LifecycleNativeConfig)` wrappers, including Copilot's existing `withoutCommands` policy. Existing Codex, Claude, Gemini, OpenCode and Kiro project wrappers survive; ChatGPT's explicit unsupported project boundary and the still-unmerged clients' prior policies remain intact. The wrappers preserve user lifecycle defaults and the existing traits rather than replacing them with project-only values.

Independent byte comparison against the reviewed Copilot parent `3f4a527cfe33cada14e6c53be6be233664efd8aa` found 20 source/test/golden files identical. The seven differing shared files consist of the combined nativeconfig project/types files, domain policy, and four compatibility goldens; their differences preserve the already integrated Codex/other-client work. Independent comparison of all 36 selected Kiro client/project fixture/provider/SDK files against reviewed leaf `0ec535e2dd12e9d7ee965c916714cd2ae4ba3c76` found no byte differences. The four planner goldens preserve every non-Scopes JSON value from both merge parents; their combined scope changes reflect the admitted client union without changing component support or readiness facts. No reviewed assertion or adapter leaf was deleted by these merge resolutions.

Root reported the combined bounded selection PASS at this frozen production SHA: nativeconfig 0.039s, domain 0.003s, planner 0.018s, and public CLI 29.299s. Those are root-run results, not an independent rerun. No extra broad compilation was necessary for this merge-only check.

## Documentation R2

The README, UAP dependency guide and new `docs/PROJECT_INSTALLATION.md` correctly mark project installation source-only/unreleased. They keep user scope as the default, require explicit project selection, document invocation-directory behavior when the project root flag is omitted, and avoid implicit parent/Git-root discovery. Examples consistently keep add, locked install, update, repair, removal and recovery in the selected environment. The source text correctly distinguishes a **local** directory/archive imperative flow from a ready HTTPS archive that can contribute saved dependency intent; the final `d194b7f0` wording closes that distinction.

The portable `uap.toml`/`uap.lock` pair is distinguished from local `.uap` ownership, journals, managed packages and retained data. Project export/import examples explicitly select their source/destination project and a separate exchange directory, preserve the exact agreement pair, and do not claim to transfer credentials, settings, receipts or plugin data. Cold restore remains subject to exact-byte availability and source access. Removal retains data/content by default and recovery stays pinned to the same selected authority.

Prepared configuration is accurately distinguished from native discovery, authentication, trust and runtime. Documentation constrains `--activation-complete` to adapter-supported attestation and does not let it override unsupported, shadowed or unobserved project behavior. User Cursor's existing operator-attestation behavior remains labeled user-only. ChatGPT project requests remain explicitly unsupported, including no-save. The docs defer client-specific qualification to supported-target reporting and reviewed per-client evidence instead of claiming every client is implemented or genuinely native-tested.

The docs-only diff passes `git diff --check`. No new tests were required for this prose change.
