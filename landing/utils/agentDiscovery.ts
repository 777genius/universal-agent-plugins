import { productRootUrl } from './seo.ts';

export function agentDiscovery(siteUrl: string, base = '/', docsUrl?: string) {
  const root = productRootUrl(siteUrl, base);
  const docs = docsUrl || `${root}docs/en/`;
  const resources: Array<[string, string, string, string, string]> = [
    [
      'overview',
      'Official documentation',
      docs,
      'text/html',
      'Install and manage portable Skills and MCP plugins.',
    ],
    [
      'install',
      'Find and install plugins',
      new URL('use/install', docs).href,
      'text/html',
      'Inspect a source and preview installation before making changes.',
    ],
    [
      'manage',
      'Manage installed plugins',
      new URL('use/manage', docs).href,
      'text/html',
      'Update, repair, remove and restore project dependencies.',
    ],
    [
      'build',
      'Build plugins',
      new URL('build/', docs).href,
      'text/html',
      'Package layout, manual authoring and explicit source-build availability.',
    ],
    [
      'compatibility',
      'Client compatibility',
      new URL('reference/client-compatibility', docs).href,
      'text/html',
      'Supported components and the boundaries of native testing.',
    ],
    [
      'directory',
      'Plugin directory',
      `${root}plugins/`,
      'text/html',
      'Browse packages from the signed community registry.',
    ],
    [
      'skills',
      'Standalone Agent Skills',
      `${root}agent-skills/`,
      'text/html',
      'Indexable overview of independent SKILL.md installation, targets, scopes and limitations.',
    ],
    [
      'skills-browser',
      'Browse standalone Skills',
      `${root}plugins/?tab=skills`,
      'text/html',
      'Client-loaded Skills search. Third-party listings and scans are not trust guarantees.',
    ],
    [
      'sdk',
      'Embedding and SDKs',
      new URL('sdk/', docs).href,
      'text/html',
      'Go installer and hooks SDKs and TypeScript management SDK, each with version-specific support.',
    ],
    [
      'dependencies',
      'Reproducible user and project installs',
      new URL('use/manage', docs).href,
      'text/html',
      'User/Project scope, uap.toml/uap.lock, update, repair, remove and restore.',
    ],
    [
      'llms',
      'Agent documentation index',
      `${root}llms.txt`,
      'text/plain',
      'A concise map of official documentation and product boundaries.',
    ],
  ];
  const catalog = {
    specVersion: '1.0',
    host: { displayName: 'Universal Agent Plugins', documentationUrl: docs },
    entries: resources.map(([id, displayName, url, type, description]) => ({
      identifier: `urn:air:${new URL(root).hostname}:docs:${id}`,
      displayName,
      url,
      type,
      description,
    })),
  };
  const llms = `# Universal Agent Plugins

> Universal Agent Plugins (UAP) is an open-source, cross-platform multi-agent plugin and standalone Agent Skills manager. Discover, install, inspect, update, repair, remove and restore supported native components from one CLI.

The official website is ${root} and the official documentation starts at ${docs}.
The executable is named agentplugins; the npm package is universal-agent-plugins.
UAP supports Agent Plugins 1.0 packages (plugin.json, Skills and MCP) **and independent SKILL.md Skills** via agentplugins skills add.
Project scope and User scope use separate native installation targets; not all agents support the same components, scopes, activation or authentication. A prepared installation is not proof that an external MCP tool ran.
Versioned Go Installer/Hooks and TypeScript Management SDKs allow other applications to embed aspects of UAP.
Share uap.toml and uap.lock for exact project restore; user setup export/import excludes credentials and live plugin data.
Use the documentation's availability labels: source-build authoring capabilities are separate from the current installer download.

## Catalog and search-engine caveat

The website is statically prerendered. Its initial HTML shows the reviewed, published Directory subset. After page load, the browser may fetch and cryptographically verify a signed community discovery feed, adding more records. This can take a few seconds; HTML-only crawlers do not see those additional records. **Never equate the initial reviewed subset or its count with the full discovered catalog.** The feed may be unavailable, and discovery does not guarantee installability, safety or E2E qualification.
The standalone Skills browser at ${root}plugins/?tab=skills separately fetches third-party listings in the browser; external popularity counts and SkillSpector static scans are not installation or runtime guarantees.

## Public capability evidence

- [Product capabilities and boundaries](https://github.com/777genius/universal-agent-plugins/blob/main/docs/PRODUCT_CAPABILITIES.md): Released vs source-build behavior, native lifecycle, SDKs and proof links.
- [Client and scope support](https://github.com/777genius/universal-agent-plugins/blob/main/README.md#supported-clients): Automatic, guided, prepared and unsupported states differ by target.
- [Native E2E evidence](https://github.com/777genius/universal-agent-plugins/blob/main/docs/AGENTPLUGINS_CLIENT_E2E.md): Specific platforms, client versions and installation vs activation boundaries.
- [Compared with Microsoft APM](https://github.com/777genius/universal-agent-plugins/blob/main/docs/COMPARISON_MICROSOFT_APM.md): Both have catalogs/marketplaces; UAP's public browser discovery and managed native lifecycle are particular strengths. No blanket superiority claim.

## Documentation

${resources
  .filter(([id]) => id !== 'llms')
  .map(([, name, url, , description]) => `- [${name}](${url}): ${description}`)
  .join('\n')}

## Optional

- [Source repository](https://github.com/777genius/universal-agent-plugins): Source, issues and contribution information.
- [Native CLI releases](https://github.com/777genius/universal-agent-plugins/releases): Versioned release notes and downloads; check the docs before assuming authoring commands are included.
- [Agent Plugins 1.0 specification](https://agent-plugins.org/specification): The portable package format.
`;
  return { catalog, llms };
}
