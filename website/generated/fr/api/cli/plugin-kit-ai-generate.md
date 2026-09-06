---
title: "plugin-kit-ai generate"
description: "Compile native target artifacts from the package graph"
canonicalId: "command:plugin-kit-ai:generate"
surface: "cli"
section: "api"
locale: "fr"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai generate"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai generate" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai generate

Généré à partir de l'arbre réel de commandes Cobra.

Compile native target artifacts from the package graph

## plugin-kit-ai generate

Compile native target artifacts from the package graph

### Synopsis

Compile native target artifacts from the package graph discovered via canonical plugin/plugin.yaml plus the standard authored directories.

Claude and Codex runtime/package lanes generate their managed native artifacts from the package graph.
Gemini generation always produces the native extension package artifacts and may also carry the optional Go runtime lane when the authored project includes it; that lane now exposes a production-ready 9-hook runtime surface, but it still does not imply blanket runtime parity for future hooks beyond the promoted contract.
OpenCode generation is workspace-config-only: it produces opencode.json plus mirrored skills, commands, agents, themes, local plugin code, and plugin-local package metadata without introducing a launcher/runtime contract.
Cursor generation now defaults to the packaged plugin lane: it produces .cursor-plugin/plugin.json plus managed skills/ and optional .mcp.json from canonical portable inputs. Use target cursor-workspace only when you intentionally need repo-local .cursor config generation and mirrored .cursor/rules/**.

```
plugin-kit-ai generate [path] [flags]
```

### Options

```
      --check           fail if generated artifacts are out of date
  -h, --help            help for generate
      --target string   generate target ("all", "claude", "codex-package", "codex-runtime", "gemini", "opencode", "cursor", or "cursor-workspace") (default "all")
```

### SEE ALSO

* plugin-kit-ai	 - plugin-kit-ai CLI - scaffold and tooling for AI plugins
