---
title: "Add extra Skills"
description: "Extend an existing standard package with another immediate Skill."
canonicalId: "page:build:skills"
section: "build"
locale: "en"
generated: false
translationRequired: true
---

# Add extra Skills

> **Prepared, unreleased authoring contract.** This public page documents a reviewed
> future authoring workflow. The commands shown here are not available in current
> releases; they do not claim that `plugin-kit-ai@2` is available. Existing installer
> examples are identified separately. Public activation remains pending.

Use the future `skills init` job to add one Skill to an existing standard package.
It works for Skill, MCP, or hybrid packages. Select the package root containing
`plugin.json`, not the `skills/` directory or an individual `SKILL.md` file.

## Name the new Skill and package

Assume `./review-helper` is the package from the [standalone Skill journey](./skill)
and `skills/release-notes/` does not exist.

Future installer-hosted spelling:

```bash
agentplugins author skills init release-notes ./review-helper \
  --description 'Use when drafting release notes from reviewed changes'
```

Equivalent future authoring executable spelling; choose one:

```bash
plugin-kit-ai skills init release-notes ./review-helper \
  --description 'Use when drafting release notes from reviewed changes'
```

The positional Skill name is required, as is an explicit description of 1–1024
characters. Names are not normalized. Use an exact lowercase portable name with
hyphens, and choose a new destination instead of expecting an overwrite.

## Review the additional component

```text
review-helper/
  plugin.json
  skills/
    review-docs/
      SKILL.md
    release-notes/
      SKILL.md
```

Edit `skills/release-notes/SKILL.md` to explain its trigger, method, and expected
output. Keep its name and description accurate. For example, instruct it to
separate user-visible changes from maintenance changes and to cite the reviewed
source for each release claim.

Scripts, references, and assets can support instructions, but their presence is
not proof of execution or permission to run them. The `allowed-tools` field is
not independent authorization evidence. Review supporting material with the
same care as the instruction text.

## Validate the package and immediate Skills

```bash
agentplugins author skills validate ./review-helper
plugin-kit-ai skills validate ./review-helper
```

These are equivalent checks; one entrypoint is sufficient. The command reports
package-wide readiness and isolated immediate Skills. An error elsewhere in the
package can therefore affect the overall result even when the new Skill's own
findings are clean.

Omitting the package path uses only the current directory, with no ancestor
search. Explicit roots make scripts and troubleshooting clearer. Running from
inside `skills/release-notes/` is not a substitute for selecting the package root.

## Handle a failure without overwriting work

Read `committed` and the accompanying error guidance if creation fails. A Skill
can be committed before resulting package checks fail or remain incomplete.
Inspect the actual files and the report before retrying; a second init against
an existing destination will not overwrite it.

Correct the reported input or package issue and repeat validation. If the name
is wrong, review the intended source change explicitly rather than adding a
force flag. No external Skills install, update, remove, or generate lifecycle is
part of this v2 authoring journey.

Continue with [static package checks](./checks) and the [handoff](./handoff).
