# VS Code workspace research: compatible hosted sandbox environment

2026-10-25. This amendment changes the standard hosted image for the existing
workspace observation workflow. It does not change the publisher, native argv,
process primitive, privilege policy, executable modes, trust or observation gates.

## Observed blocker and source-supported hypothesis

The genuine Ubuntu 24.04 run
[40182639794](https://github.com/777genius/universal-agent-plugins/actions/runs/40182639794)
stopped before producing workspace metadata. Its bounded artifact
`12596073531` is 5,185 bytes with SHA-256
`21bad1e1d409fe33476cc2b9ec96d64aaafc38c0609d7262aec8617eb897d977`.
The native fatal diagnostic requests a root-owned mode-4755 `chrome-sandbox`.
The recorded same-process prelude proves original UID/GID 1001, no groups, zero
capabilities, NNP and a separate loopback-only network namespace. Observation is
null and cleanup is not proved; the failed stage remains failed.

The exact VS Code publisher source
[`e6468682` .npmrc](https://github.com/microsoft/vscode/blob/e646868227c3e3a9e3f24277c8c7220be42c0be7/.npmrc)
selects Electron 43.7.7. That release resolves to
[`ddb762f5` DEPS](https://github.com/electron/electron/blob/ddb762f50fbd70384d0385302697e60ada549024/DEPS),
which selects Chromium 150.0.7871.250, resolved to `f916f1b7`.
Its [zygote selection](https://github.com/chromium/chromium/blob/f916f1b7136907af19d96b95b891533d091d932a/content/browser/zygote_host/zygote_host_impl_linux.cc)
prefers the native user-namespace sandbox when
`CanCreateProcessInNewUserNS()` succeeds. Only the fallback selects the setuid
helper. The helper's
[PrependWrapper check](https://github.com/chromium/chromium/blob/f916f1b7136907af19d96b95b891533d091d932a/sandbox/linux/suid/client/setuid_sandbox_host.cc)
contains the observed fatal and requires new privileges when launching it.
It is incompatible with this collector's NNP/ordinary-executable contract.

Ubuntu's [24.04 release notes](https://documentation.ubuntu.com/release-notes/24.04/)
and [security feature table](https://documentation.ubuntu.com/security/security-features/security-features-tables/)
document default AppArmor restrictions on unprivileged user namespaces in 24.04
and their absence in 22.04. This supports a host-compatibility hypothesis; the
artifact does not itself identify the rejected syscall or AppArmor rule.
Do not claim AppArmor was proved to be the cause or that another host will pass.

## Exact change and preserved admission

After two independent plan-review rounds, change only `runs-on` in
`.github/workflows/vscode-workspace-research.yml` from `ubuntu-24.04` to the
explicit standard `ubuntu-22.04` hosted image. Keep the exact client archive,
Go version, workflow permissions, inert contracts, deadlines, upload admission
and every existing runtime guard. Record the actual runner image/version from
the eventual job; a label alone is not an execution result.

The inspected official
[image inventory at `2deeef00`](https://github.com/actions/runner-images/blob/2deeef005a7e79479c137ef48d6f07f2cca37880/images/ubuntu/Ubuntu2204-Readme.md)
lists image 20261023.322.3, kernel 6.8.0-1064-azure, GTK and Xvfb. The workflow
still requires actual installed root-owned Xvfb/xauth, the exact Python/Go
contracts and successful strict cgroup/CLONE_INTO_CGROUP admission. A missing
dependency or unsupported kernel/host condition remains a failed observation.
The image's announced deprecation makes this a bounded research host choice,
not a general support promise or a product platform change.

Do not install packages, alter kernel/AppArmor/sysctl policy, enable setuid,
delete the publisher helper, pass sandbox-disabling flags, spoof hosted guards
or fall back to a different archive/process runner. The standard Electron
sandbox must initialize under the unchanged native invocation. Do not weaken
the existing untrusted-workspace, original-HOME or no-MCP-startup conditions.

## Verification and outcome

Review the one-line workflow delta and retained guard/argv source. No new test
that merely mirrors the runner label is needed. Run the meaningful existing
collector/observer inert contracts on the assembled source, then obtain one
genuine hosted artifact with exact source/client/image identities. Independently
interpret the complete A/B, restart, removal and cleanup evidence before
allowing the separately reviewed project-adapter implementation to proceed.
Neither this amendment nor a host launch alone qualifies project support.

The independent correction for cleanup of read-only Go module-cache directories
is a separate slice. Integrate both reviewed fixes for the next observation;
preserve this first failure and its raw evidence unchanged.
