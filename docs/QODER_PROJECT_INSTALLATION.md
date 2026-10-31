# Qoder project skill bundle preparation

Explicit project preparation ships in CLI 0.1.93. The observations below do not
qualify arbitrary native runtime behavior.
Project installation prepares a selected skill bundle in the project's UAP
namespace. It preserves the original user's Qoder profile and does not register
a native plugin, rewrite Qoder's registry, redirect HOME or delete native caches.

## Files and supported components

UAP owns one managed package under:

`<project>/.uap/managed/clients/qoder/<physical-artifact-id>/`

Its importable native view is the exact subdirectory:

`<managed-package>/.agentplugins-qoder-native/`

That view contains a generated `.qoder-plugin/plugin.json` and only the selected
`skills/<name>/` trees, including their selected assets and eligible portable
commands rendered as skills. Explicit-only commands are withheld when Qoder
cannot enforce their invocation rule. MCP, hooks, agents and other native
resources are unsupported for this project route. Preserved source metadata
outside the native view does not become an importable native resource.

Results remain **Prepared**. Installed-file verification describes the exact
bundle and its managed receipt. It does not establish native registration,
selected skill loading, trust, authentication, MCP initialization or a running
session. Activation attestation cannot promote this route to native activation.

## Optional separate native import

The result includes the exact native view and selected project working directory.
An operator may review a separate native import from that project directory:

```text
qoder plugins install <exact-native-view> --scope project --json
```

UAP does not execute this command. Review existing native registrations and
cache contents first. The [preserved genuine Qoder1.1.78 observation](evidence/project-scope-research/qoder-1.1.78-linux-amd64-40135008690/README.md)
found project-attributed rows in a global listing, payloads in the original user's
shared cache and changes to the user's registry. Lists were identical across
user/A/B working directories. Those fixtures were non-Git directories and used
different versions; Git-root behavior, same-name/same-version cache behavior and
actual runtime skill selection remain unqualified.

Import only the exact native view. Importing the whole managed package would
cross a different resource boundary and is not the supported preparation path.

## Maintenance and ownership

UAP validates the selected project, exact managed locator and receipt before
update, repair or removal. It refuses copied authority, project/profile overlap,
aliases, foreign claims and inconsistent ownership. Repair uses the selected
immutable pin; updating to new source is a separate operation. Ordinary status
reads saved UAP state without probing Qoder.

Removal deletes only UAP's exact project bundle. A separately imported native
cached copy has its own native lifetime: UAP removal does not unregister or delete
it. Native cleanup is a separate operation requiring review of Qoder's native
scope and shared cache. No executable MCP/stdio registration into the bundle is
created by this skills-only route. Persistent caller-owned project data, other
projects and the original user profile remain separate authorities.

The existing user-scope Qoder adapter retains its native CLI lifecycle and
conservative global-list ownership rules. This project preparation does not
qualify unrestricted same-name user/project native coexistence.
