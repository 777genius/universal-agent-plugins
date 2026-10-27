# Grok shared HTTP client fixture: reviewed validation

Reviewed code: `74f72491fdbeb722e845de71217e9d1751330020`.
Tree: `53b0e7f46404658bd385b92edddea12522c247dc`.
Base: `a1af0da362f020111a3af6a16e50cd265a708224`.
Approved plan commit: `7e5e5629d3f730df7bb2ee17bf2765882cd74112`.

Two independent GPT-6.1 Sol xhigh reviewers approved the exact plan before code
and the final implementation after inspecting executor receipts. No blocking
source findings remain.

The existing Grok child environment receives `GROK_SAMPLER_SHARED_CLIENT=0`
only when its fixture holds the private provider. Eight concrete client/provider
cases verify routing, ambient isolation, no duplicate entries, retained original
HOME/profile/state/Git/key/updater values and no process-global mutation.

The provider source stays byte-identical, SHA256
`865265e28731ceee3816ab1d5c4fc137d29d859e6851675d90fbcc867f1daa88`.
Its exact authenticated POST route, request grammar, response, failure latch,
cancellation and shutdown remain unchanged. GET `/` remains rejected. No client
pin, config, trust/permission, command, timeout or qualification flag changed.

## Actual local inert gates

- Focused Grok/shared CLI Go contracts: PASS, package1.138 seconds.
- Python runtime admission: 38 tests PASS,5.503 seconds.
- Affected all-enabled lint: zero issues.
- Unfiltered revive/funlen/gocyclo/gocognit/dupl/depguard: zero issues.
- Original a1af native archive: all20 member bodies and the summary admitted
  unchanged; failed status and allfive/runtime/auth/model false flags preserved.
- gofmt, whitespace, source-manifest and unchanged-provider checks passed.

Only inert repository contracts and artifact admission ran locally; no vendor
binary or scanner ran. The public source supports this configuration experiment,
while equivalence to the shipping1.0.70/f50904eaee6c build remains unproved.
A new genuine disposable hosted capture must show whether the pinned binary
honors the setting and what request follows. The previous failure is not promoted
to success. This validation makes no lifecycle, authentication, model, main-merge,
release or deferred VS Code claim.

## Exact review and execution records

- `/tmp/uap-additional-grok-shared-client-code-R1.md`: SHA256 `15f043736ab0deadf9b985eafbb8fd7bf148e3853cb81d5631b249a06a5157cc`.
- `/tmp/uap-grok-shared-client-code-r2-review.json`: SHA256 `8e842b6b91bc559f53f47893d14ee0b7427d42b2955ef868876d46b6897ef842`.
- `/tmp/uap-additional-final-serial-gates-1d4m_qyl/report.json`: SHA256 `1ca286a35b6c73e4dc32b6ef5525741aa8b323bb726ceda86968c06c017e00de`.
- `/tmp/uap-additional-final-serial-gates-1d4m_qyl/grok-unchanged-old-replay.json`: SHA256 `82ccf6beb6b90bd35bfac3a0ef2eb983f62286a82bc99ce86c0909503b823479`.

Local receipt paths identify this execution. The committed source, approved plan
and this validation document preserve the reviewable repository history.
