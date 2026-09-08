import { checkI18n } from './check-i18n.mjs';
import fs from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';
import { fileURLToPath } from 'node:url';
import { publishedLocales, localeMetadata } from '../data/i18n.ts';
import { productRoutes } from '../data/routes.ts';
import { clientLandingPages } from '../data/clients.ts';
import { expandLocalizedRoutes, normalizeAppBase } from '../utils/localizedRoutes.ts';
import { pageRouteSeo, productRootUrl } from '../utils/seo.ts';
import { parseDirectoryData } from '../utils/registry.ts';
import { projectRegistry } from '../utils/registryProjection.ts';

/** Check owned page-schema identities; external repositories/authors remain external. */
export function checkPageSchemaUrls(graphs, route, siteUrl, base) {
  const root = productRootUrl(siteUrl, base);
  const url = (value) => root + value.replace(/^\/+/, '');
  const canonical = url(route.path);
  const localized = (value) => url(`${route.locale === 'en' ? '' : `/${route.locale}`}${value}`);
  const node = (type) => {
    const matches = graphs.filter((entry) => entry['@type'] === type);
    assert.equal(matches.length, 1, `${route.path}: ${type} count`);
    return matches[0];
  };
  if (['home', 'download'].includes(route.family)) {
    const software = node('SoftwareApplication');
    assert.equal(software['@id'], `${root}#software`);
    assert.equal(software.url, root);
    assert.equal(software.publisher?.['@id'], `${root}#organization`);
    assert.equal(software.screenshot, `${root}og-image.png`);
    if (route.family === 'home') assert.equal(node('FAQPage')['@id'], `${canonical}#faq`);
  }
  if (route.family === 'plugin') {
    const plugin = node('SoftwareSourceCode');
    assert.equal(plugin.url, canonical);
    assert.equal(plugin['@id'], `${canonical}#plugin`);
    assert.equal(plugin.isPartOf?.['@id'], `${localized('/plugins/')}#webpage`);
  }
  if (['catalog', 'agent'].includes(route.family)) {
    const list = node('ItemList');
    assert.equal(list['@id'], `${canonical}#plugin-list`);
    for (const entry of list.itemListElement) {
      assert.equal(typeof entry.url, 'string');
      const prefix = localized('/plugins/');
      assert(
        entry.url.startsWith(prefix),
        `Schema plugin URL outside localized base: ${entry.url}`,
      );
      assert(/^[^/]+\/$/.test(entry.url.slice(prefix.length)), `Schema plugin path: ${entry.url}`);
    }
  }
  if (['plugin', 'agent'].includes(route.family)) {
    const breadcrumb = node('BreadcrumbList');
    assert.equal(breadcrumb['@id'], `${canonical}#breadcrumb`);
    assert.deepEqual(
      breadcrumb.itemListElement.map((entry) => entry.item),
      [localized(route.family === 'plugin' ? '/plugins/' : '/'), canonical],
    );
  }
}

const landing = fileURLToPath(new URL('../', import.meta.url));
const repo = path.resolve(landing, '..');
const decode = (value) =>
  value
    .replace(/&amp;/g, '&')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>');
const attrs = (tag) =>
  Object.fromEntries(
    [...tag.matchAll(/([^\s=<>]+)\s*=\s*["']([^"']*)["']/g)].map((match) => [
      match[1],
      decode(match[2]),
    ]),
  );
const tags = (html, name) =>
  [...html.matchAll(new RegExp(`<${name}\\b[^>]*>`, 'gi'))].map((match) => attrs(match[0]));
async function files(root, prefix = '') {
  const entries = await fs.readdir(path.join(root, prefix), { withFileTypes: true });
  return (
    await Promise.all(
      entries.map((entry) =>
        entry.isDirectory()
          ? files(root, path.join(prefix, entry.name))
          : [path.join(prefix, entry.name)],
      ),
    )
  ).flat();
}

/** Validate references using manifest families and assembled asset/docs namespaces.
 * Sibling sites are path-scoped exceptions, never an origin-wide bypass.
 */
