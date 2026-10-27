# Copilot exact message roles: validation

Reviewed implementation: `61d11984c18a4d81d6dfe212b093e81b7b84fde3`, tree
`7c83ae8e4515835ebfc46ff3418a95a7dce9fb4a`. The preceding plan-only commit is
`105c11de6bfc2aae565b7e3770d00d520ab02568`; the published source baseline is
`9b8ddc5332d70ca0c8d39c41c5ce60f3bba1c6e7`.

## Independent reviews and exact scope

Two independent critics, native_finish (R1) and kimi_cline_finish (R2), read
the complete plan and gave GO before implementation. Both subsequently read
the complete changed files and the 29-addition/1-deletion implementation
delta and gave code GO, conditional on the checks recorded below. Their
reviews found no outstanding issue in this correction.

| File | Reviewed SHA-256 |
| --- | --- |
| `repotests/agentplugins_copilot_project_native_provider_test.go` | `85beec456341a1f0aa63f1dab2a60d91698835f5b38ce239e267a3e81558cc3c` |
| `repotests/agentplugins_copilot_project_native_contract_test.go` | `c4cd17cabf35583584666281144b7cb6e8c9aede73cdc23ede3bf1fb57c10e88` |

The provider now requires exact membership in the existing five role labels.
The full-request contract exercises all five canonical roles, four composite
strings, and empty, case-changed, whitespace-suffixed, null and numeric values.
The canonical tool message supplies its existing required tool-call identity.
All other message, envelope, tool, provider and original-configuration checks
remain unchanged. No production or shared validator source changed.

## Actual inert checks, 2026-10-27

The regression was first executed against an overlay that replaced only the
provider file with the exact published baseline. The new full-request test
failed as expected, with `system|developer` incorrectly admitted and a nil
validation error. This is an executed counterexample at the actual request
validator; it is not a copied role predicate or a vendor observation.

| Check | Actual result |
| --- | --- |
| Baseline provider overlay, `TestCopilotProjectNativeMessageRolesAreExactInFullRequests` | Expected failure, exit 1; first composite role admitted; Go package time 0.011 s |
| Corrected source, `^(TestCopilot\|TestProjectNativeRuntimeValidationContracts)` | PASS, exit 0; Go package time 0.044 s |
| All enabled affected-code lint, `--new-from-rev=9b8ddc5332d70ca0c8d39c41c5ce60f3bba1c6e7 ./repotests` | 0 issues, exit 0 |
| Unfiltered `revive,funlen,gocyclo,gocognit,dupl,depguard` on `./repotests` | 0 issues, exit 0 |
| Formatting and diff whitespace | Clean |

Go 1.26.10 and golangci-lint 2.13.2 used the existing offline module cache.
No acquired vendor executable ran locally. Each validation checked for other
Go work before starting and would terminate only its own process group if a
competing validation started; neither run reported a conflict. The reviewed
source stayed unchanged and clean throughout the executable checks.

Local closed check logs have SHA-256 values:

- Baseline counterexample: `d9653603861c7dbd1b71d8b3e4aa290c67a1d13082249650ceade258c71af469`.
- Corrected focused contracts: `631aff5d0f8d4530a90d27fcf9deeb324ac202300b23839893e0f0a0d4e0783a`.
- Each lint log (`0 issues.`): `e92606b0bf483111dff0a120c315ea165821348f31365020e2468a0059095c47`.

These checks close this narrow static enum correction. They do not claim the
unfiltered all-enabled integration lint passed: the separate nine-lint
amendment still owns its remaining findings. They do not provide native
session, complete lifecycle, vendor-authentication or real-model evidence.
The pending genuine baseline capture remains necessary; publication and
semantic integration retain the plan's fresh-head and writer-coordination
requirements. No release or VS Code change is included.
