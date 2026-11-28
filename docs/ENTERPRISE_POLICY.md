# Installation policy

UAP installation policy lets a team control which sources, clients, scopes and
package components it accepts. Rules apply to UAP-managed installation and
restoration, including standalone Skills and dependency installations. A denied
operation explains the rule and the file that supplied it.

This guide describes the implementation in source. Use `agentplugins policy
--help` to check whether your installed CLI includes these commands. Source
availability is separate from a published CLI release.

## Start with a local example

Create a commented template in a working directory:

```bash
agentplugins policy init
agentplugins policy validate uap-policy.yaml
agentplugins policy status
```

`init` creates `uap-policy.yaml` without overwriting an existing file. Only
`schema_version: 2` is active initially; uncomment the restrictions you need.
Pass a path to create a candidate elsewhere. With an explicit project root and
no path argument, the template is created in that project. Add `--dry-run` to
inspect the destination without writing a file.

For example, allow project installations from your GitHub organization:

```yaml
schema_version: 2
sources:
  allowed_kinds: [git, directory]
  allow_repositories: ["github.com/acme/*"]
integrity:
  require_pinned_git: true
install:
  allowed_scopes: [project]
  allowed_targets: [codex, claude]
```

The `directory` kind means a package selected through the signed UAP Directory.
Its repository must also satisfy the repository rule. A local filesystem
directory has kind `local`.

An allowed repository is a source restriction, not a review of its contents.
Review package changes and lock changes through your normal process. Policy
permission does not grant native client trust or override UAP's integrity,
security, ownership or unsupported-component refusals.

## Preview before enabling a policy

Keep a candidate file outside the active locations and add it only to a check:

```bash
agentplugins policy validate ./candidate-policy.yaml
agentplugins policy check --policy ./candidate-policy.yaml \
  --scope project --project-root ./my-project
```

With no source argument, `policy check` verifies the selected environment's
manifest, lock, cached package bytes, declared dependency edges and policy for
all recorded targets. It is offline. Missing or invalid agreement/cache data
produces an incomplete, nonzero result; missing data is not treated as approval.
This form uses the recorded targets and therefore does not accept `--target`.

Inspect one package before installation with explicit targets:

```bash
agentplugins policy check ./my-plugin --policy ./candidate-policy.yaml \
  --scope project --project-root ./my-project --target codex,claude
```

This form may acquire package bytes into temporary snapshots or cache. Its
coverage is `package_only`. If the package declares dependencies, it returns an
incomplete, nonzero result because those dependencies have not been approved.
Use the no-argument check for an existing verified declared closure. Normal
installation performs its own full admission checks.

Neither check prepares client hosts, starts clients, grants trust, recovers
mutating journals or writes installation intent, receipts or lockfiles. A
successful check does not save authorization for a future command. Installation
checks the rules again.

`--policy` adds a restriction to the active layers. It cannot replace or weaken
an organization policy. `policy validate <path>` validates the specified file
independently, so it still works when an active policy needs repair.

## Policy locations and precedence

Every discovered layer applies. A lower layer can narrow permission but cannot
override a denial from another layer.

| Layer | Location | Required? |
| --- | --- | --- |
| Machine, Linux | `/etc/agentplugins/policy.yaml` | When present |
| Machine, macOS | `/Library/Application Support/AgentPlugins/policy.yaml` | When present |
| Machine, Windows | Registered system ProgramData directory, `AgentPlugins\policy.yaml` | When present |
| User | `<user-home>/.config/agentplugins/policy.yaml` | When present |
| Project | `<selected-project-root>/uap-policy.yaml` | When present |
| Explicit deployment | File named by `UAP_POLICY_FILE` | When the variable is configured |
| Candidate check | `policy check --policy <path>` | For that check |

For User-scope operations, the project layer is read from the invocation working
directory. UAP does not walk parent directories looking for policies. Use
`policy status` with the same scope and project root as the operation to see
exactly which paths are being checked, including absent files.

Machine policy is resolved independently of user `HOME`, XDG or UAP environment
settings. On Windows, UAP reads the protected machine ProgramData registration.
It preserves an administrator-configured absolute directory and resolves
`SystemDrive`, `SystemRoot` and `windir` tokens from the operating system's shared
Windows directory. Omitting or overriding `ProgramData`, `ALLUSERSPROFILE` or
`SystemDrive` in the caller's environment does not relocate machine policy.
An unreadable or invalid registration remains an error; it never means that
machine policy is absent.

An explicit file that is missing, an unreadable file, a symlink/nonregular file,
or an invalid policy blocks installation. During a reviewed operation, adding,
removing or changing policy rules requires a new review. Comment-only edits do
not change the policy decision. The decision is fixed at the final gate before
effects; transaction rollback and recovery retain their safety guarantees.

Present and explicitly required policy files must have no symbolic links in their
ancestor directories. On macOS, only the fixed `/var`, `/tmp` and `/etc` system
aliases use their `/private` locations; arbitrary user links are not resolved.
A missing automatic policy remains optional under a linked home or project
directory, while a dangling link at the policy filename is refused. Ancestry is
checked before and after the bounded file read; these checks do not isolate the
read from concurrent directory replacement.

Enterprise enforcement requires trusted filesystem ancestry: administrators must
protect the policy file and every containing directory against writes, deletion,
rename and replacement by the users governed by that policy. Use operating-system
ownership and ACLs to enforce this requirement. The checks above do not establish
that protection for an attacker-controlled directory tree.

