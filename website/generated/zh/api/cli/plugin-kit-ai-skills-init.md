---
title: "plugin-kit-ai skills init"
description: "Create a canonical SKILL.md skill package"
canonicalId: "command:plugin-kit-ai:skills:init"
surface: "cli"
section: "api"
locale: "zh"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai skills init"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai skills init" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai skills init

由实际的 Cobra 命令树生成。

Create a canonical SKILL.md skill package

## plugin-kit-ai skills init

Create a canonical SKILL.md skill package

```
plugin-kit-ai skills init [skill-name] [flags]
```

### Examples

```
  plugin-kit-ai skills init lint-repo --template go-command
  plugin-kit-ai skills init format-changed --template cli-wrapper --command "ruff format ."
  plugin-kit-ai skills init review-checklist --template docs-only
```

### Options

```
      --command string       default command for cli-wrapper template (default "replace-me")
      --description string   skill description
  -f, --force                overwrite existing authored files
  -h, --help                 help for init
  -o, --output string        project root containing skills/ (default ".")
      --template string      template ("go-command", "cli-wrapper", "docs-only") (default "go-command")
```

### SEE ALSO

* plugin-kit-ai skills	 - Experimental skill authoring tools
