---
title: "plugin-kit-ai install"
description: "Install a plugin binary from GitHub Releases (verified via checksums.txt)"
canonicalId: "command:plugin-kit-ai:install"
surface: "cli"
section: "api"
locale: "zh"
generated: true
editLink: false
stability: "historical"
maturity: "historical"
sourceRef: "cli:plugin-kit-ai install"
translationRequired: false
historicalVersion: "1.2.4"
sourceSHA: "9beca10448ac50fbe526a52101d1433a12471980"
status: "historical"
---

> Historical plugin-kit-ai v1, baseline **1.2.4** ([exact source](https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980)). Project migration is not available in v2 yet. Maintain legacy projects using the v1 1.2.4 command set.

<DocMetaCard surface="cli" stability="historical" maturity="historical" source-ref="cli:plugin-kit-ai install" source-href="https://github.com/777genius/universal-agent-plugins/tree/9beca10448ac50fbe526a52101d1433a12471980/cli/plugin-kit-ai" />

# plugin-kit-ai install

由实际的 Cobra 命令树生成。

Install a plugin binary from GitHub Releases (verified via checksums.txt)

## plugin-kit-ai install

Install a plugin binary from GitHub Releases (verified via checksums.txt)

### Synopsis

Downloads checksums.txt and a release asset for your GOOS/GOARCH, verifies SHA256, and writes the binary to --dir
(default bin). Asset selection: (1) a single *_&lt;goos&gt;_&lt;goarch&gt;.tar.gz (GoReleaser) — file extracted from archive root;
or (2) a raw binary named *-&lt;goos&gt;-&lt;goarch&gt; or *-&lt;goos&gt;-&lt;goarch&gt;.exe on Windows (e.g. claude-notifications-darwin-arm64).

Use exactly one of --tag or --latest. Draft releases are refused; prerelease requires --pre.
Optional --output-name sets the installed filename (single path segment).

This command installs third-party plugin binaries, not the plugin-kit-ai CLI itself (build plugin-kit-ai from source or use a release installer).

```
plugin-kit-ai install [owner/repo] [flags]
```

### Options

```
      --dir string            directory for the installed binary (created if missing) (default "bin")
  -f, --force                 overwrite existing binary
      --github-token string   GitHub token (optional; default from GITHUB_TOKEN env)
      --goarch string         target GOARCH override (default: host GOARCH)
      --goos string           target GOOS override (default: host GOOS)
  -h, --help                  help for install
      --latest                install from GitHub releases/latest (non-prerelease) instead of --tag
      --output-name string    write binary under this filename in --dir (default: name from archive)
      --pre                   allow GitHub prerelease (non-stable) releases
      --tag string            Git release tag (required unless --latest), e.g. v0.1.0
```

### SEE ALSO

* plugin-kit-ai	 - plugin-kit-ai CLI - scaffold and tooling for AI plugins
