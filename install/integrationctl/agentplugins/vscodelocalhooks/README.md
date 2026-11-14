# VS Code Local fixed hook commands

This public package is a pure installer-library boundary for fixed `Stop` and
`SubagentStop` entries. It consumes trusted absolute executable paths and
already projected argv. It has no file IO, environment reads, configuration
mutation, consent, profile/client logic, payload interpolation or stdout event
response. It does not establish task success or native Notifications availability.

```go
import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodelocalhooks"

target := vscodelocalhooks.Target{Shell: vscodelocalhooks.LinuxSH}
specs := []vscodelocalhooks.Spec{{
    Event:          vscodelocalhooks.Stop,
    Executable:     "/trusted/TEST runtime/notifier",
    Args:           []string{"local-stop", "--locator", "/trusted/TEST data/locator"},
    TimeoutSeconds: vscodelocalhooks.TimeoutSeconds,
}}
body, err := vscodelocalhooks.Render(target, specs)
// After checking err, the existing projector writes body to PluginPath.
// It first binds the source file/package digest to these declared specs.
err = vscodelocalhooks.VerifyOwned(body, target, specs)
```

`RenderArgv` exposes the same command rendering independently. `Render` returns
a fresh complete strict JSON file. Linux entries have only `type:"command"`,
`linux` and `timeout:5`; macOS substitutes `osx`, Windows preparation `windows`. Platform-only
commands avoid falling back to another OS's shell syntax. Event names are exact
PascalCase. There is no numeric version, Claude matcher group, CLI camelCase,
default command, explicit cwd or env. Selecting one event never adds the other;
the Notifications consumer must separately suppress subagent observations.

`VerifyOwned` compares the entire fixed authored artifact, tolerating JSON
whitespace/key ordering/equivalent string escapes and numeric representations
of five seconds. Shell command spelling remains exact. Duplicate keys, missing
or extra events/entries/fields, alternate platform commands, drift and malformed
Unicode fail. This verifies expected bytes semantically; it grants no ownership
and provides no adoption/removal operation. A foreign native file is refused
and left for its owner. The later projector must check canonical package/file
identity and maintain separate source/projection digests before writing.

Limits are two distinct events, 64 arguments per invocation, 2,048 UTF-8 bytes
per literal, 16,384 bytes per rendered command and 65,536 bytes per encoded JSON
file. Verification bounds nesting to eight before the existing hujson parser
allocates its AST. NUL, CR/LF and invalid UTF-8 are refused in executable paths
and arguments; Unix empty strings and remaining non-NUL controls stay fixed
literals. Errors classify via `errors.Is` and do not echo supplied command values.

Unresolved `${PLUGIN_ROOT}`, `${PLUGIN_DATA}` and `${CLAUDE_PLUGIN_ROOT}` are
explicitly refused even inside quoted arguments by this library's fixed-projection
contract. The standard AgentPlugin namespace parses hooks without legacy
plugin-root interpolation. The existing UAP projector must resolve declared
references once against trusted active/data roots before rendering. No native
root expansion has been admitted by this slice. Other dollar expressions,
backticks and shell operators remain literal Unix argv, as proved with a real
`/bin/sh` recorder.

