# Retired plugin-kit-ai release workflows

These files preserve the last executable implementations of the retired
`plugin-kit-ai` CLI release, preflight, npm, PyPI, and Homebrew publication
workflows, plus the paired `agentplugins` / `plugin-kit-ai` promotion workflow.
They are historical source, not GitHub Actions entrypoints: keeping them under
`docs/history/` ensures GitHub cannot dispatch or trigger them.

The files are retained for capability and design review under
`docs/AUTHORING_CAPABILITY_PRESERVATION.md`. Do not copy them back into
`.github/workflows`, dispatch them, or treat them as current release guidance.
The sole public CLI is `agentplugins`; its current release workflows remain in
`.github/workflows/agentplugins-release.yml` and
`.github/workflows/agentplugins-npm-publish.yml`.

The separate `plugin-kit-ai-runtime` helper-package publishers are intentionally
retained as manual-only workflows. They are not CLI distribution channels and
no longer trigger from the retired `Release Assets` workflow.
