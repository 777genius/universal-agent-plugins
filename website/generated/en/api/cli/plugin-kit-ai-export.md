---
title: "plugin-kit-ai export"
description: "Create a portable interpreted-runtime bundle without changing install semantics"
canonicalId: "command:plugin-kit-ai:export"
surface: "cli"
section: "api"
locale: "en"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai export"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai export" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai export

Generated from the live Cobra command tree.

Create a portable interpreted-runtime bundle without changing install semantics

## plugin-kit-ai export

Create a portable interpreted-runtime bundle without changing install semantics

### Synopsis

Create a deterministic portable .tar.gz bundle for launcher-based interpreted runtime projects.

This beta surface is a bounded handoff/export flow for python, node, and shell runtime repos.
It does not extend plugin-kit-ai install, and it does not imply marketplace packaging or dependency-preinstalled installs.

```
plugin-kit-ai export [path] [flags]
```

### Options

```
  -h, --help              help for export
      --output string     write bundle to this .tar.gz path (default: &lt;root&gt;/&lt;name&gt;_&lt;platform&gt;_&lt;runtime&gt;_bundle.tar.gz)
      --platform string   target override ("codex-runtime" or "claude")
```

### SEE ALSO

* plugin-kit-ai	 - plugin-kit-ai CLI - scaffold and tooling for AI plugins
