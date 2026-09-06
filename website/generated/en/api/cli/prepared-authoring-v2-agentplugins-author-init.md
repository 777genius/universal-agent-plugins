---
title: "agentplugins author init"
description: "Create an offline standard package in an absent destination"
canonicalId: "prepared-authoring-v2:agentplugins author init"
section: "api"
surface: "authoring-cli"
locale: "en"
generated: true
editLink: false
translationRequired: false
namespace: "prepared-authoring-v2"
status: "prepared-not-release"
released: false
sourceSHA: "801f846cac40f47c344983ccb61122cbdd3ae2f8"
factoryBaselineSHA: "070663efb27f69ecae8609e6b839f86f843efbb0"
sources:
  - "cli/plugin-kit-ai/cmd/agentplugins/release_root.go: c0465f90903c7ad3fcdc2283c241558d1b73bd9633f0e6af247ccd692a0e155e"
  - "cli/plugin-kit-ai/cmd/plugin-kit-ai/release_compat.go: d2f11a845c116160114ddd1b0ba8b825e9e5a4794354742749db7122c0c674c8"
  - "cli/plugin-kit-ai/go.mod: c19d163f4cab4b7a3cdd6c71bec885809c81beaf4aee705071b90780bc9a15eb"
  - "cli/plugin-kit-ai/go.sum: 8d0b8892b2f6528b9bfecf83d8efda5533ae696d2506c061df9bf4d88ede3eb0"
  - "cli/plugin-kit-ai/internal/agentpluginscli/add.go: 9a118d98465055c7793ab2fd31c5693757406e146164a3df9a64a46a410e42ba"
  - "cli/plugin-kit-ai/internal/agentpluginscli/add_multi.go: f09cd6557f77f79641959f425b8065457972f3b1f35e1461620de0ff3e4cce0e"
  - "cli/plugin-kit-ai/internal/agentpluginscli/app.go: 763c3a847ffe3100eae78bb9d469948c228fd6eb48548a1752025ae35af91777"
  - "cli/plugin-kit-ai/internal/agentpluginscli/binding.go: 7f8d411ecdd59db01519c8a820ff64455f276ac575e29e7534cd4e716dc5bd52"
  - "cli/plugin-kit-ai/internal/agentpluginscli/engine_boundary.go: 68f3a74da353d3a12d506f948f542e1501ba05bbfc6d7b23b6c5130631518b52"
  - "cli/plugin-kit-ai/internal/agentpluginscli/interactive_targets.go: ed172b27715c8c9bf69b01c37da47e84f4cb2df606f6bf701630987d5e27a705"
  - "cli/plugin-kit-ai/internal/agentpluginscli/legacy_controller.go: 4ba1296edf3c0cdc6e8caf0b27b577ad8715231f4201d4e8e2f56679f1713326"
  - "cli/plugin-kit-ai/internal/agentpluginscli/lifecycle.go: 083f12310c10c7f784d04f607b0948662cc3a92c32c852454114985d9c8b5b16"
  - "cli/plugin-kit-ai/internal/agentpluginscli/outdated.go: 3ecf284023dfb158e6c1c60572a57da99062ff1c68a1ecf105db677e7cb1f723"
  - "cli/plugin-kit-ai/internal/agentpluginscli/output.go: 571e944a73ac3cc89e8b66b84fb0274943913dd4ca2a28f953c5563d400ce8dc"
  - "cli/plugin-kit-ai/internal/agentpluginscli/preflight.go: eb71e71febc417494f98445dcef15024878a034bc690f51ccaba31b9b89b7875"
  - "cli/plugin-kit-ai/internal/agentpluginscli/read.go: 02be006371e139e34f580f3340d23a70b7c6e85133f4bebfd6327720e47af6e1"
  - "cli/plugin-kit-ai/internal/agentpluginscli/read_directory.go: a2ba8aa03cd1e8efc2a8b3b9aa8718aceb13822bb19d083d6fa32a76e316b9d1"
  - "cli/plugin-kit-ai/internal/agentpluginscli/read_reconciliation.go: f04a6bcccb75beb83d93002dff694e9af240a8b29410d4bbafeaf52ab2edb102"
  - "cli/plugin-kit-ai/internal/agentpluginscli/remove_multi.go: 52dbe20bd0b3ce5a7b3df188f41cb863af3024667eda456410a1bffa48b983a2"
  - "cli/plugin-kit-ai/internal/agentpluginscli/repair_multi.go: f0bc6745c9ec4b94cb066d28f705ebf421f8a524b71f40b70857617ecaf4d004"
  - "cli/plugin-kit-ai/internal/agentpluginscli/root.go: 97dac1600168a64835cc9aeeb998f56d5796e566572d9530f3f6537f8ad5ae04"
  - "cli/plugin-kit-ai/internal/agentpluginscli/search.go: d954c3b68d6bcfe6b821855aed6c14e712b9ff0eff0bf67206a1191f714fa998"
  - "cli/plugin-kit-ai/internal/agentpluginscli/security.go: ef8126625f12bdcf1aeedefdc11d243ea700c0b6050e44c9f566061426d06894"
  - "cli/plugin-kit-ai/internal/agentpluginscli/source.go: fc9dca352dae1ea9bbe963195c4d6e2ab6d378002649faf84f5474044fb892b5"
  - "cli/plugin-kit-ai/internal/agentpluginscli/state_migration.go: be360a4d68b99b40c983f4d01e64f4e81f2c29e2659400b58bd90a0b2f8bd28d"
  - "cli/plugin-kit-ai/internal/agentpluginscli/target_batch.go: afa9dc9ba6a417229c57afda8a1eacbc397b38c0802880e1c14f5f8fc98fc81c"
  - "cli/plugin-kit-ai/internal/agentpluginscli/update_all.go: 31361a5cf6043f6a27c17cb89e0e88a34f11e4e96daf83454d9c38f729d7ba40"
  - "cli/plugin-kit-ai/internal/agentpluginscli/update_multi.go: e80ac96ec11e57fbebe8f0d7ab780e80688060eb94480838be2804ca653463b5"
  - "cli/plugin-kit-ai/internal/agentpluginscli/validate.go: 8c643a08c657431d623197364f5f47d7f23ed0b4e6cf926f40a521a8bac7ee9d"
  - "cli/plugin-kit-ai/internal/authoring/commands/commands.go: 6f581be57435b8f269b80a6c1efca62e8369a1a7ca39dd215cecf15a1e59d568"
  - "cli/plugin-kit-ai/internal/authoring/commands/public_contract.go: 39c79f491f0733d352ffc0fa3a8ff4eaa169612e8876e92e5fb74c856663ec65"
  - "cli/plugin-kit-ai/internal/authoring/commands/skills.go: 566e36e02d58c5d76e92369987d901feb1bcb6f3e59d3225e02961fa041d606b"
  - "cli/plugin-kit-ai/internal/authoring/commands/version.go: ffe6cfef352faeb9a3c00722a628a2093876c14120cd6725131b3e105fda6b29"
  - "cli/plugin-kit-ai/internal/authoringcli/command.go: eaf18b18c8aeaecbc2e63c780a1472097855315a4bcdc22e7eb0386aa1375012"
  - "cli/plugin-kit-ai/internal/authoringcli/flags.go: 5f745c810e90586899cff2170433dbc58483ba17496167738387388f29be49c0"
  - "cli/plugin-kit-ai/internal/authoringcli/release.go: 45cb7a7000a31bb8477d1301fc70ce4521140b9f2a4760d5f84da3283134877f"
  - "go.mod: 352e528c2b2c34df21276c1f9b54c32f79656dad42c310da6b551f022119ce8c"
  - "go.work: b5c80635a9629f9f894201bb9ac80a9e29413f0a90d7a2f761ef4dbe1387623a"
  - "go.work.sum: c5123e0ef46e576f2663bd0d0f86a68b567b29b0d4682d64b2ebaa5f5d7fcefa"
  - "install/integrationctl/agentplugins/domain/acquisition.go: ea60178232db888d8df99a7b6722bb26b7a3f1a23776a6be041cf9c84f7a2cb3"
  - "install/integrationctl/agentplugins/domain/catalog.go: e70853260635bed305ce77fef0c420e2028a57d77a9ce0565ab9cf8b275a98bc"
  - "install/integrationctl/agentplugins/domain/clients.go: 152167e659bb40114f06532769e85bc1b58668465e2c4c6adc011433c0a0fd63"
  - "install/integrationctl/agentplugins/domain/directory.go: d917f80acd770d5749c0b95ce714df9166438aec9c51ca1c16b2d52f23b36168"
  - "install/integrationctl/agentplugins/domain/errors.go: 7229bf792c60bcbb290050a6659f129f09e85e22174d32ac681a4c6e5bb6efd8"
  - "install/integrationctl/agentplugins/domain/identity.go: e4886804b8d35c7b54ce3b87a96e50de69a6248781bd8349a4f59d32512c6c8d"
  - "install/integrationctl/agentplugins/domain/security.go: 6773e2d0fc94bf6cf8a521c1c1dd8a339648bf1634ef5eebebc2917b06ff0abd"
  - "install/integrationctl/agentplugins/domain/state.go: 2db19f678782e8d9644e85ca17517cc54ff395b63a4b1b0bb52c8a0454a1869c"
  - "install/integrationctl/agentplugins/domain/types.go: 30757c8faac4a3b67771169e1acc5aa798c61340e2db50b634614ddbe7b1b9da"
  - "install/integrationctl/go.mod: 17c94b7bcbace5f9e4ee6164e0ff8499b7994923105c9430c84de7964b416116"
  - "install/integrationctl/go.sum: 55d21b3e3f4a7928cf9064f7ccd06cf54b643097cb13c1b776b07a6449b85458"
  - "install/plugininstall/go.mod: 7d0745754d1ae04fb31d5056cbd2f4fa82d711d0536c1c868f40452cf5e6bb53"
  - "sdk/go.mod: 79e8f3d4903364c498fffcf34e9394f84437ae198dcefcd1c0c99cb41c80ba5a"