Automatic machine, user and project discovery allows a location to be absent at
the start of a new operation. A reviewed operation still rejects disappearance,
including an ancestor redirect that makes its policy file appear missing. For a
managed invocation that must refuse a missing deployment file, set
`UAP_POLICY_FILE` to the protected file and protect the launch configuration that
supplies the variable. An environment variable users can freely unset does not
create mandatory organization authority.

The policy governs this UAP binary; it does not prevent a machine administrator
from replacing the binary or a user from manually editing client files.

### Deploy a machine policy

Prepare and validate a file first. An administrator can then copy it into the
platform location. For Linux, for example:

```bash
agentplugins policy validate ./company-policy.yaml
sudo install -d -m 0755 /etc/agentplugins
sudo install -m 0644 ./company-policy.yaml /etc/agentplugins/policy.yaml
agentplugins policy status --require-policy
```

On macOS, an administrator creates `/Library/Application Support/AgentPlugins`
and copies the validated file to `policy.yaml` there, preserving administrative
ownership and read access for users. On Windows, use device management or an
administrator account to place the file beneath the system ProgramData known
folder and ensure the directory's ACL does not grant ordinary users write
access. UAP does not elevate privileges or write machine policy for the user.

## Available rules

Policy files are strict YAML. Unknown fields, duplicate keys, nulls, aliases,
merge keys, multiple documents, unsupported versions and oversized documents
are rejected. Omit a rule to leave that dimension unrestricted. An explicitly
empty allow-list, such as `allowed_targets: []`, permits nothing on that
dimension. Deny-lists add refusals; an empty deny-list adds none.

| Rule | Meaning |
| --- | --- |
| `sources.allowed_kinds` | Allow `directory`, `git`, `archive`, `web`, or `local` sources |
| `sources.allow_hosts` | Exact allowed source hostnames |
| `sources.deny_hosts` | Exact denied source hostnames |
| `sources.allow_repositories` | Allowed host-qualified Git repositories or namespace prefixes ending in `/*` |
| `sources.deny_repositories` | Denied repositories or namespace prefixes |
| `integrity.require_pinned_git` | Require the requested Git selector to be an exact commit |
| `integrity.require_archive_sha256` | Require an archive checksum in the original request |
| `install.allowed_targets` | Allowed canonical target client IDs |
| `install.allowed_scopes` | Allowed `user` and/or `project` installation scope |
| `components.denied` | Refuse packages containing `mcp`, `skills`, `commands`, `apps`, or `extensions` |
| `mcp.allowed_transports` | Allowed `stdio`, `streamable-http`, and/or `sse` declarations |
| `mcp.allow_hosts` | Exact allowed hosts in remote MCP endpoint declarations |
| `mcp.deny_hosts` | Exact denied hosts in remote MCP endpoint declarations |

Repository rules are separate from source host rules. `github.com/acme/*` matches
repositories in that namespace at a slash boundary; it does not match
`github.com/acme-other/project`. Arbitrary regular expressions and globs are not
supported. Package subpaths do not change repository identity. GitHub owner and
repository names are matched case-insensitively; non-GitHub repository path
case is preserved. Manifest `repository` metadata does not establish source
identity.

Source host rules inspect the canonical source origin; they do not form a network
egress filter for acquisition redirects or subsequent connections.

Archive, web and local sources cannot satisfy a Git repository allow-list by
putting a repository-looking string in metadata. A local source has no remote
host, so it cannot satisfy a host allow-list. A host deny-list alone does not
forbid local sources; use `sources.allowed_kinds` when that is the desired rule.

Pin requirements distinguish requested integrity from a hash resolved during
download. A branch that happened to resolve to a commit is still unpinned intent.
The signed Directory already has separate immutable provenance. Local and web
Skills do not acquire Git pin requirements merely because their snapshots are
content-addressed.

Component restrictions conservatively cover the package inventory. UAP does not
silently discard forbidden components to pass policy. Existing loader refusals
for auto-discoverable hook directories and unsupported native hook declarations
remain in force. This feature is not a detector for every possible executable
behavior inside a package.

MCP endpoint rules concern declarations at installation time. They do not
restrict subsequent redirects, DNS resolution, outbound network access or
tool calls. A host allow-list applies to remote endpoints; set
`mcp.allowed_transports` as well if you want to forbid `stdio` commands. When
host rules are active, an endpoint must declare a hostname that UAP can
determine from its URL. Use a runtime gateway or sandbox for runtime enforcement.

## CI and troubleshooting

```bash
agentplugins policy status --require-policy --format json
agentplugins policy check --require-policy --format json \
  --scope project --project-root ./my-project
```

`--require-policy` requires at least one active restriction. A schema-2 template
with all examples commented out does not satisfy it. JSON output uses the
ordinary CLI result envelope and identifies coverage, active layers and rule
violations. Denied, invalid or incomplete results exit nonzero.

When an installation is denied, inspect the named file and rule with `policy
status`. Correct the source or ask the policy owner to change the restriction.
`--yes`, `--force` and `--accept-security-risk` do not override policy.

Removal, read/export operations and safe rollback remain available for packages
that policy now forbids. Tightening policy does not silently uninstall existing
packages. An update, repair, sync or new activation must pass the current rules.

Existing `deps check --policy <path>` remains available. Schema-1 policies keep
their previous behavior, including remote-source requirements even when no host
rule is written. New policy fields use schema 2; change the schema deliberately
when migrating a legacy file rather than relying on a silent semantics change.
