# Direct complete package sources

**SOURCE ONLY / UNRELEASED.** These transports are implemented on the source
feature branch. They are not a claim that a published installer already includes
this behavior, or that every client supports every package component.

A direct source must contain a complete standard Agent Plugins package with
`plugin.json`. Its skills, MCP configuration and other components use the existing
standard loader and target compatibility/security checks. A standalone `SKILL.md`
is not imported as a plugin, and UAP does not crawl an HTTPS directory listing.

## Accepted inputs

```bash
# Arbitrary HTTPS Git, tracking the remote's default branch
agentplugins add 'https://git.example.com/team/tools.git' --target gemini

# Explicit branch/tag, or an immutable full lowercase 40-character Git commit
agentplugins add 'https://git.example.com/team/tools.git#refs/heads/main//plugins/demo' --target gemini
agentplugins add 'https://git.example.com/team/tools.git#0123456789012345678901234567890123456789//plugins/demo' --target gemini

# Extensionless Git clone endpoint, without archive auto-detection
agentplugins add 'git+https://git.example.com/team/tools' --target gemini

# Native SSH credentials and host verification
agentplugins add 'ssh://git@git.example.com:2222/team/tools.git#main//plugins/demo' --target gemini
agentplugins add 'git@git.example.com:team/tools.git#main//plugins/demo' --target gemini

# Provider browser URLs
agentplugins add 'https://github.com/owner/repo/tree/main/plugins/demo' --target gemini
agentplugins add 'https://gitlab.com/group/subgroup/repo/-/tree/main/plugins/demo' --target gemini
agentplugins add 'https://dev.azure.com/org/project/_git/repo?version=GBmain&path=%2Fplugins%2Fdemo' --target gemini

# Ready ZIP, TAR, TAR.GZ or TGZ packages; extensionless HTTPS is also detected
agentplugins add 'https://downloads.example.com/plugin.zip' --target gemini
agentplugins add 'https://downloads.example.com/plugin.tar.gz' --target gemini
agentplugins add 'https://downloads.example.com/latest-package' --target gemini

# A complete local archive is installed without portable dependency intent
agentplugins add './plugin@1.0.zip' --target gemini
```

`github:owner/repo@FULL_SHA//path` and historical exact `owner/repo@FULL_SHA`
inputs remain compatible. Bare `owner/repo` continues to select the signed
Directory workflow; use an HTTPS clone URL or explicit `github:` prefix for a
direct Git source. SCP syntax supports an explicit SSH user, including SSH host
aliases, and unambiguous dotted hosts without a user. Absolute SCP paths such as
`git@host:/srv/git/repo.git` retain their native meaning.

GitHub and GitLab repository URLs normalize to clone endpoints. GitLab tree URLs
with `/-/tree/` also work on self-hosted domains. Azure consumes only its recognized
`path`, `version` and `_a=contents` browser parameters; `GB`, `GT` and `GC` identify
a branch, tag or exact commit. Other remote URL queries, passwords and credential
fragments are refused. SSH usernames remain routing information. Azure's
username-only organization spelling is normalized away.

A tree URL containing a slash in a branch name is resolved against exact remote
heads/tags. If more than one branch/path split matches, UAP refuses the ambiguity;
use a clone URL with `#refs/heads/branch/name//package/path`. An encoded slash is not
a way to resolve provider URL ambiguity. Ordinary endpoint escaping is preserved.

## Saved intent, updates and restore

Eligible remote adds are saved by default. `--alias` chooses the dependency's
logical name; `--no-save` keeps an imperative installation. Existing client
admission rules still apply. Local archive paths remain imperative because they
cannot be restored portably on another computer.

```toml
schema_version = 1

[dependencies.tools]
git = "ssh://git@git.example.com/team/tools.git"
ref = "refs/heads/main"
path = "plugins/demo"

[dependencies.release]
archive = "https://downloads.example.com/plugin.tar.gz"
path = "plugins/demo"
# Optional: sha256 = "<64 lowercase hex characters>"

[targets]
gemini = ["tools", "release"]
```

The lock records the normalized endpoint and logical package path. Git pins an
exact commit and package tree/manifest digests. Archives additionally pin the
SHA256 of the original downloaded bytes; their immutable revision is
`sha256:<original-byte-hash>`. The archive wrapper directory is not part of the
logical path, so changing a provider's versioned wrapper does not change ownership.

A moving branch/default Git requirement or archive endpoint can advance through
`update`/`deps update`. Exact commits and explicitly requested archive checksums
stay pinned until the manifest is intentionally edited. Locked sync, repair and
cold restore do not refresh refs or accept a new archive stream in place of the
recorded bytes. If an archive endpoint replaces old bytes and the old package is
not available in the local verified store, locked restore fails before mutation.
Keep versioned stable release URLs for durable archive restoration.

