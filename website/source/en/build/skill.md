---
title: "Build a standalone Skill"
description: "Create a standard package containing one useful instruction Skill."
canonicalId: "page:build:skill"
section: "build"
locale: "en"
generated: false
translationRequired: true
---

# Build a standalone Skill

> **Prepared, unreleased authoring contract.** This public page documents a reviewed
> future authoring workflow. Future authoring commands described here are not available in current
> releases; they do not claim that `plugin-kit-ai@2` is available. Existing installer
> examples are identified separately. Public activation remains pending.

Use this path when the agent needs a repeatable method, checklist, or writing
instruction and does not need an MCP tool connection. The package contains one
Skill and a root manifest. No server, runtime installation, or client profile is
needed for the authoring steps.

## Choose an explicit destination

The examples assume your working directory is a disposable parent directory and
`./review-helper` does not exist. Choose a lowercase portable package identity.
The path names the destination; `--name` names the package inside its manifest.

Future `agentplugins author` invocation:

```bash
agentplugins author init ./review-helper \
  --template skill \
  --name review-helper \
  --description 'Review documentation changes for clarity and evidence' \
  --skill-name review-docs
```

Equivalent future `plugin-kit-ai` invocation; choose one, not both:

```bash
plugin-kit-ai init ./review-helper \
  --template skill \
  --name review-helper \
  --description 'Review documentation changes for clarity and evidence' \
  --skill-name review-docs
```

Init requires an absent destination and does not overwrite an existing directory.
If creation reports a committed result with a later failure, inspect the result
before retrying. There is no overwrite flag in this workflow.

## Review the created files

```text
review-helper/
  plugin.json
  README.md
  .gitignore
  skills/
    review-docs/
      SKILL.md
```

The generated Skill is a starting instruction. Replace its body with a concrete
method useful to your users. Keep the Skill name and directory aligned.
For example, the body of `skills/review-docs/SKILL.md` could be:

```markdown
# Review documentation changes

Use this Skill when reviewing a documentation change before handoff.

1. Identify the intended reader and the task the page helps them finish.
2. Check that every command names its input and describes its effect.
3. Separate verified behavior from prerequisites and untested claims.
4. Report unclear instructions with a suggested replacement sentence.

Return a short list of findings with file locations. If no issue is found,
state which pages you reviewed and which checks remain outside the review.
```

Keep the generated frontmatter above that body. Its `name` and `description`
identify the Skill and help the agent decide when to use it. Instructions in a
Skill do not grant permission to run scripts or access private resources.

## Check the exact package root

```bash
agentplugins author validate ./review-helper
agentplugins author skills validate ./review-helper
agentplugins author inspect ./review-helper
agentplugins author test ./review-helper
```

For the other entrypoint, replace `agentplugins author` with `plugin-kit-ai` and
keep every path and argument unchanged. These checks are static. They do not
ask an agent to follow the instructions or score the quality of its response.

Review the report's conformance and readiness separately. A well-formed Skill
still needs human review for usefulness, accuracy, and appropriate scope.

Continue with [extra Skills](./skills), [target checks](./checks), or the
[local installer handoff](./handoff). To add tools as well as instructions,
choose the [hybrid journey](./hybrid) for a new package.
