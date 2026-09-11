---
title: "plugin-kit-ai validate"
description: "Validate a package-standard plugin-kit-ai project"
canonicalId: "command:plugin-kit-ai:validate"
surface: "cli"
section: "api"
locale: "en"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai validate"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai validate" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai validate

Generated from the live Cobra command tree.

Validate a package-standard plugin-kit-ai project

## plugin-kit-ai validate

Validate a package-standard plugin-kit-ai project

### Synopsis

Validate a package-standard plugin-kit-ai project.

Text mode is the human-readable default and prints Warning:/Failure: lines.
Use --format json for CI or automation. That mode emits the versioned
"plugin-kit-ai/validate-report" contract with schema_version=1 and an
explicit outcome of "passed", "failed", or "failed_strict_warnings".

```
plugin-kit-ai validate [path] [flags]
```

### Options

```
      --format string     output format ("text" or "json") (default "text")
  -h, --help              help for validate
      --platform string   target override ("codex-package", "codex-runtime", "claude", "gemini", "opencode", "cursor", or "cursor-workspace")
      --strict            treat validation warnings as errors
```

### SEE ALSO

* plugin-kit-ai	 - plugin-kit-ai CLI - scaffold and tooling for AI plugins
