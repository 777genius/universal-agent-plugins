---
title: "plugin-kit-ai completion zsh"
description: "Generate the autocompletion script for zsh"
canonicalId: "command:plugin-kit-ai:completion:zsh"
surface: "cli"
section: "api"
locale: "en"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai completion zsh"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai completion zsh" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai completion zsh

Generated from the live Cobra command tree.

Generate the autocompletion script for zsh

## plugin-kit-ai completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" &gt;&gt; ~/.zshrc

To load completions in your current shell session:

	source &lt;(plugin-kit-ai completion zsh)

To load completions for every new session, execute once:

#### Linux:

	plugin-kit-ai completion zsh &gt; "${fpath[1]}/_plugin-kit-ai"

#### macOS:

	plugin-kit-ai completion zsh &gt; $(brew --prefix)/share/zsh/site-functions/_plugin-kit-ai

You will need to start a new shell for this setup to take effect.


```
plugin-kit-ai completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
      --no-descriptions   disable completion descriptions
```

### SEE ALSO

* plugin-kit-ai completion	 - Generate the autocompletion script for the specified shell
