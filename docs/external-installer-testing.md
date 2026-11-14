# External installer test protocol

This is a reproducible test invitation template, not a completed test report or
evidence of independent adoption. UAP is a community-maintained installer;
testing does not imply endorsement or certification.

## Choose an exact public release

Use an exact version already published to both GitHub and npm. Record its release
URL, source commit, checksums and verified artifact provenance using the
[release runbook](agentplugins-release.md). Do not use `latest` or an unpublished
candidate. Verified public target: `universal-agent-plugins@0.1.53`,
[binary release agentplugins-v0.1.53](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.53),
source commit `28cf05af0a1e4fea642825dd34b78f9c99094ab5`. Both GitHub and npm
availability were checked on 2026-09-08. The retained 0.1.52 draft is not the
test target. This publication check does not claim the external protocol ran.

The commands below require Bash, Node.js 22+ and npm on macOS or Linux. Use a
disposable VM with a fresh OS account and project, without personal credentials
or mounted user projects. A temporary `HOME` on your regular desktop is not
equivalent isolation. Have the supported Codex client installed in that VM so
the installer can detect its target. The add operation runs real Codex plugin
marketplace registration, plugin installation and inventory commands. No
interactive model session or login is needed. Use Codex CLI `0.153.4`, the
version whose native command contract this protocol follows. The exact
[client archive pins](https://github.com/777genius/universal-agent-plugins/blob/9c33cfac81fc00e61205591321c89c5675fedb60/scripts/run-native-client-matrix.py)
and [Codex lifecycle adapter](https://github.com/777genius/universal-agent-plugins/blob/28cf05af0a1e4fea642825dd34b78f9c99094ab5/install/integrationctl/agentplugins/providers/activator.go)
identify the harness and released implementation separately.

## Skills-only local fixture

The example pins 0.1.53; change it only after verifying another exact release. These commands install into the
VM account's user scope. They do not require Directory registration.

```bash
set -e
UAP_VERSION='0.1.53'
mkdir uap-external-test
cd uap-external-test
mkdir -p fixture/skills/uap-external-check logs
printf 'preserve this unrelated file\n' > unrelated-sentinel.txt
cp unrelated-sentinel.txt unrelated-sentinel.expected
node --version
npm --version
cat > fixture/plugin.json <<'JSON'
{
  "$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
  "name": "uap-external-check",
  "version": "1.0.0",
  "description": "Disposable external installer test",
  "license": "MIT"
}
JSON
cat > fixture/skills/uap-external-check/SKILL.md <<'SKILL'
---
name: uap-external-check
description: Return a fixed marker when explicitly asked to run this disposable test skill.
---
When explicitly invoked, respond with UAP_EXTERNAL_CHECK_OK.
SKILL

run_command() {
  local step="$1" rc=0
  shift
  "$@" > "logs/$step.txt" 2>&1 || rc=$?
  cat "logs/$step.txt"
  printf '%s exit=%s\n' "$step" "$rc" | tee -a logs/exit-codes.txt
  return "$rc"
}
run_step() {
  local step="$1"
  shift
  run_command "$step" npx --yes "universal-agent-plugins@$UAP_VERSION" "$@"
}
run_command client-version codex --version
run_step version version
run_step validate validate ./fixture --format json
run_step add add ./fixture --target codex --format json
```

Check that the version matches, validation reports one skill and no MCP servers
or app bindings, and installation reports its exact state label. Record the
validation tree/manifest digests and generated locations. Inspect only the test
plugin's managed files. Stop on failure or an unavailable target and report the
message; do not retry against your real profile.

Then run:

```bash
run_step repair repair uap-external-check --target codex --format json
run_command native-before-remove codex plugin list --json
```

This repair call checks an intact installation and may be a no-op; it does not
prove recovery from corruption. In the native inventory, locate exactly the
`uap-external-check` entry whose `source.path` matches the managed path recorded
by add. Copy its `pluginId` and `marketplaceName`; verify that the former is
`uap-external-check@` followed by the latter. Stop if the identity is ambiguous,
missing or mismatched. Do not substitute an unrelated plugin or marketplace.

Unregister that exact plugin and marketplace before acknowledging external
uninstallation to UAP. Replace both placeholders from the verified inventory:

```bash
NATIVE_PLUGIN_ID='<verified-pluginId>'
NATIVE_MARKETPLACE='<verified-marketplaceName>'
run_command native-remove codex plugin remove "$NATIVE_PLUGIN_ID" --json
run_command native-marketplace-remove codex plugin marketplace remove "$NATIVE_MARKETPLACE" --json
run_command native-after-remove codex plugin list --json
```

Inspect the new inventory and confirm the exact plugin is no longer installed
or enabled. Only after both unregister commands succeed and that check passes,
run the following acknowledgement. If either command or inspection fails, stop
and report it; do not assert `--external-uninstalled` merely to bypass the gate.

```bash
run_step remove remove uap-external-check --target codex --external-uninstalled --format json
cmp unrelated-sentinel.expected unrelated-sentinel.txt
```

Confirm the test plugin's managed files were removed. The sentinel comparison
checks only unrelated project-file preservation, not all user-profile data.
Report retained plugin data separately from managed client files. Destroy the
VM after collecting redacted evidence.

## Separate runtime evidence

The native inventory can establish registration and enabled state; it does not
prove skill invocation in a model session. Native-client runtime testing has a separate protocol and
maintainer-owned runner in [released native-client proof](released-native-client-proof.md).
Its results are not independent external observations. This baseline does not
test MCP calls, OAuth, model quality, desktop application behavior or Windows.
Use a separately agreed protocol for additional runtime or immutable Git-source
tests; do not infer those results from the local fixture.

## Feedback to return

Include failures and `not tested` results. Redact tokens, credentials, personal
paths and unrelated project content before sharing logs or screenshots.

- Date/time, OS/architecture, Node/npm versions and detected client version.
- Exact UAP version, release URL, source commit and artifact checksum/provenance.
- Isolation used, fixture digests, commands, exit codes and exact state labels.
- Validation, add, repair and remove: pass / fail / not tested, with output.
- Managed paths inspected, unrelated-data preservation and unexpected changes.
- Expected versus actual behavior and reproducible failure steps.
- Relationship to the authors: independent / contributor / paid / other;
  maintainer assistance received; one-off test versus actual ongoing use.
- Permission to publish the report: yes / no / redacted only; separately,
  permission to attribute your optional handle: yes / no.

A report is not permission to publish identifying details. Obtain explicit
consent before quoting it. Record external testing separately from ongoing
adoption, and keep activation, skill execution, MCP and OAuth marked
`not tested` unless separately observed and documented.
