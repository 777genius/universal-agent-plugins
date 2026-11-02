# Native Git transport hardening parity (2026-11-02)

## Why

Microsoft APM's historical issue #2368 (https://github.com/microsoft/apm/issues/2368) demonstrates that auth environment merges can erase Git hardening (`safe.bareRepository=explicit`, noninteractive credentials). UAP avoids the exact APM bug by clearing arbitrary inherited `GIT_CONFIG_*` and placing explicit `-c` arguments in the native Git transport. However, its current `nativeGitOptions` does not enforce either of those two protective Git settings. The UAP invariant should be stronger regardless of a user's global Git config.

## Scoped implementation

- Add `safe.bareRepository=explicit` and `credential.interactive=never` to **existing** `nativeGitOptions`; do not alter native credential helper selection, SSH wrappers, TLS certificate choices or project transport protocol list.
- Add real `git config` subprocess tests verifying command overrides even when inherited `GIT_CONFIG_COUNT` attempts to inject unsafe config; verify command execution does not touch the user's source files.
- Preserve real HTTPS/SSH auth E2E and Git sealed acquisition, Windows native transport. Never replace with fake Git scripts.
- No new policy framework, feature flag, saved auth fields, extra env injection, VS Code changes or accepted ADR modifications.

## Review and acceptance

Security: bound intent to one Git subprocess; configuration cannot be lost by merged env; legacy users retain own authentication inputs.
Portability: target bundled/installed Git executable recognizes these config keys or safely ignores unknown keys.
SOLID/DRY: one native Git options producer; no per-source duplicated toggle.
- [ ] Tests + production commit
- [ ] Real native Git transports and source-acquisition E2E, Core fast/CodeQL
- [ ] Independent security/lifecycle/architecture critique
- [ ] Merge only once final SHA CI is proven successful
