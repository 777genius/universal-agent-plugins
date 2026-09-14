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

> **Milestone A is available.** Install `agentplugins` from
> `universal-agent-plugins@0.1.65` on npm or the `777genius/agentplugins` Homebrew tap.
> GitHub release: `agentplugins-v0.1.65`. Public-channel E2E is verified.
> Runtime/dev/bootstrap, migration, export and publication remain deferred.
> Historical YAML v1 workflows remain separate and preserved.

Use this path when the agent needs a repeatable method, checklist, or writing
instruction and does not need an MCP tool connection. The package contains one
Skill and a root manifest. No server, runtime installation, or client profile is
needed for the authoring steps.

## Choose an explicit destination

The examples assume your working directory is a disposable parent directory and
`./review-helper` does not exist. Choose a lowercase portable package identity.
The path names the destination; `--name` names the package inside its manifest.

```bash
agentplugins author init ./review-helper \
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

These checks are static. They do not
ask an agent to follow the instructions or score the quality of its response.

Review the report's conformance and readiness separately. A well-formed Skill
still needs human review for usefulness, accuracy, and appropriate scope.

Continue with [extra Skills](./skills), [target checks](./checks), or the
[local installer handoff](./handoff). To add tools as well as instructions,
choose the [hybrid journey](./hybrid) for a new package.