The source contract is pinned to VS Code **1.140.0** / bundled Copilot **0.68.0**, commit
[`07f806f999227108933c2e30515b26eecc1fda74`](https://github.com/microsoft/vscode/tree/07f806f999227108933c2e30515b26eecc1fda74).
The Linux official archive supplied to the earlier native spike has SHA-256
`d32031e9e213d59532af3cf32fcb8b357a1cdd10417967b4f5b5ba30436dc0dc`.
Relevant public sources at that commit:

- [`hookExecutor.ts`](https://github.com/microsoft/vscode/blob/07f806f999227108933c2e30515b26eecc1fda74/extensions/copilot/src/platform/chat/node/hookExecutor.ts):
  `getShellCommand` and `_spawn`; Unix `shell:true` selects Node's `/bin/sh`,
  inherited environment, default `homedir()`, seconds-to-milliseconds timer.
- [`hookSchema.ts`](https://github.com/microsoft/vscode/blob/07f806f999227108933c2e30515b26eecc1fda74/src/vs/workbench/contrib/chat/common/promptSyntax/hookSchema.ts):
  `normalizeHookCommand`, `resolveEffectiveCommand`, `resolveHookCommand` and
  flat extraction. Native accepts broader forms than this authored contract.
- [`hookTypes.ts`](https://github.com/microsoft/vscode/blob/07f806f999227108933c2e30515b26eecc1fda74/src/vs/workbench/contrib/chat/common/promptSyntax/hookTypes.ts):
  Local PascalCase versus separate CLI naming.
- [`pluginParsers.ts`](https://github.com/microsoft/vscode/blob/07f806f999227108933c2e30515b26eecc1fda74/src/vs/platform/agentPlugins/common/pluginParsers.ts):
  Standard AgentPlugin parsing passes hooks directly to `parseHooksJson`;
  legacy Claude/OpenPlugin root interpolation is a separate contract.

The supplied prior native Linux spike genuinely selected a public synthetic
language-model provider after anonymous setup, completed one Local response,
observed `Stop`, discovered `com.github.copilot/hooks/hooks.json`, and recorded
adversarial argv plus default cwd in a fresh TEST HOME. Its archived receipt,
source attestations and limits are referenced in `.research/implementation-evidence.md`.
This implementation reuses that evidence; it launches no native application.
Its Linux/macOS process test invokes the actual `/bin/sh -c` boundary with a
copied inert test executable, fresh TEST HOME/USERPROFILE, hostile executable/path/literal
values and an injection sentinel. It emits no synthetic Stop input and claims
no new native lifecycle result. The supplied coordinator Mac receipt proves a
genuine native Stop ran the exact 8955bd6 POSIX command after changing only `linux` to `osx`; it does not prove
execution through the new public `MacOSSH` API. The coordinator will rerun that
API after source review. Complete installed candidate qualification remains a
later gate.

Windows is **prepared only**. The trusted snapshot must give an ordinary
drive-absolute `SystemRoot` and its exact `System32\cmd.exe` `ComSpec` (case
insensitive). The pinned native executor then calls
`SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe` with
`-ExecutionPolicy Bypass -NoProfile -NoLogo -Command <rendered command>`,
`POWERSHELL_UPDATECHECK=Off`, and no intermediate cmd shell. This is Windows
PowerShell 5.1, not pwsh 7. Commands use `&` and single-quoted literals.
Missing/non-system ComSpec, fallback shells, scripts/UNC/device/relative paths,
empty arguments, double quotes, trailing backslashes and controls are refused;
smart single quotes (U+2018/U+2019/U+201A/U+201B) are refused in both paths and arguments.
Windows executable paths must end in `.exe`. Native Windows CI must qualify
actual argv representation before activation; Linux string tests and a Windows
cross-build are solely preparation/compilation evidence.

`MacOSSH` is an explicit trusted target with the identical POSIX renderer,
bounds and placeholder refusals as `LinuxSH`. Both require empty `ComSpec` and
`SystemRoot`; unsupported targets refuse without fallback. Only native `osx`
is emitted. The shared process contract runs on actual Darwin `/bin/sh` in the
focused workflow; Linux execution does not qualify Darwin. Supplied signed Mac
VS Code receipts are attributed coordinator evidence, with both failed first
attempts retained; artifact hashes and exact field projection were inspected.
They establish native command transport, not this new API's native execution,
installed Notifications delivery, a desktop banner or timeout/tree cleanup.

`TestTESTWindowsPowerShell51Process` runs only on Windows, through public
`Render`/`VerifyOwned` and the exact system PowerShell executable/vendor argv.
It probes that executable's actual `$PSVersionTable` independently, requiring
5.1/Desktop and the exact system process path. The unchanged rendered command
then launches a copied inert native `.exe` receiver with hostile path/Unicode,
space, apostrophe, dollar, backtick, percent and ampersand literals. The receiver
writes actual argv/cwd/TEST HOME/USERPROFILE in an exclusive UTF-8 file, so
PowerShell's legacy stdout transcoding cannot conceal argument corruption.
Raw stdout/stderr bytes, exit code, version, encoding code pages, rendered JSON
and receiver SHA-256 are logged. It uses fresh TEST profiles, injection sentinels,
a bounded process deadline and cleanup; refused forms produce no artifact or
receiver effects. The version probe is a separate invocation of the same exact
system executable; no test instrumentation modifies the rendered command.

`.github/workflows/vscode-local-native.yml` runs these contracts on actual
`windows-latest` and `macos-latest`, with exact-head logs and an explicit
`U2_REQUIRE_NATIVE` OS gate. Missing PS5.1 fails. A missing/skipped required
process test also fails the workflow. There is no GUI/model/auth dependency.
A Windows CI pass qualifies only the exact PS5.1 argv transport; genuine Windows
Local Stop, installed delivery and desktop visibility remain unproved. Until
that CI result exists Windows remains prepared. Local Linux passes, compilation
and supplied Mac command receipts must never be substituted for native CI.

Five seconds is a native timer value, not a hard tree cleanup guarantee. The
vendor executor signals its spawned shell/process with SIGTERM, escalates that
child to SIGKILL after another five seconds and resolves on stream close. It
does not establish process-group/descendant termination; inherited descendant
pipes may delay close. This renderer adds no process manager and cannot repair
that vendor limitation. Installed artifact timeout/descendant qualification is
pending; native support must retain the limitation or refuse such activation.

Public tests cover the observable artifact, refusal and actual Unix argv
boundary. Run with a fresh TEST `TMPDIR`/HOME and isolated Go caches, as recorded
in the implementation evidence. No native app/model/provider test runs here.