export async function checkHtmlReferences({
  html,
  routePath,
  root,
  fileSet,
  routes,
  base,
  siteUrl,
  allowedSiblingSites = [],
}) {
  const appBase = normalizeAppBase(base);
  const site = new URL(siteUrl);
  for (const value of allowedSiblingSites) {
    assert(new URL(value).pathname !== '/', `Invalid sibling scope: ${value}`);
  }
  const namespaces = new Set([
    'docs',
    'og-image.png',
    '_nuxt',
    'api',
    'registry',
    'discovery',
    'security',
    ...routes.map((route) => route.path.split('/').filter(Boolean)[0]).filter(Boolean),
    ...[...fileSet].filter((file) => !file.endsWith('.html')).map((file) => file.split('/')[0]),
  ]);
  const owned = (local) =>
    !local ||
    namespaces.has(local.split('/')[0]) ||
    fileSet.has(local) ||
    fileSet.has(`${local.replace(/\/$/, '')}/index.html`);
  const references = [];
  for (const name of ['a', 'link', 'script', 'img', 'source']) {
    for (const tag of tags(html, name)) {
      if (tag.href || tag.src)
        references.push({
          href: tag.href || tag.src,
          asset:
            ['img', 'source', 'script'].includes(name) ||
            (name === 'link' && /^(stylesheet|preload|modulepreload|icon)$/.test(tag.rel || '')),
        });
      // Consume URL then descriptors (HTML srcset parsing); data URLs may contain commas.
      let candidates = tag.srcset || '';
      while (candidates.trim()) {
        candidates = candidates.replace(/^[\s,]+/, '');
        const match = candidates.match(/^\S+/);
        if (!match) break;
        const token = match[0];
        candidates = candidates.slice(token.length);
        references.push({ href: token.replace(/,+$/, ''), asset: true });
        if (!token.endsWith(',')) {
          const separator = candidates.indexOf(',');
          candidates = separator < 0 ? '' : candidates.slice(separator + 1);
        }
      }
    }
  }
  for (const tag of tags(html, 'meta')) {
    if (tag.property === 'og:image' || tag.name === 'twitter:image')
      references.push({ href: tag.content, customImage: true });
  }
  for (const { href, customImage, asset } of references) {
    if (!href || /^(mailto:|tel:|data:|javascript:|blob:)/i.test(href)) continue;
    const target = new URL(href, `${site.origin}${appBase}${routePath.replace(/^\/+/, '')}`);
    if (target.origin !== site.origin) continue;
    const pathname = decodeURIComponent(target.pathname);
    const unbased = pathname.replace(/^\/+/, '');
    const inside = appBase === '/' || pathname.startsWith(appBase);
    // Explicit absolute custom social images may intentionally live outside this product.
    if (
      customImage &&
      /^https?:\/\//i.test(href) &&
      !owned(inside ? pathname.slice(appBase.length) : unbased)
    )
      continue;
    const sibling = allowedSiblingSites.some((value) => {
      const url = new URL(value);
      return (
        url.origin === target.origin && target.pathname.startsWith(normalizeAppBase(url.pathname))
      );
    });
    if (!inside) {
      assert(
        !owned(unbased) && (!asset || sibling),
        `Missing app base in owned reference: ${href}`,
      );
      continue;
    }
    let local = pathname.slice(appBase.length);
    const landingRoute = routes.some((route) => route.path === `/${local.replace(/\/+$/, '')}/`);
    if (!asset && landingRoute) assert(pathname.endsWith('/'), `Missing trailing slash in landing reference: ${href}`);
    if (sibling && !owned(local)) continue;
    if (appBase === '/' && !owned(local) && !asset) continue;
    if (!local || local.endsWith('/')) local += 'index.html';
    if (!fileSet.has(local) && fileSet.has(`${local}/index.html`)) local += '/index.html';
    assert(fileSet.has(local), `Missing final artifact file: ${local}`);
    if (local.endsWith('.html') && target.hash) {
      const document = await fs.readFile(path.join(root, local), 'utf8');
      const anchor = decodeURIComponent(target.hash.slice(1)).split(':~:text=')[0];
      if (!anchor) continue;
      assert(
        tags(document, '[a-zA-Z][a-zA-Z0-9]*').some(
          (tag) => tag.id === anchor || tag.name === anchor,
        ),
        `Missing ${local.startsWith('docs/') ? 'docs' : 'landing'} anchor: ${href}`,
      );
    }
  }
}

