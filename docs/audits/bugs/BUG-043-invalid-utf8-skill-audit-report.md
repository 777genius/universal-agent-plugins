# BUG-043 — non-UTF-8 SKILL.md aborts per-case E2E reporting

## Reproduction

Independent [CodeRabbit inline review](https://github.com/777genius/universal-agent-plugins/pull/698#discussion_r4470160705) (2026-11-04) identified `skill_file_present`: it calls `f.read_text(encoding="utf-8")` but catches only `OSError`. A source with invalid UTF-8 in `SKILL.md` raises `UnicodeDecodeError`, skipping the ordinary add/list/repeat/remove evidence and reducing that case to `harness-error`. The testing harness must record exact original byte proof independently and avoid crashing on invalid text.

## Minimal plan

1. Decode only the diagnostic frontmatter scan with `errors="replace"`; do not modify source bytes, actual normalized package bytes, Git blob SHA-1 verification or the installer.
2. Preserve existing path/ownership check and exception on unreadable file (`OSError`). A corrupt filename must not be silently treated as an installed/cleaned owned Skill; source pin and whole Skill tree/removal oracles remain authoritative.
3. Add a disposable test with malformed UTF-8 preceding a legitimate `name:` line and assert the existing frontmatter scan finds the Skill, while a foreign name remains unmatched. Run Python E2E guard and final-head Actions.

## Critical review round 1 — correctness

Replacing undecodable bytes **only for the bounded read-only name probe** cannot change the installed raw bytes, signed source, Git blob hash or `SKILL.md` files. Keep the distinct raw-byte integrity oracle unchanged.

## Critical review round 2 — robustness

Catching `UnicodeDecodeError` and returning `None` would hide leftover payloads and risk false PASS. Replacement decoding lets the name matcher inspect valid spans, while the independent full Skill-tree and pin oracles still refuse malformed or relocated content. Do not add new encoding heuristics, parsers or a second acquisition mechanism.

## Status

Review finding confirmed in source. Focused test/code implementation and current-head full E2E pending; no merge approval.
