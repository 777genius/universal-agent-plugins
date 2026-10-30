# ADR: Public Docs Platform

Status: Accepted; publication scope updated 2026-10-29

Decision:

- Build the public docs site in `website/`, not on top of the repo `docs/` tree.
- Keep maintainer/internal docs versioned in `docs/` and exclude them from public build, navigation, search, sitemap, and SEO.
- Use `vitepress@2.0.0-alpha.18` as the generating layer with exact version pinning.
- Generate API reference through official or established generators:
  - Cobra docs for CLI
  - gomarkdoc for public Go packages
  - TypeDoc plus `typedoc-plugin-markdown` for the Node runtime
  - pydoc-markdown for `plugin-kit-ai-runtime`
  - repo-native descriptor exports for events, capabilities, and support summaries
- Normalize generator output into a unified registry and deterministic markdown tree before the VitePress build.
- Publish current hand-authored guides in all nine landing languages under
  `/docs/{en,ru,uk,zh,es,hi,ar,pt,fr}/`, with `/docs/` reserved for a noindex
  language gateway. Use native language names and language-aware canonical
  page switching. Arabic uses RTL layout with isolated LTR technical syntax.
- Keep the pinned authoring API canonical English and label genuine English
  fallbacks. Expanding current guides does not republish archived material or
  modify immutable source provenance and historical locale snapshots.

Consequences:

- Public docs information architecture stays independent from maintainer process docs.
- Generator churn is isolated behind the docs pipeline instead of leaking into site structure.
- Generated output remains reviewable in git diffs because committed markdown and registries are source artifacts, not build artifacts.
