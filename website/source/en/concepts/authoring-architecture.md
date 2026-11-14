---
title: "Project Source And Outputs"
description: "How authored files, generated outputs, strict validation, and handoff fit together in plugin-kit-ai."
canonicalId: "page:concepts:authoring-architecture"
section: "concepts"
locale: "en"
generated: false
translationRequired: true
aside: true
outline: [2, 3]
---

# Project Source And Outputs

This page is narrower than the main product model. It explains the working boundary inside the repo: what you author, what gets generated, and why that split keeps the project maintainable.

## The Core Shape

```text
project source -> generate -> target outputs -> validate --strict -> handoff
```

The source stays stable. The outputs can change per target. Validation makes sure the generated result is still safe to hand off.

## Authored Files vs Generated Files

Authored files are the part of the repo you are expected to maintain directly.

Generated files are build artifacts for the targets you chose. They are real delivery output, but they are not the place where the project truth should drift.

That distinction keeps the repo readable and makes regeneration safe.

## Why The Split Matters

Without a clear split, teams end up editing generated output, losing repeatability, and making upgrades harder than they need to be.

With a clear split, you can:

- review source changes directly
- regenerate output confidently
- validate the same delivery shape every time
- add another supported output later without rebuilding the repo from scratch

## How This Relates To The Bigger Model

If you want the higher-level explanation, start with [How plugin-kit-ai Works](/en/concepts/managed-project-model).
