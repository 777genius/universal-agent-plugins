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

The authoritative CI setup is [`.github/workflows/docs.yml`](../.github/workflows/docs.yml).
Full generation needs Node.js **22.21.1**, pnpm **8.15.1**, Go **1.26.10**,
Python **3.12**, the hash-pinned Python requirements and gomarkdoc **v1.1.0**.
Installing only the website's npm dependencies is insufficient.

The generated authoring API consumes the immutable **0.1.65** source selected by
`tools/lib/source-contract.mjs`. It must be a separate clean checkout, outside
the writable repository, output, tool and scratch directories. It is not the
current installer source or a substitute pin for the current Build guide.

The following Linux shell setup mirrors CI. Start at the repository root with
the exact tool versions above on `PATH`, complete Git history and the
`agentplugins-v0.1.65` tag available. Provisioning downloads dependencies; the
subsequent generation and consumer gate run with Go network resolution disabled.

```sh
docs_repo=$(pwd -P)
docs_workspace=$(mktemp -d /tmp/uap-docs-review.XXXXXXXX)
export DOCS_TOOLS_ROOT="$docs_workspace/tools"
export DOCS_GOMARKDOC="$DOCS_TOOLS_ROOT/bin/gomarkdoc"
export DOCS_AUTHORING_CHECKOUT="$docs_workspace/authoring-source"
export TMPDIR="$docs_workspace/tmp"
export GOCACHE="$docs_workspace/go-build-cache"
export GOMODCACHE="$docs_workspace/go-module-cache"
export GOTOOLCHAIN=local GOMAXPROCS=2 GOENV=off
export GOPROXY=https://proxy.golang.org,direct GOSUMDB=sum.golang.org
unset GOFLAGS
mkdir -p "$TMPDIR" "$GOCACHE" "$GOMODCACHE" \
  "$DOCS_TOOLS_ROOT/bin" "$docs_workspace/home"

(cd "$docs_repo/website" && pnpm install --frozen-lockfile)
python3.12 -m venv "$DOCS_TOOLS_ROOT/python-venv"
"$DOCS_TOOLS_ROOT/python-venv/bin/pip" install --require-hashes \
  -r "$docs_repo/website/tools/pydoc-markdown-requirements.txt"
GOBIN="$DOCS_TOOLS_ROOT/bin" go install \
  github.com/princjef/gomarkdoc/cmd/gomarkdoc@v1.1.0

docs_authoring_sha=$(node --input-type=module -e \
  'import { acceptedAuthoringSHA } from "./website/tools/lib/source-contract.mjs"; process.stdout.write(acceptedAuthoringSHA)')
git clone --no-local --no-checkout "$docs_repo" "$DOCS_AUTHORING_CHECKOUT"
git -C "$DOCS_AUTHORING_CHECKOUT" checkout --quiet --detach "$docs_authoring_sha"
chmod -R a-w "$DOCS_AUTHORING_CHECKOUT"

(cd "$DOCS_AUTHORING_CHECKOUT" && \
  GOWORK="$DOCS_AUTHORING_CHECKOUT/go.work" go build -p 2 \
  -o "$docs_workspace/authoring-preflight" ./cli/plugin-kit-ai/tools/authoring-docs)

export GOWORK="$docs_repo/go.work"
export GOPROXY=off GOSUMDB=off GOENV=off
unset GOFLAGS
cd "$docs_repo/website"
env HOME="$docs_workspace/home" pnpm run docs:check
```

Before repeating `docs:check`, `docs:build` or another production rebuild, remove
only the stale website output. Postbuild intentionally refuses to overwrite an
existing redirect alias:

```sh
rm -rf -- "$docs_repo/website/dist"
```

`docs:clean` also removes `DOCS_TOOLS_ROOT`, including externally provisioned
pinned tools and cached dependencies. Reserve it for a deliberate toolchain
reset followed by reprovisioning; use the narrow output removal for rebuilds.

Keep `TMPDIR` as a **sibling** of `DOCS_TOOLS_ROOT`, not inside it. The consumer
creates temporary immutable checkouts under `TMPDIR`; overlap with writable
tools is rejected by the provenance preflight. The private command-level `HOME`
keeps disposable integration state separate from the developer's profile.

`docs:check` includes tests, generation, locale/model/public-boundary and drift
checks, the production build and output verification. After it passes, provision
Chromium and check the built site with the same environment:

```sh
env HOME="$docs_workspace/home" pnpm exec playwright install --with-deps chromium
env HOME="$docs_workspace/home" pnpm run docs:smoke-ui
```

The Chromium install may require system package privileges on a fresh Linux
host; CI provisions it in its disposable runner. `docs:check-ui` combines a
fresh build with the browser check and needs the same complete setup.
`docs:dev` also generates before starting VitePress; `docs:preview` serves an
existing production build. Keep the scratch paths until the run and any failure
artifacts have been reviewed.

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
