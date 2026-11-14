# Archive members: one conventional leading dot prefix

Owner authorization and both independent plan reviews completed on 2026-10-23.
This narrowly amends member-name handling in `sources-plan.md`; it does not
relax URL grammar or requested logical package paths.

Ordinary `tar -czf package.tar.gz -C package .` output uses a root `./` directory
and members such as `./plugin.json` and `./skills/demo/SKILL.md`. ZIP, TAR and
effective PAX names may therefore have exactly one leading `./` removed before
the existing portable-path checks and collision map. The root marker is ignored
only for a directory and still counts toward archive limits.

No unsafe input is cleaned into acceptance. After stripping one prefix,
residual dot/traversal or empty segments, absolute/drive/UNC/backslash/control
paths, links, special entries and nonportable names still fail. All explicit
names and implicit ancestor keys use the same effective name. Consequently
`./x` plus `x` is a duplicate, and `./X/a` plus `x/b` is a case collision.

R1 by `project_core` accepted exactly this normalization and required shared
collision handling. R2 by `sources_lead` independently returned GO; the full
parent-workspace ledger is `research/sources-dot-prefix-review-r2.md`. The
parent recorded both GO decisions before implementation.

`archive_compat_test.go` verifies real TAR/gzip-TAR/ZIP dot-prefixed packages,
logical root selection and executable inventory; aliases collide across both
formats. Regressions reject `././x`, `./../x`, `.//x`, `./C:/x`, `./` regular
files, repeated separators and prefixed links. Byte pins, wrapper rules and
strict archive endings remain unchanged. Preserved legacy bundle code is
unmodified.
