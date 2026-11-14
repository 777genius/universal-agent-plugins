---
title: "Build A Claude Plugin"
description: "A focused guide for the stable Claude plugin path in plugin-kit-ai."
canonicalId: "page:guide:claude-plugin"
section: "guide"
locale: "en"
generated: false
translationRequired: true
---

# Build A Claude Plugin

Choose this path when you are explicitly targeting Claude hooks instead of the default Codex runtime path.

## Recommended Starting Point

```bash
plugin-kit-ai init my-claude-plugin --platform claude
cd my-claude-plugin
plugin-kit-ai generate .
plugin-kit-ai validate . --platform claude --strict
```

## What This Path Means

- the project targets Claude hook execution
- the stable subset is narrower than the full Claude runtime feature set
- `validate --strict` remains the main readiness check

## Use Extended Hooks Carefully

```bash
plugin-kit-ai init my-claude-plugin --platform claude --claude-extended-hooks
```

Only choose extended hooks when you intentionally want the wider supported set and you accept looser stability than the stable subset.

## Good Fit

- a plugin that must integrate with Claude runtime hooks
- teams that want one repo and one workflow instead of hand-editing native Claude artifacts
- users who need a stronger structure than ad-hoc local scripts

## Next Steps

- Read [Target Model](/en/concepts/target-model) to see how Claude differs from packaging or workspace-configuration targets.
- Check [Platform Events](/en/api/platform-events/claude) for event-level reference.