To pin a CLI archive download and optionally select a nested logical package:

```bash
agentplugins add 'https://downloads.example.com/plugins.zip#sha256=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef&path=plugins%2Fdemo' --target gemini
```

For a moving archive with an explicit nested path, use the `archive`/`path` manifest
table above. Without an explicit path, a root `plugin.json` takes precedence; one
nested package may be selected automatically. Multiple candidates produce a
bounded diagnostic requiring an explicit path. A malformed root manifest does
not fall back to a nested package. Archive selection reuses one retained download.

Different endpoints remain distinct sources even when their package trees match.
Original archive bytes retain their own checksum when wrappers or framing differ.
The shared content store holds bytes; selected lock provenance supplies ownership
and immutable source identity on every load. Historical GitHub and signed
Directory identities remain unchanged. Copy only `uap.toml` and `uap.lock` for a
portable restore, then use `install --locked` or `deps sync --locked --yes`.

## Credentials and archive policy

Direct Git uses native Git credential helpers, trusted user Git/SSH configuration,
SSH agent/key access and configured CA certificates. Provision credentials and
SSH known-host trust before invoking UAP. Native transport is bounded and
noninteractive; UAP does not silently accept an unknown SSH host, embed secrets in
portable files, allow a local-file/remote-helper rewrite, or disable TLS
verification. Repository checkout uses isolated Git configuration so package
attributes cannot run user filters/hooks. Signed Directory/Discovery acquisition
retains its separate anonymous immutable GitHub contract.

HTTPS archives are credential-free downloads from a stable query-free URL.
Bounded HTTPS redirects may use transient signed CDN queries; those URLs and query
tokens are neither persisted nor included in errors. Downgrades, redirect userinfo
and fragments are rejected, and headers/cookies are not carried to another hop.
An extensionless HTTPS input first attempts safe archive recognition; only a
nonarchive response can try native Git. Recognized but corrupt, oversized,
incompatible or unselectable archives never fall back to Git.

| Boundary | Supported policy |
| --- | --- |
| Formats | Ordinary single-disk ZIP (store/deflate, no ZIP64), TAR, one-member gzip TAR; `.tar.gz` and `.tgz` are equivalent |
| Download | At most 256 MiB retained original bytes; bounded two-minute acquisition |
| Expansion | At most 256 MiB file payload and at most 256 MiB decompressed TAR stream |
| Individual file | At most 64 MiB |
| Entries and paths | At most 10,000; TAR metadata and implicit parent paths are counted |
| Depth | At most 64 portable path segments |
| Members | Regular files/directories only; no links, sparse entries, devices, sockets, FIFOs, encrypted ZIP or multi-volume archives |
| Names | Portable NFC relative names, no traversal/absolute/drive/UNC/backslash/control names or case/Unicode/file-directory collisions; exactly one conventional leading `./` is accepted |
| Framing | Exact ZIP records/end/comment, two TAR zero end blocks with aligned zero padding, one checksum-validated gzip member; no archive preamble, undeclared trailer or concatenated archive |
| Filename | Recognized initial suffix must match detected format; extensionless URLs use bytes; redirect filenames and Content-Type do not override original format policy |

Acquisition never executes package code. The unchanged standard loader, existing
security assessment and each target client's supported-component checks still run
before installation. A successful transport fixture is not native agent runtime
certification or release qualification.


## Transport verification boundary

The `Source transports / native-ssh-linux` workflow provisions OpenSSH on
Ubuntu 24.04 and requires the real native Git SSH fixture and saved CLI SSH URL
and SCP lifecycles to execute. Missing SSH tooling, skipped fixtures or missing
expected test pass events fail the job. Disposable keys use exact known-host
verification; the fixture also refuses an untrusted host.

These loopback server fixtures require a Unix host: they provision Unix sshd
accounts, POSIX forced commands and Unix repository paths. They do not qualify
a Windows-hosted OpenSSH server or a Windows SSH client runtime. The required
Linux job fails on unsupported platform/tooling and cannot turn into a skip.
Windows source grammar and real TLS Git/archive lifecycle contracts remain
separate coverage; production SSH source admission is unchanged.

Real loopback TLS contracts exercise anonymous extensionless archive recognition,
private extensionless Git fallback through native credentials, and archives with
identical package trees but distinct original-byte checksums in both content-store
population orders. Agent responses in these source tests remain inert fixtures;
native agent runtime and project-scope qualification are separate gates.
