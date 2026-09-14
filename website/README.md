# Public Docs Website

This workspace hosts the public VitePress 2 documentation site for Agent Plugins.

Architecture:

- `source/`: hand-authored public docs in `en` and `ru`
- `generated/`: committed generated docs and registries
- `.site/`: assembled runtime source tree for VitePress
- `tools/`: extractors, normalizers, enrichers, assemblers, and quality checks
- `.vitepress/`: generating layer only

Key commands:

- `pnpm install`
- `pnpm run docs:gen`
- `pnpm run docs:build`
- `pnpm run docs:check`
- `pnpm run docs:check-ui`

Rules:

- Do not edit `generated/` files by hand unless you are fixing the generator itself and immediately regenerating output.
- Do not add public docs content under the repo `docs/` tree.
- Do not route API generation directly into VitePress internals. Generators must emit source-level markdown and registries first.
