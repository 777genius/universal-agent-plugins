---
title: "One Project, Multiple Targets"
description: "How to decide when one repo should grow to more outputs, when it should stay narrow, and when it is time to split."
canonicalId: "page:guide:one-project-multiple-targets"
section: "guide"
locale: "en"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

# One Project, Multiple Targets

Use this page after the first working repo, when the real question becomes: should this same repo grow, and if so, how far?

## The Short Rule

One repo can safely cover more than one output when the same plugin logic, release intent, and ownership model still hold together.

## When One Repo Should Grow

Grow the same repo when:

- the plugin behavior is still one coherent product
- the new output is another way to deliver the same plugin
- one team can still own the authored source cleanly
- regeneration and validation still keep the repo easy to review

## When One Repo Should Stay Narrow

Keep the repo focused when the current output already solves the real need and extra outputs would only add maintenance overhead.

## When To Split Repos

Split repos when the product stops being one thing in practice:

- different teams own the work
- release timing diverges
- behavior diverges beyond simple target adaptation
- the repo would become harder to reason about than two smaller repos

## The Safe Mental Model

Start narrow, validate one working output, and only then grow the repo with another supported output.
