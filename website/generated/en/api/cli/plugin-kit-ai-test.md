---
title: "plugin-kit-ai test"
description: "Run stable fixture-driven smoke tests against the launcher entrypoint"
canonicalId: "command:plugin-kit-ai:test"
surface: "cli"
section: "api"
locale: "en"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai test"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai test" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai test

Generated from the live Cobra command tree.

Run stable fixture-driven smoke tests against the launcher entrypoint

## plugin-kit-ai test

Run stable fixture-driven smoke tests against the launcher entrypoint

### Synopsis

Run stable Claude or Codex runtime smoke tests from JSON fixtures.

The command loads a fixture, invokes the configured launcher entrypoint with the correct carrier
(stdin JSON for Claude stable hooks, argv JSON for Codex notify), and optionally compares or updates
golden stdout/stderr/exitcode files for CI-grade regression checks.

Gemini has a production-ready 9-hook Go runtime with dedicated runtime gates and stays outside this stable fixture surface.
For Gemini use go test, generate --check, validate --strict, inspect, capabilities --mode runtime,
make test-gemini-runtime, then gemini extensions link . and optionally make test-gemini-runtime-live.

```
plugin-kit-ai test [path] [flags]
```

### Options

```
      --all                 run every stable event for the selected platform
      --event string        stable event to execute (for example Stop, PreToolUse, UserPromptSubmit, or Notify)
      --fixture string      fixture JSON path for single-event runs (default: fixtures/&lt;platform&gt;/&lt;event&gt;.json)
      --format string       output format: text or json (default "text")
      --golden-dir string   golden output directory (default: goldens/&lt;platform&gt;)
  -h, --help                help for test
      --platform string     target override ("claude" or "codex-runtime")
      --update-golden       write current stdout/stderr/exitcode outputs into the golden files
```

### SEE ALSO

* plugin-kit-ai	 - plugin-kit-ai CLI - scaffold and tooling for AI plugins
