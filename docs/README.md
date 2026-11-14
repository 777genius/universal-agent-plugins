# Maintainer documentation

This directory contains tracked design contracts, operational runbooks and
version-bound evidence. Public guides are published from `website/source/` at
[universal-agent-plugins.com/docs](https://universal-agent-plugins.com/docs/en/).
Generated API references live in `website/generated/`; `website/.site/` is the
assembled build input. Do not point the public site at this maintainer tree.

## Start with the current product

`agentplugins` is the public CLI, distributed on npm as
`universal-agent-plugins`. The last verified public installer baseline for this
documentation update is [0.1.94](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.94).
Main can contain features that are absent from that release; a merged change,
source-level test or successful installer release does not by itself qualify
an authoring release or every native client runtime.

| Need | Reference |
| --- | --- |
| Install the CLI | [Native installation](NATIVE_INSTALL.md) |
| Understand support and client activation | [Support boundary](SUPPORT.md#universal-agent-plugins-installer), [public compatibility](https://universal-agent-plugins.com/docs/en/reference/client-compatibility.html) |
| Restore a dependency environment | [Dependency contract](UAP_DEPENDENCIES.md) |
| Prepare one project | [Project installation](PROJECT_INSTALLATION.md), [client support](PROJECT_CLIENT_SUPPORT.md) |
| Install standalone Skills | [Skills installer](AGENT_SKILLS_INSTALLER.md) |
| Understand direct package sources | [Source formats and pins](DIRECT_PACKAGE_SOURCES.md) |
| Understand ownership and recovery | [Removal safety](REMOVAL_SAFETY.md), [installer principle](INSTALLER_AUTOMATION_PRINCIPLE.md) |
| Embed installation | [Go installer](../install/integrationctl/agentplugins/installer/README.md), [TypeScript Management SDK](../sdks/typescript/README.md) |
| Change or release the CLI | [Architecture](ARCHITECTURE.md), [release runbook](agentplugins-release.md), [release checklist](RELEASE_CHECKLIST.md) |
| Reproduce version-bound native evidence | [Released installer proof](released-native-client-proof.md), [project observations](evidence/PROJECT_INSTALLATION.md) |

Check availability labels before using source examples. Automatic project
selection requires CLI 0.1.94; portable commands are present in the verified
0.1.93 baseline with target-specific limits. Current-source authoring commands
and the TypeScript Management SDK retain separate qualification boundaries.
Read each contract rather than treating every checked-in example as part of
the version installed from npm.

## Historical material

The preserved `plugin-kit-ai` YAML CLI, its release ledgers, plans and runtime
examples document earlier products and implementation decisions. They are not
a second current CLI or an installation path for new users. In particular,
[STATUS.md](STATUS.md) records the historical v1 delivery ledger; older
`public-stable` labels describe that recorded API contract.

Keep historical evidence identities and owner decisions intact. The
[capability preservation contract](AUTHORING_CAPABILITY_PRESERVATION.md)
requires useful implementation, tests and design material to remain available;
archival status is not permission to delete them. Add current guidance at the
appropriate boundary instead of rewriting a historical result as a new pass.
Local scratch notes belong outside this tracked tree.
