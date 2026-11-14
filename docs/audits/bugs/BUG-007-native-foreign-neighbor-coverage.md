# BUG-007 — Foreign native-profile preservation was only checked for Codex

## Evidence (2026-11-04 source inspection)

In `scripts/live-install-audit.py`, every Skill case creates a foreign project Skill next to the managed Skill, but user-scope plugin cases only create `native_foreign` when `target == "codex"`. For the other available native client adapters, the safety oracle checks `home/foreign-untouched.txt` outside the client's own profile, so it cannot detect deletion of an unrelated file *inside* that native profile during add/remove. This is a missing E2E safety assertion, not an observed data-loss incident or known production installer bug.

## Minimal plan

1. For every target with a nonempty native-profile root list, place the same harmless `foreign-neighbor.txt` sentinel under the first disposable client profile before taking the baseline. Keep ChatGPT's manual/remote target without a fabricated profile.
2. Preserve the existing literal byte-for-byte post-removal check. Do not create a fake client executable, copy a real user profile, relax client detection, modify production adapters or hide native failures.
3. Verify the change through the full client matrix. If the sentinel affects a vendor's actual profile initialization, diagnose the specific client and use an equally foreign but valid marker rather than weakening cleanup assertions.

## Plan critique

- **Data-loss critic:** A sentinel in the user's home but outside `.cursor`/`.claude`/`.config/opencode` cannot catch an adapter deleting a whole client-native folder. An in-profile neighbor is the minimum useful destructive-removal canary.
- **Compatibility critic:** The canary must remain under a disposable HOME after environment rebinding, must not masquerade as a native plugin and cannot substitute for a real client binary. This is coverage hardening, not automatic proof of activation.

**State:** native profile sentinel expanded to every local adapter and isolated no-network regression pushed in PR #698; current-head client E2E pending.
