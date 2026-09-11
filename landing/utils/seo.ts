import { localeMetadata, publishedLocales, type KnownLocale } from '../data/i18n.ts';
import type { ProductRoute } from '../data/routes.ts';
import { expandLocalizedRoutes, normalizeAppBase, localizedPath } from './localizedRoutes.ts';

const DEFAULT_DESCRIPTION_LIMIT = 160;

export function canonicalPath(path: string): string {
  const normalized = `/${path.split(/[?#]/, 1)[0]}`.replace(/\/{2,}/g, '/').replace(/\/+$/, '');
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
  applicationSubCategory?: string;
  softwareRequirements?: string;
  featureList?: string;
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
    applicationSubCategory: options.applicationSubCategory ?? 'Agent plugin manager',
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
    softwareRequirements:
      options.softwareRequirements ??
      'Native CLI for macOS, Linux, and Windows; or Node.js 22+ for npx',
    codeRepository: options.githubUrl,
    screenshot: `${siteUrl}/og-image.png`,
    sameAs: [options.githubUrl, npmUrl],
    publisher: { '@id': `${siteUrl}/#organization` },
    license: 'https://www.apache.org/licenses/LICENSE-2.0',
    featureList:
      options.featureList ??
      'Install, inspect, update, repair, switch source, and remove Agent Plugins 1.0',
  };
}

/** siteUrl can contain the deployment path or just its origin. */
export function productRootUrl(siteUrl: string, base = '/'): string {
  const site = new URL(siteUrl);
  const sitePath = site.pathname === '/' ? normalizeAppBase(base) : normalizeAppBase(site.pathname);
  return `${site.origin}${sitePath}`;
}

export function productImageUrl(imageUrl: string, siteUrl: string, base = '/'): string {
  if (/^https?:\/\//i.test(imageUrl)) return imageUrl;
  if (imageUrl.startsWith('//')) return new URL(imageUrl, siteUrl).href;
  const root = productRootUrl(siteUrl, base);
  const rootPath = new URL(root).pathname;
  const relative =
    rootPath !== '/' && imageUrl.startsWith(rootPath)
      ? imageUrl.slice(rootPath.length)
      : imageUrl.replace(/^\/+/, '');
  return new URL(relative, root).href;
}

/** Resolve only real published HTML identities; query and hash never enter SEO. */
export function pageRouteSeo(
  path: string,
  routes: readonly ProductRoute[],
  siteUrl: string,
  base = '/',
  locales: readonly KnownLocale[] = publishedLocales,
) {
  const appBase = normalizeAppBase(base);
  let clean = canonicalPath(path);
  if (appBase !== '/' && clean.startsWith(appBase))
    clean = canonicalPath(clean.slice(appBase.length));
  const variants = expandLocalizedRoutes(routes, locales);
  const current = variants.find((route) => route.path === clean);
  const root = productRootUrl(siteUrl, base);
  const absolute = (routePath: string) => `${root}${routePath.replace(/^\/+/, '')}`;
  if (!current)
    return {
      canonical: undefined,
      alternates: [],
      ogLocale: undefined,
      ogAlternates: [],
      current: undefined,
    };
  const source = routes.find(
    (route) =>
      route.family === current.family && localizedPath(route.path, current.locale) === current.path,
  )!;
  const alternates = current.indexable
    ? [
        ...locales.map((locale) => ({
          hreflang: locale,
          href: absolute(localizedPath(source.path, locale)),
        })),
        { hreflang: 'x-default', href: absolute(source.path) },
      ]
    : [];
  return {
    current,
    canonical: current.family === 'community' ? undefined : absolute(current.path),
    alternates,
    ogLocale: localeMetadata[current.locale].iso.replace('-', '_'),
    ogAlternates: current.indexable
      ? locales
          .filter((locale) => locale !== current.locale)
          .map((locale) => localeMetadata[locale].iso.replace('-', '_'))
      : [],
  };
}

/** Nuxt's shared SSG fallback bypasses page setup, so mark its actual HTML noindex. */
export function noindexStaticFallback(html: string): string {
  const robots = /<meta\b[^>]*\bname\s*=\s*["']robots["'][^>]*>/gi;
  const marker = '<meta name="robots" content="noindex, follow">';
  if (robots.test(html)) return html.replace(robots, marker);
  if (!/<head(?:\s[^>]*)?>/i.test(html)) throw new Error('Static fallback has no HTML head');
  return html.replace(/<head(?:\s[^>]*)?>/i, (head) => `${head}${marker}`);
}
