# Installing Agent Skills

The released `agentplugins` CLI includes `skills add` for public GitHub sources
and compatible HTTPS publishers. This guide is verified against
[CLI 0.1.110](https://github.com/777genius/universal-agent-plugins/releases/tag/agentplugins-v0.1.110).
Check `agentplugins version` before following release-specific guidance. The
historical `plugin-kit-ai` Skills commands remain documented in [SKILLS.md](./SKILLS.md).

**Development-source availability:** builds containing the generic Git and local
snapshot changes also accept private GitHub repositories, other HTTPS/SSH Git
sources and local Skill directories. These additions are not included in the
verified CLI 0.1.110 release. The examples labelled source build below apply to
such builds, including `main` after these changes merge; check release notes
before expecting them in a downloaded CLI.

A Skill can be installed from a public GitHub repository, a folder URL, or a
`SKILL.md` URL. The installer keeps its original document, scripts and resources,
then uses the existing package installer and ownership/recovery rules.
Installation does not execute the Skill's scripts or prepare arbitrary runtime
libraries. See [shared MCP ownership and locked Skill runtime preparation](SHARED_MCP_AND_SKILL_RUNTIME.md)
for explicit prerequisites and what the installed lock does (and does not) cover.

Compatible HTTPS publishers can also expose complete Skills through a
`.well-known` discovery index. Publisher installation requires the exact
Skill name and uses terminal review:

```sh
agentplugins skills add https://publisher.example --skill review --target codex
```

For unattended installation, add `--yes`; use `--dry-run` to review without
applying. Website sources do not use the repository browser or Git options
`--ref` and `--path`.

```sh
# Interactive browser: nested folders, search, basket, target selection and review.
agentplugins skills add owner/repository

# Exact folder or file URL: terminal review of that Skill.
agentplugins skills add https://github.com/owner/repository/tree/main/skills/review
agentplugins skills add https://github.com/owner/repository/blob/main/skills/review/SKILL.md

# Explicit unattended selection; duplicate names require an exact --path.
agentplugins skills add owner/repository --path skills/review --target codex --no-ui --yes --format json
agentplugins skills add owner/repository --skill review --skill pdf --target codex,claude --no-ui --yes

# Source build: a private GitHub repository uses your native Git credentials.
agentplugins skills add owner/private-repository --path skills/review --target codex --yes

# Source build: explicit HTTPS or SSH clone URL, ref and Skill directory.
agentplugins skills add 'https://git.example/team/skills.git#refs/heads/main//skills/review' --target codex --yes
agentplugins skills add 'ssh://git@git.example/team/skills.git#refs/heads/main//skills/review' --target claude --yes

# Source build: one local directory with a matching root SKILL.md.
agentplugins skills add ./skills/review --target codex --yes
```

Git sources in these source builds use the user's existing native Git credential
helper or SSH agent/configuration. Configure repository access in Git first;
the installer does not collect or save a separate repository password. Source
URLs must not contain access tokens, passwords or query-string credentials.
An SSH username such as `git@` is supported.

The clone URL syntax is `clone-url#ref//path`. Use an exact branch or tag name,
`refs/heads/main`, or a full lowercase commit SHA; the selected revision is
resolved and pinned before review. SCP-style sources such as
`git@git.example:team/skills.git#refs/heads/main//skills/review` also work.
For an HTTPS clone address without a `.git` suffix, use `git+https://` to
distinguish it from a publisher website. `--path skills/review` is an alternative
to the source's `//skills/review` suffix.

Bare repository input opens the bundled picker in an interactive terminal.
`--no-ui` selects the terminal flow. Explicit `--skill`, `--path`, `--target` and
exact file/folder URLs also use terminal review by default. `--ui` explicitly
opens browser selection; it rejects `--skill`, `--target` and `--dry-run` rather
than ignoring them. JSON/non-interactive input requires explicit selection and
targets; applying also requires `--yes`. `--dry-run` prepares a review without
applying it. Use `--ref` to disambiguate a GitHub URL whose branch name contains
slashes. Browser and terminal operate on the same resolved commit.

In the interactive `--no-ui` catalog, use the arrow keys to move, Space to
select Skills, `/` to search, and Enter to review. While searching, Enter applies
the filter. Escape or Ctrl+C cancels.

For the line-oriented catalog, add `--plain --no-ui`. Enter numbers to toggle
selections. Use `/query` to search names, paths and descriptions; your basket
stays selected between searches. `/` clears the filter, `clear` empties the basket, and `done`
continues to target selection and review. `q`, Escape or an empty line cancels.
The plain catalog shows at most 50 matches at once; narrow the query to find more.

Keep the CLI running while using its temporary local page. The capability URL
is private to this session. The page renders safe Markdown and offers the raw
instructions as text; it does not load remote images or execute source HTML.
Closing the tab after confirmation does not cancel an accepted installation.
Reloading the live session recovers its operation result. A lost apply response
never implies success: the page recovers the existing operation or offers an
explicit retry of the same confirmation.

## Install Skills for one project

In CLI 0.1.93, the terminal flow and browser picker use the same explicit
project environment selection as complete packages:

```sh
agentplugins skills add owner/repository --ui \
  --scope project --project-root /path/to/project
agentplugins skills add owner/repository --skill review --target codex --no-ui --yes \
  --scope project --project-root /path/to/project
```

The directory must already exist. The default is still `user`; with project
scope and no `--project-root`, the invocation directory is selected without a
parent-directory search. Only targets admitted for that scope can be selected in the picker. Unavailable
targets can remain visible with a reason. Its reviewed selection remains bound
to that project when applying; a stale
or changed environment requires a new review.

Saved Skill intent and pins live in the selected project's `uap.toml` and
`uap.lock`. Receipts, mutation locks, recovery and retained data remain local
under `.uap/`. Pass the same scope and project on later list, update, repair,
remove and dependency commands. Complete packages and Skills share the same
client ownership, trust and activation rules. See [Project installation](PROJECT_INSTALLATION.md)
and the [current client support table](PROJECT_CLIENT_SUPPORT.md).

## Discovery and validation

Search in the picker matches skill names, descriptions and repository paths.
Selection stays in the basket when you search or browse another folder. Open a
skill's title to preview it; its checkbox changes selection independently.
The review shows the exact destination before installation changes files.

A complete scan can include disabled skills. Discovery does not silently omit
invalid metadata: for example, the declared name must match the containing
folder, and descriptions must fit the standard's length limit. A popular
repository can contain templates or skills that fail these rules. The installer
does not rewrite their documents to make them pass; choose another valid skill
or a corrected source revision. A partial scan is labelled separately and does
not prove the repository contains no other matches.

Preview lists resources without executing their contents. A resource size may
be unknown until full acquisition for review/install; an unknown size is not an
empty file. Source builds using native Git fetch the complete repository commit
for authenticated discovery, so resource content may already be downloaded
before selection. Resource files and scripts are not executed by preview or install.

## Saved source and updates

Each selection becomes a normal dependency and uses existing target assignments:

```toml
schema_version = 1

[dependencies.review]
kind = "skill"
git = "https://github.com/acme/agent-skills.git"
skill = "review"
path = "skills/review"
ref = "refs/heads/main"

[targets]
codex = ["review"]
claude = ["review"]
```

`path` is the originally selected folder. A later move does not rewrite it.
The lock records the immutable commit, resolved Skill path, raw-source digest
and normalizer version separately from normalized package digests. An author's
`metadata.version` is descriptive; the commit and digests pin installed bytes.

Repeating `skills add` for an already saved source keeps its existing selector
and immutable pin. It does not follow a newer branch revision, refresh a website
index or adopt edits from a local directory. Use an explicit update to accept
new source bytes:

```sh
# Refresh and install the selected saved Skill through the normal update flow.
agentplugins update review

# Refresh a moving selector, then review/apply the saved environment.
agentplugins deps update review
agentplugins deps sync --locked --yes

# Restore exactly the lock; cold restore still needs the recorded source bytes.
agentplugins deps sync --locked --yes

# Repair a missing managed projection, or remove a saved dependency.
agentplugins repair review --target codex
agentplugins remove review
```

An update first checks the last resolved folder, then the original folder.
If neither contains the expected name, a complete repository scan must find
exactly one valid match. Missing, ambiguous or incomplete discovery blocks the
batch before application. Renames and moves to another repository require an
explicit dependency edit. Locked restore and repair do not search for a newer
version. Existing protections for foreign files, drift and pending recovery apply.
Native clients may require their normal uninstall/recovery steps; follow the
command's instructions rather than deleting agent configuration manually.

For local Skills in a source build, the selected directory must contain a regular
root `SKILL.md` whose name matches the directory, for example `review`. The
installer snapshots its accepted document, scripts and resources, then delivers
the same normalized immutable package used by remote Skills. It does not create
a live link or watch the source directory. Local selection uses terminal review;
`--ui`, `--ref` and `--path` do not apply. An optional `--skill` must match the
directory's name.

Saved local intent uses a canonical absolute source path, independently of the
working directory used by later commands:

```toml
[dependencies.review]
local = "/absolute/path/skills/review"
kind = "skill"
skill = "review"
```

The local lock records the raw-source digest and normalization version as well
as the normalized package digests. `update review` explicitly snapshots source
edits. Locked sync and repair prefer verified stored content, so they can work
after the original directory disappears. Without that content, they must find
the original directory and reproduce its exact recorded bytes; changed or missing
sources refuse instead of selecting a newer snapshot. A path from another OS
can be replayed from verified storage but cannot be reopened as a local source
on the current host. Local sources are root declarations and cannot become a
published package's dependencies.

`deps export` transfers `uap.toml` and `uap.lock`, not the stored package bytes.
Importing that agreement preserves the exact pin, but a fresh machine without
verified content or the original accessible source cannot restore a local Skill.
Keep the source available or recover the matching verified content before cold
restore. An explicitly supplied remote-host dependency policy refuses local
sources rather than treating them as an approved remote host.

## Website sources

The publisher must serve an index at
`/.well-known/agent-skills/index.json` or `/.well-known/skills/index.json`.
The supported discovery dialect supplies a `skills` array with each Skill's
`name` and complete `files` list. The installer downloads every listed resource
and validates the resulting Skill through the same normalization and package
installation pipeline. It does not scrape arbitrary web pages. Other discovery
formats, including proposed 0.2 artifact entries, are not supported yet.
Publisher downloads must use direct HTTPS addresses; this workflow refuses
redirects.

Website intent uses its own source field:

```toml
[dependencies.review]
website = "https://publisher.example"
kind = "skill"
skill = "review"

[targets]
codex = ["review"]
```

The lock preserves the resolved file addresses, each file's SHA256, discovery
provenance, raw tree digest and normalizer version. `deps update review` explicitly
discovers a newer version. Locked sync, repair and export/import restore the
recorded bytes without selecting a newer version from the index. If the old
files have changed or disappeared and no verified cache remains, restoration
fails; the publisher must retain those bytes for cold restore to succeed.

Website discovery remains an emerging convention. This compatibility does not
promise that all websites or every proposed discovery format can be installed.

## Building and checking the picker

From the repository root, use the pinned toolchain and package lock:

```sh
cd ui/skills-picker
npm ci
npm run bundle:go
cd ../..
go build -o /absolute/owned/output/agentplugins ./cli/cmd/agentplugins
```

Node is required only for development/build. The built Go executable embeds
local assets and serves its own loopback API. Plain Go builds without the UI
step contain a development placeholder and offer terminal selection.
Release builds must bundle production assets and pass the picker qualification.

The [OpenAPI contract](../contracts/skills-picker/README.md) generates Go and
TypeScript transport models. To check it and the critical UI scenarios:

```sh
node contracts/skills-picker/generate.mts --check
cd ui/skills-picker
npm run test:model
UAP_SKILLS_LIVE=1 npm test
```

The live browser test starts the production Go bridge with a new disposable Git
repository and test client profiles. It checks two-Skill/two-target installation,
recovery after a lost response, saved receipts and locked sync; it never uses
real agent profiles. Go integration tests cover relocation, cold restore,
partial revision repair, drift and removal. Existing project gates still apply.
