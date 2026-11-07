# ChatGPT target presentation split (2026-11-07)

## User-facing contract

- **Supported agents / client landing:** one ChatGPT entry, explicitly covering Desktop and Web; Codex remains a separate agent. The support table has one ChatGPT row.
- **Installer picker:** show **ChatGPT Desktop** and **ChatGPT Web** separately, as well as Codex, with precise eligibility and setup messages.
- **Public CLI targets:** `--target codex` selects Codex CLI; `--target chatgpt` selects local ChatGPT Desktop (shared Codex marketplace); `--target chatgptweb` selects the remote ChatGPT app-binding/preparation path. The old public `chatgpt=Web` meaning is intentionally changed before general adoption. Internal signed Directory and durable-state identities stay `codex` and `chatgpt` (Web). Normalize public `--target` once at the flag boundary, never while rereading canonical state.
- **Shared backend:** selecting Desktop and Codex must not run the same install twice; the picker chooses only one alias for that shared native installation. Never promise distinct uninstall ownership across the same Codex profile. A non-default `CODEX_HOME` may not match Desktop.
- **Security:** signed eligibility remains based on canonical `codex` or `chatgpt`, including ChatGPT web app-binding requirements. Desktop is user scope only in this UX; do not silently turn a project request into a user install. No cloud ChatGPT connection is made by a local CLI call.
- **Release boundary:** do not publish a new CLI release without owner authorization. Do not deploy a public website command requiring unreleased alias support; the combined PR remains draft until qualified and released or guarded.

## Minimal implementation

1. Adapt public CLI flag spelling and target tests (no new adapter, store, schema or safety bypass).
2. Create a picker-only target mapping layer. Resolve/install policies through canonical IDs but generate explicit surface target commands. Preserve the signed Directory's exact client identities and the internal signed `chatgpt` identity.
3. Replace the separate always-visible Desktop command block with a contextual explanation when Desktop is actually selected. Keep one ChatGPT tile/support row, disclose both Web and Desktop behavior.
4. Update the nine maintained locale texts together; add regression tests for command generation, release eligibility, scope refusal, duplicate backend prevention, copied commands and light/dark layouts.

## Explicit E2E and review gates

- Go parser/unit contract; read-only exact Directory policy, CLI native Codex lifecycle with a real supported binary on Linux/macOS/Windows.
- Site registry, i18n parity, browser picker + clipboard and build at both Pages paths.
- CLI alias release must precede deploying site command; downloaded public CLI must be checked, not inferred from HEAD.
- Real signed-in ChatGPT Desktop activation and Web marketplace connection are separate user/account qualifications, **not** certified by native CLI or fixtures.
- Two review rounds: architecture/security (no bypass or backend duplication), UX/E2E (correct names, capabilities, refusal and legacy commands). Merge only on exact-head green checks and release compatibility.

## Scope exclusions

No new native ChatGPT adapter, account OAuth automation, browser clicking, custom file injection, registry signing changes, VS Code edits, or feature flag.
