---
title: "Glossary"
description: "Short definitions for the public terms used across plugin-kit-ai docs."
canonicalId: "page:reference:glossary"
section: "reference"
locale: "en"
generated: false
translationRequired: true
---

# Glossary

Use this page when a docs term is slowing you down. The goal is not perfect theory. The goal is a fast shared meaning.

## Authored State

The part of the repo your team owns directly. `generate` turns this source into target-specific output.

## Generated Target Files

Files produced for a specific target after generating. They are real delivery output, but they are not the long-term source of truth.

## Path

A practical way to build and ship the plugin. Examples include the default Go runtime path, the local Node/TypeScript path, and repo-owned integration setup.

## Target

The output you are aiming at, such as `codex-runtime`, `claude`, `codex-package`, `gemini`, `opencode`, or `cursor`.

## Runtime Path

A path where the repo owns executable plugin behavior directly.

## Package Or Extension Path

A path focused on producing the right package or extension artifact instead of the main executable runtime shape.

## Repo-Owned Integration Setup

A path where the repo mainly ships checked-in configuration for another tool or workspace.

## Install Channel

A way to install the CLI, such as Homebrew, npm, PyPI, or the verified script. It is not a public runtime API.

## Shared Runtime Package

The `plugin-kit-ai-runtime` dependency used by approved Python and Node flows instead of copying helper files into every repo.

## Support Boundary

The public line between what the project recommends by default, what it supports more carefully, and what stays experimental.

## Readiness Gate

The check you should treat as the signal that a repo is healthy enough to hand off. For most repos this is `validate --strict`.

## Handoff

The point where another teammate, another machine, or another user can use the repo without hidden setup knowledge.

Related pages: [Target Model](/en/concepts/target-model), [Support Boundary](/en/reference/support-boundary), and [Production Readiness](/en/guide/production-readiness).
