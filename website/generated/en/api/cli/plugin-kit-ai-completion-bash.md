---
title: "plugin-kit-ai completion bash"
description: "Generate the autocompletion script for bash"
canonicalId: "command:plugin-kit-ai:completion:bash"
surface: "cli"
section: "api"
locale: "en"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai completion bash"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai completion bash" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai completion bash

Generated from the live Cobra command tree.

Generate the autocompletion script for bash

## plugin-kit-ai completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source &lt;(plugin-kit-ai completion bash)

To load completions for every new session, execute once:

#### Linux:

	plugin-kit-ai completion bash &gt; /etc/bash_completion.d/plugin-kit-ai

#### macOS:

	plugin-kit-ai completion bash &gt; $(brew --prefix)/etc/bash_completion.d/plugin-kit-ai

You will need to start a new shell for this setup to take effect.


```
plugin-kit-ai completion bash
```

### Options

```
  -h, --help              help for bash
      --no-descriptions   disable completion descriptions
```

### SEE ALSO

* plugin-kit-ai completion	 - Generate the autocompletion script for the specified shell