export async function checkPagesArtifact({
  root = path.join(repo, '.pages-dist'),
  mirror = process.env.UAP_VERIFIED_MIRROR,
  base = process.env.NUXT_APP_BASE_URL || '/universal-agent-plugins/',
  siteUrl = process.env.NUXT_PUBLIC_SITE_URL ||
    'https://777genius.github.io/universal-agent-plugins',
  docsDist = path.join(repo, 'website/dist'),
  // Explicit fixture injection only; CLI always uses the production published set.
  locales = publishedLocales,
  allowedSiblingSites = [],
} = {}) {
  assert(mirror, 'UAP_VERIFIED_MIRROR must identify the verified mirror used by this build');
  const read = (relative) => fs.readFile(path.join(root, relative));
  const json = async (relative) => JSON.parse(await read(relative));
  const pointer = JSON.parse(
    await fs.readFile(path.join(mirror, 'registry/schemas/1/latest.json'), 'utf8'),
  );
  const snapshot = JSON.parse(
    await fs.readFile(path.join(mirror, 'registry/schemas/1', pointer.snapshot_path), 'utf8'),
  );
  const registry = parseDirectoryData(snapshot, 'published_snapshot');
  const routes = productRoutes(
    registry.plugins.map((plugin) => plugin.name),
    clientLandingPages.map((client) => client.slug),
  );
  const expanded = expandLocalizedRoutes(routes, locales);
  const artifactFiles = await files(root);
  const fileSet = new Set(artifactFiles);
  const requireFile = (relative) =>
    assert(fileSet.has(relative), `Missing final artifact file: ${relative}`);
  requireFile('.nojekyll');
  requireFile('404.html');
  requireFile('robots.txt');
  requireFile('docs/index.html');
  requireFile('docs/sitemap.xml');
  // Compare every byte of both assembled inputs, including signed metadata/icons and docs aliases.
  for (const source of [mirror, docsDist]) {
    for (const relative of await files(source)) {
      const target = source === mirror ? relative : `docs/${relative}`;
      assert(
        (await read(target)).equals(await fs.readFile(path.join(source, relative))),
        `Assembly changed bytes: ${target}`,
      );
    }
  }
  const expectedHtml = new Set(expanded.map((route) => `${route.path.slice(1)}index.html`));
  for (const relative of artifactFiles.filter(
    (file) => file.endsWith('.html') && !file.startsWith('docs/'),
  )) {
    if (['404.html', '200.html'].includes(relative)) continue;
    assert(expectedHtml.has(relative), `Unexpected/draft/fake HTML: ${relative}`);
  }
  const sitemap = (await read('sitemap.xml')).toString();
  const locations = [...sitemap.matchAll(/<loc>([^<]+)<\/loc>/g)].map((match) => decode(match[1]));
  const expectedLocations = [];
  for (const route of expanded) {
    const relative = `${route.path.slice(1)}index.html`;
    requireFile(relative);
    const raw = await read(relative);
    const html = raw.toString();
    const payloadPath = `${route.path.slice(1)}_payload.json`;
    requireFile(payloadPath);
    const payloadText = (await read(payloadPath)).toString();
    const payload = JSON.parse(payloadText);
    assert(Array.isArray(payload), `Invalid Nuxt payload: ${payloadPath}`);
    assert(
      !payloadText.includes('registryIndex'),
      `Full runtime registry in payload: ${payloadPath}`,
    );
    let projection = { kind: 'empty' };
    if (route.family === 'home' || route.family === 'catalog') projection = { kind: 'catalog' };
    if (route.family === 'plugin')
      projection = { kind: 'plugin', value: route.path.split('/').filter(Boolean).at(-1) };
    if (route.family === 'agent')
      projection = {
        kind: 'client',
        value: clientLandingPages.find((client) => route.path.endsWith(`/agents/${client.slug}/`))
          .id,
      };
    const allowedDescriptions = new Set(
      projectRegistry(registry, projection).plugins.map((plugin) => plugin.description),
    );
    const strings = new Set(payload.filter((value) => typeof value === 'string'));
    for (const plugin of registry.plugins) {
      if (!allowedDescriptions.has(plugin.description))
        assert(
          !strings.has(plugin.description),
          `Unrelated registry description in ${payloadPath}: ${plugin.name}`,
        );
    }
    if (route.family === 'download')
      assert(!payloadText.includes('registry-page:'), 'Download must not initialize registry');
    assert.equal(tags(html, 'html')[0]?.lang, route.locale, `HTML lang: ${relative}`);
    const seo = pageRouteSeo(route.path, routes, siteUrl, base, locales);
    const links = tags(html, 'link');
    assert.deepEqual(
      links.filter((link) => link.rel === 'canonical').map((link) => link.href),
      seo.canonical ? [seo.canonical] : [],
      `Canonical: ${relative}`,
    );
    assert.deepEqual(
      links
        .filter((link) => link.rel === 'alternate' && link.hreflang)
        .map(({ hreflang, href }) => ({ hreflang, href }))
        .sort((a, b) => a.hreflang.localeCompare(b.hreflang)),
      [...seo.alternates].sort((a, b) => a.hreflang.localeCompare(b.hreflang)),
      `Alternates: ${relative}`,
    );
    const meta = tags(html, 'meta');
    assert.equal(
      meta.find((tag) => tag.property === 'og:locale')?.content,
      localeMetadata[route.locale].iso.replace('-', '_'),
    );
    assert.deepEqual(
      meta
        .filter((tag) => tag.property === 'og:locale:alternate')
        .map((tag) => tag.content)
        .sort(),
      seo.ogAlternates.sort(),
    );
    const robots = meta.find((tag) => tag.name === 'robots')?.content || '';
    assert.equal(/\bnoindex\b/.test(robots), !route.indexable, `Robots: ${relative}`);
    const graphs = [
      ...html.matchAll(
        /<script\b[^>]*type=["']application\/ld\+json["'][^>]*>([\s\S]*?)<\/script>/g,
      ),
    ].flatMap((match) => JSON.parse(match[1])['@graph'] || []);
    const pages = graphs.filter((graph) => ['WebPage', 'CollectionPage'].includes(graph['@type']));
    if (route.family === 'community') assert.equal(pages.length, 0);
    else {
      assert.equal(pages.length, 1);
      assert.equal(pages[0].url, seo.canonical);
      assert.equal(pages[0].inLanguage, route.locale);
    }
    checkPageSchemaUrls(graphs, route, siteUrl, base);
    if (route.indexable) expectedLocations.push(seo.canonical);
    if (['plugin', 'download'].includes(route.family)) {
      assert(raw.length < 450000, `HTML payload budget: ${relative} (${raw.length})`);
      assert(!html.includes('registryIndex'), `Full registry payload: ${relative}`);
    }
    await checkHtmlReferences({
      html,
      routePath: route.path,
      root,
      fileSet,
      routes: expanded,
      base,
      siteUrl,
      allowedSiblingSites,
    });
  }

  assert.deepEqual(
    locations.sort(),
    expectedLocations.sort(),
    'Sitemap must exactly match indexable manifest',
  );
  for (const entry of [...sitemap.matchAll(/<url>([\s\S]*?)<\/url>/g)]) {
    const loc = decode(entry[1].match(/<loc>([^<]+)<\/loc>/)[1]);
    const route = expanded.find(
      (route) => pageRouteSeo(route.path, routes, siteUrl, base, locales).canonical === loc,
    );
    const expected = pageRouteSeo(route.path, routes, siteUrl, base, locales).alternates;
    assert.deepEqual(
      tags(entry[1], 'xhtml:link').map(({ hreflang, href }) => ({ hreflang, href })),
      expected,
    );
  }
  const apiRoutes = [
    'catalog',
    'empty',
    ...clientLandingPages.map((client) => `client/${client.id}`),
    ...registry.plugins.map((plugin) => `plugin/${plugin.name}`),
  ];
  for (const endpoint of apiRoutes) {
    const data = await json(`api/registry/${endpoint}`);
    assert(Array.isArray(data.plugins), `Invalid projection: ${endpoint}`);
    const [kind, value] = endpoint.split('/');
    assert.deepEqual(
      data,
      projectRegistry(registry, { kind, value }),
      `Route-scoped projection changed: ${endpoint}`,
    );
  }
  const expectedApis = new Set(apiRoutes.map((endpoint) => `api/registry/${endpoint}`));
  for (const relative of artifactFiles.filter(
    (file) => file.startsWith('api/registry/') && !/\.(gz|br)$/.test(file),
  )) {
    assert(expectedApis.has(relative), `Unexpected API or client slug used as ID: ${relative}`);
  }
  const release = await json('api/releases/latest');
  assert(
    release && typeof release === 'object' && !Array.isArray(release),
    'Invalid release metadata',
  );
  assert(!fileSet.has('agents/index.html'));
  for (const locale of Object.keys(localeMetadata).filter((locale) => locale !== 'en')) {
    assert(!artifactFiles.some((file) => file.startsWith(`${locale}/api/`)), 'Localized API copy');
    if (!locales.includes(locale))
      assert(
        !artifactFiles.some((file) => file.startsWith(`${locale}/`)),
        `Draft output: ${locale}`,
      );
  }
  for (const fallback of ['404.html', '200.html'].filter((file) => fileSet.has(file))) {
    const errorHtml = (await read(fallback)).toString();
    assert(
      tags(errorHtml, 'meta').some((tag) => tag.name === 'robots' && /noindex/.test(tag.content)),
      `${fallback} must be noindex`,
    );
    assert(
      !tags(errorHtml, 'link').some((tag) => tag.rel === 'canonical' || tag.hreflang),
      `${fallback} must have no SEO identity`,
    );
  }
  console.log(
    `Verified final Pages artifact: ${expanded.length} HTML, ${expectedLocations.length} indexed, N=${registry.plugins.length}, ${apiRoutes.length} registry APIs; ${root}`,
  );
}
if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  // Every publishing path invokes this entrypoint through the final assembler.
  await checkI18n();
  await checkPagesArtifact();
}
