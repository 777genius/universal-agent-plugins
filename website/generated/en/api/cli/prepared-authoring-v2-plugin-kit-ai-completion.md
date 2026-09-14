---
title: "plugin-kit-ai completion"
description: "Generate the autocompletion script for the specified shell"
canonicalId: "prepared-authoring-v2:plugin-kit-ai completion"
section: "api"
surface: "authoring-cli"
locale: "en"
generated: true
editLink: false
translationRequired: false
namespace: "prepared-authoring-v2"
status: "released"
released: true
sourceSHA: "ec9397a63f7c7550a680fcb0f5565902acbf7243"
factoryBaselineSHA: "070663efb27f69ecae8609e6b839f86f843efbb0"
sources:
  - "cli/plugin-kit-ai/cmd/agentplugins/release_root.go: c0465f90903c7ad3fcdc2283c241558d1b73bd9633f0e6af247ccd692a0e155e"
  - "cli/plugin-kit-ai/cmd/plugin-kit-ai/release_compat.go: d2f11a845c116160114ddd1b0ba8b825e9e5a4794354742749db7122c0c674c8"
  - "cli/plugin-kit-ai/go.mod: d388e12cb393cca6fcf8034e4a05d3c4eb8777427fed47ed064ef0623c377606"
  - "cli/plugin-kit-ai/go.sum: 0cf114be6b68dd165b75b588776e0bb7b9cc4aaa08fae382f7fa42b580bfa2da"
  - "cli/plugin-kit-ai/internal/agentpluginscli/add.go: 48beb684fc6e99c59d1e4a2ba8ab2b2a895c0ba6e6a5130f5a9bdef7bc0dc456"
  - "cli/plugin-kit-ai/internal/agentpluginscli/add_multi.go: c8638e7b9b043bc80ced72d683e363f8ad183ebaf42204b6ff8c67da0e128c1a"
  - "cli/plugin-kit-ai/internal/agentpluginscli/app.go: dd3770fc553ddc3d6ddbc830ea8aca4e5fdcf7d90d5c142ec8728b4454aa7771"
  - "cli/plugin-kit-ai/internal/agentpluginscli/binding.go: 873ab086ce83699f10ef9355d6f3e595cb16c1dc9ab4a29aef17250faa123b70"
  - "cli/plugin-kit-ai/internal/agentpluginscli/chatgpt_guidance.go: 7dacf7fcb0581cfff8f7aabfc85cd21cef15b93486376f31eba25ad39711e19a"
  - "cli/plugin-kit-ai/internal/agentpluginscli/color_error.go: 950bda3855f187a6670e6de60eda11757169f5d37b3d04fb70811706be4c1930"
  - "cli/plugin-kit-ai/internal/agentpluginscli/engine_boundary.go: fa6b55354bd983d1b84fd504b24b9949bd0a3d95ce674a197fd4a2faf55fd264"
  - "cli/plugin-kit-ai/internal/agentpluginscli/interactive_targets.go: 5bbc2d0effea1bf37c91f7f5aea82c85d080a3e9a6a62c08ab276b64e0cbbc7c"
  - "cli/plugin-kit-ai/internal/agentpluginscli/legacy_controller.go: 4ba1296edf3c0cdc6e8caf0b27b577ad8715231f4201d4e8e2f56679f1713326"
  - "cli/plugin-kit-ai/internal/agentpluginscli/lifecycle.go: 47947e8993581135caf6329f83f9d0d66214059f0a7a9ebce14f3a0dc49c22dd"
  - "cli/plugin-kit-ai/internal/agentpluginscli/outdated.go: 3ecf284023dfb158e6c1c60572a57da99062ff1c68a1ecf105db677e7cb1f723"
  - "cli/plugin-kit-ai/internal/agentpluginscli/opencode_runtime_notice.go: c213ce3f979273b1ee0f92e5ab07f287677fa996f7d2fc370bcde5d2d1c3636f"
  - "cli/plugin-kit-ai/internal/agentpluginscli/output.go: 571e944a73ac3cc89e8b66b84fb0274943913dd4ca2a28f953c5563d400ce8dc"
  - "cli/plugin-kit-ai/internal/agentpluginscli/preflight.go: 1d63ecceb199ae58feafbba48fbf332f3f2ebd531695ba03d1ce1a44b1d71e8b"
  - "cli/plugin-kit-ai/internal/agentpluginscli/prepare.go: eeee90f8626474fa870650849f1f22f333889affc9477dcdb277651e275409e1"
  - "cli/plugin-kit-ai/internal/agentpluginscli/prompt/prompt.go: fdaf0eb273e3f1574e00f33335953a3086e4ea627fc51765634d3f33f96005f2"
  - "cli/plugin-kit-ai/internal/agentpluginscli/prompts.go: 5af513ab2743428a6bb2b304b43e07c339c7846b8bb7547ee254a10a54285bd3"
  - "cli/plugin-kit-ai/internal/agentpluginscli/read.go: 709909a299641218133ec39dcfc5c2747506940e2b5f3a15e3eca476ecef1615"
  - "cli/plugin-kit-ai/internal/agentpluginscli/read_directory.go: a2ba8aa03cd1e8efc2a8b3b9aa8718aceb13822bb19d083d6fa32a76e316b9d1"
  - "cli/plugin-kit-ai/internal/agentpluginscli/read_reconciliation.go: 37f50b1155d26eb6695c8b50b14848aa8eed1b0ac0ee54d117d63fc4e67ed41a"
  - "cli/plugin-kit-ai/internal/agentpluginscli/remove_multi.go: 52dbe20bd0b3ce5a7b3df188f41cb863af3024667eda456410a1bffa48b983a2"
  - "cli/plugin-kit-ai/internal/agentpluginscli/repair_multi.go: f0ac13678c89c473b3f42082fb9c61f1759540f01a58d7139998d9357134d334"
  - "cli/plugin-kit-ai/internal/agentpluginscli/root.go: 04a77692a6950c85afa6ba06ea3efb81becf0ddfab21220f00e3dba242e39118"
  - "cli/plugin-kit-ai/internal/agentpluginscli/search.go: d954c3b68d6bcfe6b821855aed6c14e712b9ff0eff0bf67206a1191f714fa998"
  - "cli/plugin-kit-ai/internal/agentpluginscli/security.go: 2933cca3234ea86ba6a2d47c7f1a01d08fc38f4cbdda7981f3b8e16913421429"
  - "cli/plugin-kit-ai/internal/agentpluginscli/source.go: f8f1a5d225f77bc9f36a2d87b5388b81b424a6f9966502e2c6c9a3fbf680f2b1"
  - "cli/plugin-kit-ai/internal/agentpluginscli/state_migration.go: 5b24b5d5d83dd086ca98d8b8967f5936d6cb4be752720cc993e8c109f28a5630"
  - "cli/plugin-kit-ai/internal/agentpluginscli/target_batch.go: afa9dc9ba6a417229c57afda8a1eacbc397b38c0802880e1c14f5f8fc98fc81c"
  - "cli/plugin-kit-ai/internal/agentpluginscli/update_all.go: 7b791dc9fe04cf3ceae4f3ade5c36c308104761242cb3cb37474d1551961cc65"
  - "cli/plugin-kit-ai/internal/agentpluginscli/update_multi.go: 0ff5bedf401e3736178ea7274e4e7b3eac113867288f0f86bb354ae724991f23"
  - "cli/plugin-kit-ai/internal/agentpluginscli/validate.go: 8c643a08c657431d623197364f5f47d7f23ed0b4e6cf926f40a521a8bac7ee9d"
  - "cli/plugin-kit-ai/internal/authoring/commands/commands.go: 9ed49814d6145f61e28ac8c5b914383749f06d946dcece15db2c1059fd290220"
  - "cli/plugin-kit-ai/internal/authoring/commands/public_contract.go: 39c79f491f0733d352ffc0fa3a8ff4eaa169612e8876e92e5fb74c856663ec65"
  - "cli/plugin-kit-ai/internal/authoring/commands/skills.go: 566e36e02d58c5d76e92369987d901feb1bcb6f3e59d3225e02961fa041d606b"
  - "cli/plugin-kit-ai/internal/authoring/commands/version.go: ffe6cfef352faeb9a3c00722a628a2093876c14120cd6725131b3e105fda6b29"
  - "cli/plugin-kit-ai/internal/authoringcli/command.go: ab3ea92ca93a676a49cb99898a23386e1766e6f8becb2468bcfbb84ec22e1ec4"
  - "cli/plugin-kit-ai/internal/authoringcli/flags.go: 5f745c810e90586899cff2170433dbc58483ba17496167738387388f29be49c0"
  - "cli/plugin-kit-ai/internal/authoringcli/release.go: 45cb7a7000a31bb8477d1301fc70ce4521140b9f2a4760d5f84da3283134877f"
  - "go.mod: 352e528c2b2c34df21276c1f9b54c32f79656dad42c310da6b551f022119ce8c"
  - "go.work: 5d34933494465aac01f6e8149744cab2e0deb675647e66f587e63b1144793943"
  - "go.work.sum: 806569ceb74454bbf59a2f7a5de9513276726c204c4972ce303779bad40fba25"
  - "install/integrationctl/agentplugins/domain/acquisition.go: ea60178232db888d8df99a7b6722bb26b7a3f1a23776a6be041cf9c84f7a2cb3"
  - "install/integrationctl/agentplugins/domain/catalog.go: e70853260635bed305ce77fef0c420e2028a57d77a9ce0565ab9cf8b275a98bc"
  - "install/integrationctl/agentplugins/domain/chatgpt_mapping.go: a124fe2ad1c349628b36bd901ea280c6401487aa341a05a0216d379d7cac3669"
  - "install/integrationctl/agentplugins/domain/clients.go: 4c6c135e19c405a9321ea5a452e5ee62bd8a6749cea8b125f07d6eac6edec5c0"
  - "install/integrationctl/agentplugins/domain/directory.go: d860da1142fdc01ff1857f3c2d44d8de481c23c8a5a966d5eb5af0a2f8477c8f"
  - "install/integrationctl/agentplugins/domain/directory_context7_preparation.go: 6bb51f159351154fbd04a2a5dad8d5232fdd984aeb9c3726a7c0c49ec746011d"
  - "install/integrationctl/agentplugins/domain/errors.go: 7229bf792c60bcbb290050a6659f129f09e85e22174d32ac681a4c6e5bb6efd8"
  - "install/integrationctl/agentplugins/domain/identity.go: 85a3ab1c67a3137f9de3492a99c8ad8e005e7723a77952f389f0748eb9f58382"
  - "install/integrationctl/agentplugins/domain/install_intent.go: a0921941f3c3e59d59441b43da575e814f3617bdfda89a02246654a7871897a7"
  - "install/integrationctl/agentplugins/domain/security.go: 6773e2d0fc94bf6cf8a521c1c1dd8a339648bf1634ef5eebebc2917b06ff0abd"
  - "install/integrationctl/agentplugins/domain/selection.go: 9d1cde69bfc2829da904e26a3881bc0dabb8f1e143b817391a7e05022d7e4a1f"
  - "install/integrationctl/agentplugins/domain/state.go: 3410f0a1f9c697f6538682d79ac4d6db7f7e7a29aaf36936cf6298e0e65623a3"
  - "install/integrationctl/agentplugins/domain/types.go: 5428390af193ec99661203ea07fd103b7ed3823318c249592c2d6dcbd6722442"
  - "install/integrationctl/go.mod: 17c94b7bcbace5f9e4ee6164e0ff8499b7994923105c9430c84de7964b416116"
  - "install/integrationctl/go.sum: 55d21b3e3f4a7928cf9064f7ccd06cf54b643097cb13c1b776b07a6449b85458"
  - "install/plugininstall/go.mod: 7d0745754d1ae04fb31d5056cbd2f4fa82d711d0536c1c868f40452cf5e6bb53"
  - "sdk/go.mod: 79e8f3d4903364c498fffcf34e9394f84437ae198dcefcd1c0c99cb41c80ba5a"
