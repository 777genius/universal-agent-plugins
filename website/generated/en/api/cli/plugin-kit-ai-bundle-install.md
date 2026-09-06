---
title: "plugin-kit-ai bundle install"
description: "Install a local exported Python/Node bundle into a destination directory"
canonicalId: "command:plugin-kit-ai:bundle:install"
surface: "cli"
section: "api"
locale: "en"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai bundle install"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai bundle install" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai bundle install

Generated from the live Cobra command tree.

Install a local exported Python/Node bundle into a destination directory

## plugin-kit-ai bundle install

Install a local exported Python/Node bundle into a destination directory

### Synopsis

Install a local .tar.gz bundle created by plugin-kit-ai export into a destination directory.

This stable local handoff surface only supports local exported Python/Node bundles for codex-runtime or claude.
It unpacks bundle contents safely, prints next steps, and does not extend the binary-only plugin-kit-ai install flow.

```
plugin-kit-ai bundle install &lt;bundle.tar.gz&gt; [flags]
```

### Options

```
      --dest string   destination directory for unpacked bundle contents
  -f, --force         overwrite an existing destination directory
  -h, --help          help for install
```

### SEE ALSO

* plugin-kit-ai bundle	 - Bundle tooling for exported interpreted-runtime handoff archives
