# marketing-kit

A small, standard Agent Plugins package containing the `campaign-planning`
skill. Its UAP dependency manifest brings in the existing Notion, HubSpot CRM
and Playwright packages; it does not duplicate their MCP configuration.

## What it does

Turn a product brief and existing workspace context into a campaign plan,
content calendar, draft copy and a landing-page review. Notion provides brand
and content context, the workflow reads HubSpot CRM context, and Playwright
checks the landing page. Account authorization remains a separate step.

## Install the published package

After the Directory listing is published, add this dependency to the selected
user environment's `uap.toml` (merge it with existing declarations):

```toml
schema_version = 1

[dependencies]
"777genius/marketing-kit" = "0.1.0"

[targets]
codex = ["777genius/marketing-kit"]
```

Run `agentplugins install` with CLI 0.1.91 or later. Review the dependency plan
before confirming it. This installs the full dependency closure, including MCP
packages; `plugin.json` alone contains only the campaign-planning skill.
The TOML dependency contract belongs to UAP, not the portable Agent Plugins
standard. Directory publication is separate from this source package.

CLI 0.1.91 also supports `agentplugins add marketing-kit --target codex` after
Directory publication. Project scope still requires a compatible source build
until its CLI release. Project users keep `uap.toml`
and `uap.lock` in version control and collaborators restore them with
`agentplugins install --locked --scope project`. Account login, native
activation and workspace trust remain separate steps.

## Dependency pins

The dependencies select exact SemVer versions of canonical Directory
distributions: `777genius/notion` 0.1.0, `777genius/hubspot-crm` 0.1.0 and
`777genius/playwright-bridge` 0.0.80. The last distribution's plugin manifest
name is `playwright`. An installing environment creates its own `uap.lock`
with exact publications and content digests; this package does not dictate
the user's targets. No credentials or machine-specific paths are included.

These are package versions. The remote Notion and HubSpot services can evolve
independently; their server implementations are not frozen by this manifest.

## Connect after installation

The agent can help configure the client and start its supported login flow.
The user completes Notion and HubSpot OAuth login and permission consent.
HubSpot may first require creating an MCP connector in the selected account.
Playwright has no package-level account login; websites it opens may require
their own login. Never put account credentials in this package.

Picking up installed tools depends on the client: reconnecting or reloading
MCP may work in the current session, while other clients require a new session.
Installation alone does not promise that an already-running agent can see or
use the tools immediately. Follow the installer's activation instructions.

Figma is intentionally left to a design-focused kit. Context7 is not required
for this campaign-planning workflow. The skill contains no executable scripts,
install hooks or automatic publication.
