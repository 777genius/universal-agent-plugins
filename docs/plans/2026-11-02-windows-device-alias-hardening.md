# Windows device alias portability hardening (2026-11-02)

## Confirmed defect

Win32 reserves `COM¹/²/³` and `LPT¹/²/³` (including those followed by an extension) in addition to ASCII COM1-9/LPT1-9. Microsoft Learn: https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file. UAP `pathpolicy.ValidatePortablePathSegment` currently checks ASCII names but misses these six aliases. The bug is cross-platform: Linux/macOS can snapshot or unpack a package that Windows cannot reproduce safely.

## Minimal solution

- Extend the existing *one* reserved name map; no new Windows-only parser or public contract.
- Prove case-insensitive rejection in the pure path-policy API, ZIP/TAR archive extraction (including nested directory paths), and source-spec normalized package subpaths.
- Preserve valid Unicode path segments (including non-ASCII names that aren't device aliases). Strictly no broad Unicode/ASCII ban.
- Detect failure before extraction output and cleanup must preserve the disposable root.
- Run Go tests and actual package/source E2E on Linux, Windows, macOS; include Windows native assertions. Do not claim green CI until final SHA passes.

## Critique and merge checklist

Security: escaped aliases cannot bypass validation via extension/casing; no temp write leaks.
Portability: Unicode NFC preserved, no loss of valid skills.
Architecture/DRY: one shared map used by `ValidatePortablePathSegment`, snapshots, archives and source grammar; existing clean dependency direction.
- [ ] New negative and positive tests
- [ ] True four-platform CI and review comments
- [ ] Merge only after E2E evidence, no VS Code adapters changed
