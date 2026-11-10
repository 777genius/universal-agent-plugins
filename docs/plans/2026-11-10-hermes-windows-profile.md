# Hermes native default profile correction

Date: 2026-11-10. Narrow source fix found while qualifying the three-platform
Hermes lifecycle; no migrations, receipt rewrites or other client changes.

## Confirmed source contract

At Hermes 0.23 commit `2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19`,
[`get_process_hermes_home` and `_platform_default_hermes_home`](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/hermes_constants.py)
select explicit `HERMES_HOME` first. Without it, Windows uses
`LOCALAPPDATA/hermes`, falling back to `HomeDir/AppData/Local/hermes` when
LOCALAPPDATA is empty; POSIX uses `HomeDir/.hermes`. The literal
`HERMES_DATA_DIR_SUFFIX` is appended to the default directory name. There is no
filesystem-dependent fallback to an existing Windows `~/.hermes` directory.

The [PowerShell installer](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/scripts/install.ps1)
sets HERMES_HOME in its process and persists only the launcher PATH. The
[launcher](https://github.com/NousResearch/hermes-agent/blob/2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19/hermes_cli/_launchers.py)
pins HERMES_HOME inside its own runtime. Neither supplies HERMES_HOME to an
independent UAP process started in a later shell. Bootstrap repair recovers a
torn source update; it does not change the platform default profile rule.

## Plan before implementation

1. Select these defaults inside the existing Hermes adapter only; preserve
   explicit HERMES_HOME precedence, strict path validation and canonicalization.
   Treat the default directory name plus suffix as one portable path segment.
2. Capture HERMES_HOME and HERMES_DATA_DIR_SUFFIX in the existing detector's
   environment whitelist. No client switch, new interface or ambient host read.
3. Test injected Linux/macOS/Windows roots, missing LOCALAPPDATA, literal suffix,
   explicit override, rejected ambiguous input and an untouched legacy folder.
   Add a NewOS composition regression for the previously dropped environment.
4. Regenerate the Windows detection golden with the real Go test harness;
   require the candidate's normal core/native/Required CI before merge.

Self-review: scope 9/10, source confidence 10/10, complexity 3/10. Local Go
compilation is unavailable; source validation is not a substitute for CI PASS.
