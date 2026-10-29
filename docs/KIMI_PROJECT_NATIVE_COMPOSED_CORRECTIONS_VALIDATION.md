# Kimi scoped selection and native capture corrections: validation

Composed source commit: `c1a3720f341ae2e746051bdcd2199567995627b0`.
Composed tree: `83ebe639b645e367d5b4707d27c64714d83027e8`.
Existing PR524 base: `b2bb2b7f85b5461420f932478b197905e6fc5ad9`.

## Reviewed source and reason

The genuine b2bb component capture completed repeat-add and unchanged update,
then showed intact repair planning removal of selected project MCP. Kimi's old
managed-selection policy always selected the user-format .kimi-plugin/plugin.json,
while its project artifact uses root mcp.json. Reading the wrong layout erased
recorded selection during repair preflight. The native byte-stability assertion
correctly failed and remains unchanged.

Production fix `f09fbb1e52960c56e6b731aaf2aafc1c2aefd01b` adds an optional scoped
selection port and carries explicit held scope through the guarded usecase and
provider into Kimi's layout policy. It preserves the old user API/layout, optional
legacy ports, exact target/digest authority, both artifact verifications, original
profile checks and existing damaged-artifact reconstruction gates. A refused
scoped policy never retries a user layout.

The same genuine b2bb single-tool capture preserved one completion request but
no owned tool call. Its original7839-byte request contains prompt_cache_key;
whole-request SHA256 is
`ddd8a402037716fe582f854dcf5d5dae1ebb902dc6b3a962466e4d34874c4de7`.
The old strict provider rejected that known client metadata before issuing the
owned first response. Exact pinned source proves generated session ID -> profile
cache key -> OpenAI request field. Fix `20921c06b3212f1243bde77f7d6cbbb35adcdeed`,
with Go1.22 test compatibility correction `30f5dfef137604b2a2367be2a125bb724fe6ab57`,
admits only the optional observed44-byte canonical string and requires unchanged
presence/value on the second turn. It grants no credential/tool authority and
makes no unobserved native API session join. Authentication, route, body/model/tool
schema, owned call/result facts, counts, SSE and first-error latch remain intact.

Separately reviewed poll diagnostic `8311c8e9f571e9fa272abc2daed1a2f367ad160e`
retains only finite scalar facts at the original multiple-pending and exhaustion
failures. It preserves manual approval validation/actions, polling/timing, ordinary
supervision/quarantine and all qualification decisions.

Each slice had two independent GPT-6.1 Sol xhigh plan GOs before implementation
and two independent exact code GOs after meaningful executor verification. The
composition preserves all20 reviewed source/document file hashes without conflict.
No native collector acceptance, native byte-stability gate, VSCode source, lint
configuration, authentication/trust setting or release policy was relaxed.

## Actual inert validation

- Old product: actual composed public CLI and public Engine intact-repair tests
  failed on native skill/MCP byte changes. Current production source passes both
  public paths, retaining explicit native MCP selection and persisted identities.
- Final frozen SDK intact-repair regression: PASS0.097seconds. Its complete user
  and B tree comparison covers membership and file bytes, not mode/inode/link
  metadata. Final affected whole-file lint and unfiltered six-linter gate:0issues.
- CLI, Kimi adapter, provider and usecase focused gates passed on the identical
  reviewed files; both CLI lint gates reported0issues. Exact chronology and
  earlier fixture/setup/test-helper failures are retained in executor receipts.
- Old provider: permanent canonical metadata regression plus original unchanged
  request replay failed through the actual parser and actual in-process provider
  with400 before issued. Corrected provider: exact original request reaches only
  the expected owned first SSE response, with identical captured bytes and no
  completed result/callback/sentinel claim.
- Final Go1.22-compatible provider contracts, including that unchanged raw replay,
  PASS0.017seconds; both final lint gates report0issues. Controls cover absent/same
  metadata,19 invalid type/shape values, duplicate JSON, unknown/foreign authority,
  changed/added/removed second-turn metadata and inability to clear a refusal.
- Poll diagnostic:57 inert Kimi Python contracts and27 runtime contracts PASS.
- Original23-member failed single-tool artifact re-admits with identical bodies
  and summary; all failure/qualification flags remain unchanged.
- Final composed-source Go compile and Kimi/shared CLI contracts PASS0.017seconds.
- gofmt, whitespace and exact reviewed-source hash comparisons pass. Session-bearing
  transient replay files are removed from the source checkout; no raw session key
  is committed by this correction.

Retained saved-Git CLI lifecycle cases encountered the executor's missing
/proc/PID/task/TID/children capability before acquisition. That environmental
failure is preserved and is not counted as a pass; no ordinary containment guard
was changed. Genuine hosted CI retains that execution responsibility.

No vendor/client/scanner process ran locally. Existing native component/single-tool
failures remain failed and unqualified. New actual disposable hosted captures must
establish the effect of the fixes and the remaining user/A/B lifecycle and MCP
boundaries. This validation does not claim authentication, real-model operation,
complete native qualification, main merge, release or deferred VSCode completion.

## Exact review and executor records

- `/tmp/uap-kimi-scoped-selection-f09f-source-manifest.json`: SHA256 `9ecb212ca8674580140ca55b7267ad359daf265ea010b576f799be72741a21d7`.
- `/tmp/uap-kimi-scoped-selection-f09f-code-R1.md`: SHA256 `7a0102a2b1dcd7d6a1eae04439a667faeab20df2202769c91760257e14da9acd`.
- `/tmp/uap-kimi-scoped-selection-code-r2-review.json`: SHA256 `bdcc89c716026c5f723812dee235c204f58b04bfbcf994e4dcbdb42b0df9d733`.
- `/tmp/uap-additional-kimi-cache-key-code-R1.md`: SHA256 `6a13e71db5947b89543b52e0b117c2230b6430f46ee991c1e26a7d1163ba91b9`.
- `/tmp/uap-kimi-cache-key-go122-delta-r2-review.json`: SHA256 `b75b3fa369d3e5ace1a11ded6b8b244801932b4353ef08ff56f6f833d9c836b5`.
- `/tmp/uap-additional-kimi-composition-code-R1.md`: SHA256 `d06ea64110f288d74de89f5fd97ee8ca9c39406167d4fba3918dc3a8434a67b9`.
- `/tmp/uap-kimi-composed-source-code-r2-review.json`: SHA256 `c208f636c5f22e1b654aa268b4b8e7682ed3696cf21d3d05b19d8b44f5d1f8ee`.
- `/tmp/uap-additional-final-serial-gates-1d4m_qyl/report.json`: SHA256 `1ca286a35b6c73e4dc32b6ef5525741aa8b323bb726ceda86968c06c017e00de`.
- `/tmp/uap-additional-final-serial-gates-1d4m_qyl/kimi-30f-report.json`: SHA256 `51053bf6d849d31679fe9be7f7cadc46fce654f961cdd6b1a8721f56d38e0a8f`.
- `/tmp/uap-additional-final-serial-gates-1d4m_qyl/kimi-composed-go.json`: SHA256 `9a3c437119aa1c945e2fbfe0fabc280387d63189c520d938825a9c3abc49d55e`.
- `/tmp/uap-additional-final-serial-gates-1d4m_qyl/kimi-call-unchanged-old-replay.json`: SHA256 `04ceb745d9019c529c3e2ed484a4575646fb567ea35c91b3d5a7586433140944`.

Local paths identify this execution; committed source, plans and this validation document preserve the durable repository record.
