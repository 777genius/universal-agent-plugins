# Kiro — selected installation and native trust (21 Nov 2026)

## Baseline and goal
Main already contains the Kiro user-scope native Skills/MCP transaction and ACP observer. PR #927 merged and proved a **Prepared**, no-auth Kiro lifecycle. Current Project delivery deliberately remains Prepared because workspace trust, custom-agent selection and native discovery are separate authorities. Open #931 and #1016 are E2E/research, not an automatic-trust implementation.

One `agentplugins add ... --target kiro` should install selected Kiro components without manual JSON editing, and the confirmed installation plan should supply only the selected package's MCP tool consent where Kiro provides a documented, ownership-safe config. No vendor login, credential acquisition, broad agent shell trust, unrelated client changes, or native trust database hacks.

## Plan R1: architecture criticism and resolution
**Risk:** a new installer, generic `ClientID == Kiro` branch or second JSON engine violates SOLID/DRY. **Resolution:** keep changes in `clients/kiro`, reuse the existing `shared.ProjectMCPServers`, Kiro exact JSONC mutation, plan ownership digests, native receipts and rollback. A scoped projected-server transform may be required; no new state store or adapter interface.

## Plan R2: trust/security criticism and resolution
**Risk:** `autoApprove: ["*"]` silently grants every future tool of one server, and writing a workspace trust hash is undocumented. Kiro's official security guidance recommends only individually audited safe MCP tools. **Resolution:** do not add broad wildcard approval, global trust-all, permissions.yaml changes or undocumented workspace trust file writes. Reuse explicitly authored per-server allowlists only when the confirmed plan authorizes them; validate and preserve the selected servers and existing foreign/deny settings. Kiro's documented `--trust-workspace` lasts only one V3 CLI launch (2.34+); it does not persist IDE or later workspace trust. Project delivery must stay Prepared until native trust and discovery can be proven.

## Plan R3: acceptance criticism and resolution
**Risk:** a green compilation, mocked ACP response, skipped native job or a copied config is not user-ready Kiro. **Resolution:** cover owned User/Project Skills/MCP initial add, exact repeat, safe replacement, repair, remove, idempotency, foreign entries and changed-file rejection; inspect the real native no-auth boundary without presenting a login refusal as callable tool success. Keep working across Linux, macOS and Windows. Require final-HEAD CI, review threads resolved and postmerge evidence before reporting complete.

## Implementation work
1. Inspect the actual existing Kiro schema and tests, and whether a trustworthy native list of tool names is available. Prefer no production change if configuration already satisfies an approved exact-list case.
2. Add a Kiro-only, least-privilege, confirmed-install path for selected MCP tool approvals if and only if there is a stable native authorization representation. If not available, keep an honest explicit limitation rather than synthesizing trust.
3. Add focused regressions and exact-source E2E evidence; review twice for boundaries and security, then once for lifecycle/results.
4. Draft PR first; publish each commit immediately. Mark ready and merge only when the actual native and mandatory CI gates pass; no unrelated PRs, release or generated changes.

## Estimates (before implementation)
Production: 60–180 LoC. Tests: 150–350 LoC. Complexity 5/10; reliability target 9/10; confidence 7/10 before real native acceptance. Scope excludes changes to vendor Kiro binaries, user's credentials, and implicit trust of entire workspaces.