stability: "prepared-not-release"
maturity: "prepared"
publicVisibility: "preparation"
localeStrategy: "canonical-en"
sourceKind: "authoring-docs-adapter"
sourceRef: "https://github.com/777genius/universal-agent-plugins/tree/801f846cac40f47c344983ccb61122cbdd3ae2f8/cli/plugin-kit-ai/internal/authoring/commands"
---
> Prepared reference; **not released**. [Exact source](https://github.com/777genius/universal-agent-plugins/tree/801f846cac40f47c344983ccb61122cbdd3ae2f8/cli/plugin-kit-ai/internal/authoring/commands).

<!-- namespace: prepared-authoring-v2; status: prepared-not-release; source-sha: 801f846cac40f47c344983ccb61122cbdd3ae2f8 -->

Prepared reference only; not a public release.

# agentplugins author init

Create an offline standard package in an absent destination

### Synopsis

Create an offline standard package in an absent destination

Installer-only inherited flags --scope, --accept-security-risk, and --security-details
are rejected. Use the positional package path and author doctor for project checks;
use agentplugins add for installation/security policy.

```
agentplugins author init <path> [flags]
```

### Options

```
      --author-name string        optional explicit author
      --copyright-holder string   explicit license holder
      --copyright-year string     explicit four-digit license year
      --description string        explicit package description
  -h, --help                      help for init
      --include-root              disclose the explicitly selected root in this report
      --license string            optional MIT or ISC license
      --mcp-template string       hybrid MCP choice: mcp-remote or mcp-stdio
      --name string               exact plugin identity
      --runtime string            stdio template runtime: node
      --skill-name string         explicit Skill identity
      --template string           skill, mcp-remote, mcp-stdio, or hybrid
      --url string                explicit remote MCP URL
```

### Options inherited from parent commands

```
      --accept-security-risk   continue despite blocking automated security findings
      --dry-run                show the exact plan without changes
      --format string          output format: human or json (default "human")
      --no-color               disable color output
      --scope string           installation scope (user only in this release) (default "user")
      --security-details       show every automated security finding in human output
      --target string          target client(s), comma-separated: codex, chatgpt, cursor, copilot, vscode, kiro, claude, gemini, opencode, cline, windsurf
```

### SEE ALSO

* [agentplugins author](/en/api/cli/prepared-authoring-v2-agentplugins-author)	 - Build Agent Plugins packages
