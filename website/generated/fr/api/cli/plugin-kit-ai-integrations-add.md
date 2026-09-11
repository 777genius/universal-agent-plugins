---
title: "plugin-kit-ai integrations add"
description: "Plan installation of an integration across supported agent targets"
canonicalId: "command:plugin-kit-ai:integrations:add"
surface: "cli"
section: "api"
locale: "fr"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai integrations add"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai integrations add" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai integrations add

Généré à partir de l'arbre réel de commandes Cobra.

Plan installation of an integration across supported agent targets

## plugin-kit-ai integrations add

Plan installation of an integration across supported agent targets

```
plugin-kit-ai integrations add &lt;source&gt; [flags]
```

### Options

```
      --adopt-new-targets string   policy for newly supported targets: manual or auto (default "manual")
      --auto-update                desired auto-update policy (default true)
      --dry-run                    plan only without mutating native targets (default true)
  -h, --help                       help for add
      --pre                        allow prerelease updates
      --scope string               scope intent for the planned installation (default "user")
      --target strings             limit planning to one or more targets
```

### SEE ALSO

* plugin-kit-ai integrations	 - Foundation lifecycle commands for multi-agent integration management
