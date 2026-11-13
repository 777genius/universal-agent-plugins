# Antigravity native admission after prepared delivery

Baseline: main `c94c8c752ea23a6dcb89dec9a952bd7520ee841c`.

## Actual pinned admission

The digest-verified Linux agy 1.5.3 was executed locally without account login,
provider credentials or a model request. Its archive SHA-256 is
`1b4850599cce73a17f94ffb422df3bf9531531b77dbddc40c1a435e0c2c16807`.
All HOME, USERPROFILE, APPDATA, LOCALAPPDATA and XDG configuration/cache paths
were private disposable directories.

- Strict local validation succeeds.
- A directly prepared User plugin appears in `plugin list --json`, including
  its exact native path, version, component names and boolean enabled state.
  No vendor import is needed. Its source field can be absent.
- Inventory names are physical directory basenames, not manifest names. A
  renamed native directory is listed and enabled by its new basename.
- Direct Project `.agents/plugins` packages are absent from this CLI inventory;
  enable/disable of the Project identity fails without effects.
- `plugin install PROJECT_PATH` copies the package into the global User plugin
  root and writes import_manifest.json. It is not a Project activation route.
- Enable/disable writes the shared config.json `plugins[FOLDER].enabled` member.
  This requires separate consent and ownership-aware recovery before production
  mutation; ordinary installation confirmation does not authorize it.

## Bounded change and acceptance

Strengthen the existing genuine native lifecycle oracle to verify direct UAP
User inventory by exact physical path/basename, manifest version and selected
Skill/MCP names. Verify that inventory reads preserve profile/package bytes.
Exercise vendor enable/disable only in the already disposable imported-plugin
profile, verifying readback and source preservation. Keep product Prepared and
do not add a global registration fallback for Project scope.

The standalone IDE remains a separate required observation: open the selected
workspace and inspect its agent-side-panel Customizations dropdown. A CLI
inventory response does not establish IDE loading or a successful MCP call.

## Critique rounds

1. **Scope:** CLI install has actual global-copy effects. Correction: no Project
   registration or enable fallback; assert Project stays absent from CLI User
   inventory before the deliberately separate vendor import.
2. **Identity:** a name-only inventory decoder loses UAP's physical artifact
   identity. Correction: require exact path/basename and declared manifest
   version/components; allow an absent source on direct directory discovery.
3. **Consent and recovery:** enable changes shared config. Correction: retain
   production behavior and exercise toggles only in an isolated vendor fixture.
   Enabled inventory is an observation, not runtime/IDE activation attestation.

References: https://antigravity.google/docs/plugins/ and
https://antigravity.google/docs/customization-discovery/ . Current documentation
is supplementary; pinned executable observations determine admitted behavior.
