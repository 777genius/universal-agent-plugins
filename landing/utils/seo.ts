const DEFAULT_DESCRIPTION_LIMIT = 160;

export function canonicalPath(path: string): string {
  const normalized = `/${path}`.replace(/\/{2,}/g, '/').replace(/\/+$/, '');
  return normalized === '' ? '/' : `${normalized}/`;
}

export function seoDescription(
  parts: Array<string | undefined>,
  limit = DEFAULT_DESCRIPTION_LIMIT,
) {
  const value = parts
    .filter((part): part is string => Boolean(part))
    .join(' ')
    .replace(/\s+/g, ' ')
    .trim();
  if (value.length <= limit) return value;

  const candidate = value
    .slice(0, limit - 1)
    .replace(/\s+\S*$/, '')
    .replace(/[,:;.!?]+$/, '');
  return `${candidate || value.slice(0, limit - 1)}…`;
}

export function spdxLicenseUrl(license: string): string | undefined {
  if (!/^[A-Za-z0-9.+-]+$/.test(license)) return undefined;
  return `https://spdx.org/licenses/${license}.html`;
}

type ProductSoftwareSchemaOptions = {
  siteUrl: string;
  githubUrl: string;
  releasesUrl: string;
  docsUrl: string;
  description: string;
};

export function productSoftwareSchema(options: ProductSoftwareSchemaOptions) {
  const siteUrl = options.siteUrl.replace(/\/+$/, '');
  const npmUrl = 'https://www.npmjs.com/package/universal-agent-plugins';
  return {
    '@type': 'SoftwareApplication',
    '@id': `${siteUrl}/#software`,
    name: 'Universal Agent Plugins',
    alternateName: 'UAP',
    url: `${siteUrl}/`,
    description: options.description,
    applicationCategory: 'DeveloperApplication',
    applicationSubCategory: 'Agent plugin manager',
    operatingSystem: 'macOS, Linux, Windows',
    isAccessibleForFree: true,
    offers: {
      '@type': 'Offer',
      price: '0',
      priceCurrency: 'USD',
    },
    downloadUrl: options.releasesUrl,
    installUrl: npmUrl,
    softwareHelp: options.docsUrl,
    softwareRequirements: 'Native CLI for macOS, Linux, and Windows; or Node.js 22+ for npx',
    codeRepository: options.githubUrl,
    screenshot: `${siteUrl}/og-image.png`,
    sameAs: [options.githubUrl, npmUrl],
    publisher: { '@id': `${siteUrl}/#organization` },
    license: 'https://www.apache.org/licenses/LICENSE-2.0',
    featureList: 'Install, inspect, update, repair, switch source, and remove Agent Plugins 1.0',
  };
}
