# Public documentation website

This workspace builds the VitePress 2 documentation published at
[universal-agent-plugins.com/docs](https://universal-agent-plugins.com/docs/en/).

## Source and publication boundary

- `source/`: hand-authored documentation in English, Russian, Spanish, French
  and Simplified Chinese (`en`, `ru`, `es`, `fr`, `zh`).
- `generated/`: committed generated API references and registries.
- `.site/`: assembled runtime source tree for VitePress.
- `tools/`: extractors, normalizers, assembly and quality checks.
- `.vitepress/`: site configuration and presentation.

Only the approved home, Use and Build journeys, quickstart, client
compatibility reference and authoring API routes are public. Other source
pages preserve earlier documentation and remain outside the public build.
The publication model and quality checks define the exact routes; a Markdown
file's presence in `source/` does not publish it.

The five maintained locales must give the same release availability and
functional guidance. Translate prose and keep command names, flags, paths,
manifest keys and evidence identities exact. Preserve historical-source blocks
and the locale disposition registry when updating active pages. An English
fallback must be identified as a fallback; it is not a completed translation.

## Build and checks

Use the versions pinned in `package.json` (Node.js 22.21.1 and pnpm 8.15.1):

```sh
pnpm install --frozen-lockfile
pnpm run docs:gen
pnpm run docs:check
pnpm run docs:check-ui
```

`docs:check` runs the documentation tests, generation, locale/model/public-boundary
and generated-drift checks, production build and output checks. `docs:check-ui`
builds the site and runs the browser smoke check. `docs:dev` starts local
preview; `docs:preview` serves an existing production build.

Do not edit generated files by hand. Fix the generator or its source input,
regenerate and review the resulting diff. Generators emit source-level Markdown
and registries before assembly; do not generate API content directly inside
VitePress internals. Maintainer runbooks belong in the root `docs/` tree, not
on public routes.

## Availability and evidence

Write user instructions against a verified public CLI release. Clearly label
commands that require a qualified authoring build or current source. Generated
API provenance can refer to an older qualified authoring artifact and does not
mean the latest installer exposes those commands.

A schema pass proves structure. Installer tests prove the recorded lifecycle
contract. Native discovery, MCP initialization, authentication and model/tool
execution require their own client-, version-, platform- and scope-specific
evidence. Keep those distinctions in all maintained languages.
