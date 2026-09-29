# Global OpenCode JS plugin placement

`Plan(Input)` is a pure Go helper for one global local `.js` plugin. The caller
passes the intended filename, desired bundle SHA256, trusted owned preimage
SHA256 (if any), and its current observation of the target path. It returns the
selected root, `plugins/<filename>` target, a copied observation, and one of
`create`, `replace`, `unchanged`, or `conflict`. Conflict reasons distinguish a
foreign regular file, an edited owned file, and a non-regular object.

Root precedence is `Override` (the OpenCode config root itself), then
`XDGConfigHome/opencode`, then `HomeDir/.config/opencode`. An unset value is an
empty string. A selected root must be an absolute, clean path; invalid selected
values fail instead of choosing another root. `FileName` must be a single,
visible `.js` basename; separators, control characters, and traversal are
rejected. An observation whose path differs from the computed target is rejected.
Paths use the host Go
platform's `filepath` rules; this package makes no cross-platform loader claim.

The helper performs lexical containment only. It never reads the filesystem,
resolves symlinks, writes or removes files, changes `package.json`, or invokes
the legacy installer. The product transaction must verify the selected root
and `plugins` ancestors against symlink escapes, capture the target with a
no-follow observation, and compare the preimage again at commit time. It must
also own consent, registration, rollback, and recovery. An ownership digest is
meaningful only when read from the product's trusted ownership record; matching
bytes alone do not grant ownership. Changing the selected root requires an
explicit migration/reinstall decision by the product; this plan does not infer
or remove files at an old root.
