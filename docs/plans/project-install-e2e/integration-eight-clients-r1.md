# Eight-client integration merge — independent R1

Reviewed frozen aggregate `e18b94a68ea280414e199a286142dd335fdfdf7d`, Kiro merge `0c2edce6` and Copilot merge `e18b94a6` on 2026-10-23. This is limited to root-authored integration preservation; reviewed client implementations retain their own independent GO reports. **GO.** No source was changed or broad additional test run for this review.

The Copilot merge's `nativeconfig.supportedCodec` retains the complete earlier set, including `CodecCodex`, and adds `CodecCopilot`. Both constants and dispatch branches remain present. Earlier Codex-only authentication/TOML checks and options remain intact; Copilot's new project transport codec remains the same reviewed implementation. No codec was substituted for another.

The domain table retains independent project declarations for Cursor and Copilot, plus the previously reviewed Codex, Claude, Gemini and OpenCode declarations and the new reviewed Kiro declaration. ChatGPT keeps its explicit project refusal/manual user boundary; no project capability is invented for it or any later client. Copilot's unsupported command wrapper and existing user shared-backend policy remain intact.

Independent byte comparisons show all **13 Copilot client files** identical to reviewed leaf `3f4a527c`, all **29 Kiro client files** identical to reviewed leaf `0ec535e2`, and both public Copilot composed E2E files identical to the final reviewed leaf. The final exact-package and direct-delivery corrections therefore survive integration. The first-parent merge diffs introduce only their reviewed client/native codec/domain/tests/evidence changes, with no other semantic conflict resolution.

For each of the four compatibility goldens, independently decoded and compared both sides of each merge: all 14 client rows remain in order. Kiro merge `0c2edce6` changes only Kiro's scope list from `[user]` to `[user,project]`; Copilot merge `e18b94a6` changes only Copilot's corresponding list. Every other field and row is unchanged in each comparison, preserving all earlier client scope declarations.

The documentation conflict preserves the earlier OpenCode row and adds only Copilot's reviewed two-GO/native-pending status. Pending native discovery/trust/MCP/runtime evidence is not relabeled by this merge. Parent's bounded combined regression run remains separate execution evidence; no main/release publication authority is added by this integration GO.

## Parent documentation follow-up R1

Reviewed documentation-only `bbc0473ae554491eafac5d9b8d50be6b9465bbde` plus clarification `d194b7f00e5cb2cff764a1d7a1ddeb74992afda4`. **GO.** README and dependency guide preserve default user scope, explicit project selection, invocation-directory fallback without ancestor search, and selected-scope export/import with a separate exchange directory. The new project guide correctly separates portable intent from local `.uap` ownership/data/recovery, exact recorded repair/removal, immutable cache, unsupported ChatGPT and per-client prepared/native activation limits. Attestation does not bypass a project adapter's admission; there is no automatic trust/auth action or all-14/native/runtime/release completion claim.

Requested one wording clarification: “a local path or archive” could misleadingly include eligible remote HTTPS archives, whose saved intent is supported. `d194b7f0` explicitly says “a local directory or local archive,” preserving that distinction. Scope/root commands, maintenance/recovery and transfer examples are consistent with the reviewed CLI contracts. No broader test run was needed for these prose-only changes; the independently reported bounded aggregate test run remains separate execution evidence.
