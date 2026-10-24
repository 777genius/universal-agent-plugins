# Cline post-observation plan: independent review receipts

## R1, 2026-10-24 — NO-GO and adopted corrections

Reviewed exact local `370b7365ce365130f9b0a4f75ec2dc9ecc8e559a`, published as
`4461dfa9445fc9b8731a77214295fc8215c9d3bf` with identical tree. This is an
actual review record, not a substitute for review of the revised plan.

Independent critic `apm_vercel_scope` issued **NO-GO** on two concrete source/
plan mismatches, and root agreed both required revision before product code:

1. `usecase.preflightActivation` constructs an early ActivationRequest without
   `input.ActivationComplete`. The adapter's later provider guard cannot meet
   the promised no-state-change boundary if core already recorded an attempt.
   Adopt the one-field shared forwarding correction with actual public-core
   first-add/replay/update/repair/reopen state/native-effects regressions. A
   direct providers-only test cannot qualify that earlier boundary. Root
   explicitly assigned this coordinated shared delta to the Cline group.
2. Current `nativeconfig.Server`/Cline codec cannot emit the disabled field, so
   an actual UAP-produced file cannot satisfy the plan's disabled-marker native
   fixture promise unchanged. Adopt an explicitly reviewed read-only config
   observation contract for actual unedited UAP output instead of inventing a
   production option for testing or post-editing receipt-owned bytes. The
   revised plan traces exact immutable Cline dispatch/resolver/transport/update
   source, requires the fixed non-session command, and honestly records
   `disabled:false`. It makes no MCP connection/initialization claim.

The critic also required user guidance that the explicit MCP file selects the
whole source; user registrations are not merged/copied into that session. This
is added. The architecture otherwise received GO: separate original profile and
project namespace, exact fixed prepared path bypassing ambient user helpers,
distinct receipt kinds, shared combined mutation/recovery, Prepared lifecycle,
local verification versus discovery, no CLI requirement for removal, and
separate unique-marker versus same-name precedence stages. The reviewer
independently verified all 29 preserved member lengths/hashes and the original
false qualification flags. No native/source acquisition was performed by that
reviewer. The implementing worker completed the six-file immutable source
reread for the revised plan; its review is a separate R2 gate.

## R2

Pending independent review of the revised frozen plan. No product adapter or
shared preflight code may be implemented until both final R2 GOs are recorded.
