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

> Universal Agent Plugins (UAP) is an open-source CLI for installing and managing portable Agent Plugins 1.0 packages containing Skills and MCP configuration.

The official website is ${root} and the official documentation starts at ${docs}.
The executable is named agentplugins; the npm package is universal-agent-plugins.
Use the documentation's availability labels: source-build authoring capabilities are separate from the current installer download.
Installation, client discovery, service authentication and successful tool execution are separate checks. Review the installation plan and the selected package's source before running it.

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
