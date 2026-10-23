# Installing Agent Skills from GitHub

This guide describes the source implementation in this change. It does not claim
that these commands are available in an already published installer release.
The existing historical authoring and external Skills wrapper are documented in
[SKILLS.md](./SKILLS.md); their source and support boundaries remain preserved.

A Skill can be installed from a public GitHub repository, a folder URL, or a
`SKILL.md` URL. The installer keeps its original document, scripts and resources,
then uses the existing package installer and ownership/recovery rules.
Installation does not execute the Skill's scripts.

```sh
# Interactive browser: nested folders, search, basket, target selection and review.
agentplugins skills add owner/repository

# Exact folder or file URL: terminal review of that Skill.
agentplugins skills add https://github.com/owner/repository/tree/main/skills/review
agentplugins skills add https://github.com/owner/repository/blob/main/skills/review/SKILL.md

# Explicit unattended selection; duplicate names require an exact --path.
agentplugins skills add owner/repository --path skills/review --target codex --no-ui --yes --format json
agentplugins skills add owner/repository --skill review --skill pdf --target codex,claude --no-ui --yes
```

Bare repository input opens the bundled picker in an interactive terminal.
`--no-ui` selects the terminal flow. Explicit `--skill`, `--path`, `--target` and
exact file/folder URLs also use terminal review by default. `--ui` explicitly
opens browser selection; it rejects `--skill`, `--target` and `--dry-run` rather
than ignoring them. JSON/non-interactive input requires explicit selection and
targets; applying also requires `--yes`. `--dry-run` prepares a review without
applying it. Use `--ref` to disambiguate a GitHub URL whose branch name contains
slashes. Browser and terminal operate on the same resolved commit.

Keep the CLI running while using its temporary local page. The capability URL
is private to this session. The page renders safe Markdown and offers the raw
instructions as text; it does not load remote images or execute source HTML.
Closing the tab after confirmation does not cancel an accepted installation.
Reloading the live session recovers its operation result. A lost apply response
never implies success: the page recovers the existing operation or offers an
explicit retry of the same confirmation.

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

```sh
# Refresh a moving selector, then review/apply the saved environment.
agentplugins deps update review
agentplugins deps sync --locked --yes

# Restore exactly the lock, including when the package store is empty.
agentplugins deps sync --locked --yes

# Explicitly repair managed drift, or remove a saved dependency.
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