stability: "public-stable"
maturity: "stable"
publicVisibility: "public"
localeStrategy: "canonical-en"
sourceKind: "authoring-docs-adapter"
sourceRef: "https://github.com/777genius/universal-agent-plugins/tree/ec9397a63f7c7550a680fcb0f5565902acbf7243/cli/plugin-kit-ai/internal/authoring/commands"
---
> Milestone A reference from the released authoring engine. [Exact source](https://github.com/777genius/universal-agent-plugins/tree/ec9397a63f7c7550a680fcb0f5565902acbf7243/cli/plugin-kit-ai/internal/authoring/commands).

<!-- namespace: prepared-authoring-v2; status: released; source-sha: ec9397a63f7c7550a680fcb0f5565902acbf7243 -->

Released Milestone A reference.

# plugin-kit-ai completion

Generate the autocompletion script for the specified shell

### Synopsis

Generate the autocompletion script for plugin-kit-ai for the specified shell.
See each sub-command's help for details on how to use the generated script.


### Options

```
  -h, --help   help for completion
```

### Options inherited from parent commands

```
      --color string    human output color: auto, never, always (default "auto")
      --dry-run         show the plan without changes (supported commands only)
      --format string   output format: human or json (default "human")
      --no-color        disable color output
      --plain           use accessible line prompts (independent of color)
      --target string   target clients, comma-separated (supported commands only)
```

### SEE ALSO

* [plugin-kit-ai](/en/api/cli/prepared-authoring-v2-plugin-kit-ai)	 - Build Agent Plugins packages
* [plugin-kit-ai completion bash](/en/api/cli/prepared-authoring-v2-plugin-kit-ai-completion-bash)	 - Generate the autocompletion script for bash
* [plugin-kit-ai completion fish](/en/api/cli/prepared-authoring-v2-plugin-kit-ai-completion-fish)	 - Generate the autocompletion script for fish
* [plugin-kit-ai completion powershell](/en/api/cli/prepared-authoring-v2-plugin-kit-ai-completion-powershell)	 - Generate the autocompletion script for powershell
* [plugin-kit-ai completion zsh](/en/api/cli/prepared-authoring-v2-plugin-kit-ai-completion-zsh)	 - Generate the autocompletion script for zsh
