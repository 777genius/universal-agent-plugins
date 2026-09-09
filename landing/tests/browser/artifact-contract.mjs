import fs from 'node:fs/promises';
import path from 'node:path';
import os from 'node:os';
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { checkPagesArtifact } from '../../scripts/check-pages-artifact.mjs';
import { productRoutes } from '../../data/routes.ts';
import { publishedLocales, candidateLocales } from '../../data/i18n.ts';
import { expandLocalizedRoutes } from '../../utils/localizedRoutes.ts';
import { pageRouteSeo } from '../../utils/seo.ts';
import { clientLandingPages } from '../../data/clients.ts';
import { parseDirectoryData } from '../../utils/registry.ts';
import { projectRegistry } from '../../utils/registryProjection.ts';

// A deliberately small HTML fixture exercises rejection behavior; never a deployable site.
// Snapshot bytes are the verified build input, not a fabricated registry or plugin counter.
test('final artifact gate rejects missing, altered, draft and mis-scoped output at both bases', async () => {
  const mirror = process.env.UAP_VERIFIED_MIRROR;
  assert(mirror, 'Set UAP_VERIFIED_MIRROR to the staged verified build mirror');
  const pointer = JSON.parse(
    await fs.readFile(path.join(mirror, 'registry/schemas/1/latest.json'), 'utf8'),
  );
  const registry = parseDirectoryData(
    JSON.parse(
      await fs.readFile(path.join(mirror, 'registry/schemas/1', pointer.snapshot_path), 'utf8'),
    ),
    'published_snapshot',
  );
  const routes = productRoutes(
    registry.plugins.map((plugin) => plugin.name),
    clientLandingPages.map((client) => client.slug),
  );
  const temp = await fs.mkdtemp(path.join(os.tmpdir(), 'uap-pages-check-'));
  const root = path.join(temp, 'pages');
  const docsDist = path.join(temp, 'docs');
  const put = async (file, content) => {
    await fs.mkdir(path.dirname(file), { recursive: true });
    await fs.writeFile(file, content);
  };
  const escape = (value) => value.replaceAll('&', '&amp;').replaceAll('"', '&quot;');
  try {
    await put(path.join(docsDist, 'index.html'), '<h1>Docs</h1>');
    await put(
      path.join(docsDist, 'en/guide/quickstart.html'),
      '<h1 id="historical-v1">Historical v1</h1>',
    );
    await put(path.join(docsDist, 'sitemap.xml'), '<urlset/>');
    for (const [base, locales] of [
      ['/', publishedLocales],
      ['/universal-agent-plugins/', candidateLocales],
    ]) {
      await fs.rm(root, { recursive: true, force: true });
      await fs.cp(mirror, root, { recursive: true });
      await fs.cp(docsDist, path.join(root, 'docs'), { recursive: true });
      await put(path.join(root, '.nojekyll'), '');
      await put(path.join(root, '404.html'), '<meta name="robots" content="noindex, follow">');
      await put(path.join(root, 'robots.txt'), 'User-agent: *');
      const siteUrl = `https://example.test${base}`;
      const entries = [];
      for (const route of expandLocalizedRoutes(routes, locales)) {
        const seo = pageRouteSeo(route.path, routes, siteUrl, base, locales);
        const links = seo.alternates
          .map(
            (link) =>
              `<link rel="alternate" hreflang="${link.hreflang}" href="${escape(link.href)}">`,
          )
          .join('');
        const graph =
          route.family === 'community'
            ? []
            : [{ '@type': 'WebPage', url: seo.canonical, inLanguage: route.locale }];
        // Minimal owned schemas for the strengthened page-specific URL gate.
        const canonical = seo.canonical;
        const localized = (value) =>
          `${siteUrl}${route.locale === 'en' ? '' : `${route.locale}/`}${value}`;
        if (['home', 'download'].includes(route.family)) {
          graph.push({
            '@type': 'SoftwareApplication',
            '@id': `${siteUrl}#software`,
            url: siteUrl,
            publisher: { '@id': `${siteUrl}#organization` },
            screenshot: `${siteUrl}og-image.png`,
          });
          if (route.family === 'home')
            graph.push({ '@type': 'FAQPage', '@id': `${canonical}#faq` });
        }
        if (route.family === 'plugin')
          graph.push({
            '@type': 'SoftwareSourceCode',
            '@id': `${canonical}#plugin`,
            url: canonical,
            isPartOf: { '@id': `${localized('plugins/')}#webpage` },
          });
        if (['catalog', 'agent'].includes(route.family))
          graph.push({
            '@type': 'ItemList',
            '@id': `${canonical}#plugin-list`,
            itemListElement: [{ url: localized(`plugins/${registry.plugins[0].name}/`) }],
          });
        if (['plugin', 'agent'].includes(route.family))
          graph.push({
            '@type': 'BreadcrumbList',
            '@id': `${canonical}#breadcrumb`,
            itemListElement: [
              { item: localized(route.family === 'plugin' ? 'plugins/' : '') },
              { item: canonical },
            ],
          });
        const html = `<html lang="${route.locale}"><head>${seo.canonical ? `<link rel="canonical" href="${seo.canonical}">` : ''}${links}<meta property="og:locale" content="${seo.ogLocale}">${seo.ogAlternates.map((value) => `<meta property="og:locale:alternate" content="${value}">`).join('')}<meta name="robots" content="${route.indexable ? 'index, follow' : 'noindex, follow'}"><script type="application/ld+json">${JSON.stringify({ '@graph': graph })}</script></head><body><a href="${base}docs/en/guide/quickstart.html#historical-v1">History</a></body></html>`;
        await put(path.join(root, route.path.slice(1), 'index.html'), html);
        await put(path.join(root, route.path.slice(1), '_payload.json'), '[]');
        if (route.indexable)
          entries.push(
            `<url><loc>${escape(seo.canonical)}</loc>${seo.alternates.map((link) => `<xhtml:link rel="alternate" hreflang="${link.hreflang}" href="${escape(link.href)}"/>`).join('')}</url>`,
          );
      }
      await put(path.join(root, 'sitemap.xml'), `<urlset>${entries.join('')}</urlset>`);
      for (const projection of [
        { kind: 'catalog' },
        { kind: 'empty' },
        ...clientLandingPages.map((client) => ({ kind: 'client', value: client.id })),
        ...registry.plugins.map((plugin) => ({ kind: 'plugin', value: plugin.name })),
      ]) {
        await put(
          path.join(
            root,
            `api/registry/${projection.kind}${projection.value ? `/${projection.value}` : ''}`,
          ),
          JSON.stringify(projectRegistry(registry, projection)),
        );
      }
      await put(path.join(root, 'api/releases/latest'), '{"ok":false,"source":"unavailable"}');
      const options = { root, mirror, docsDist, base, siteUrl, locales };
      await checkPagesArtifact(options);
      const mutation = async (relative, value, pattern) => {
        const file = path.join(root, relative);
        let previous;
        try {
          previous = await fs.readFile(file);
        } catch (error) {
          if (error.code !== 'ENOENT') throw error;
        }
        if (value === null) await fs.rm(file);
        else await put(file, value);
        try {
          await assert.rejects(checkPagesArtifact(options), pattern);
        } finally {
          if (previous) await put(file, previous);
          else await fs.rm(file, { force: true });
        }
      };
      await mutation('download/index.html', null, /Missing final artifact/);
      await mutation('download/_payload.json', null, /Missing final artifact/);
      await mutation(
        'download/_payload.json',
        JSON.stringify([registry.plugins[0].description]),
        /Unrelated registry/,
      );
      await mutation(
        'api/registry/client/copilot',
        JSON.stringify(projectRegistry(registry, { kind: 'empty' })),
        /Route-scoped projection/,
      );
      await mutation('api/registry/client/github-copilot-cli', '{}', /Unexpected API/);
      await mutation('agents/index.html', '<h1>Fake catalog</h1>', /Unexpected\/draft\/fake HTML/);
      await mutation('es/index.html', '<h1>Draft</h1>', /Unexpected\/draft\/fake HTML/);
      await mutation('registry/schemas/1/latest.json', '{}', /Assembly changed bytes/);
      await mutation(
        'docs/en/guide/quickstart.html',
        '<h1>Removed history</h1>',
        /Assembly changed bytes/,
      );
      const home = await fs.readFile(path.join(root, 'index.html'), 'utf8');
      await mutation(
        'index.html',
        home.replace('#historical-v1', '#missing'),
        /Missing docs anchor/,
      );
      await mutation(
        'index.html',
        home.replace('hreflang="x-default"', 'hreflang="es"'),
        /Alternates/,
      );
      await mutation(
        '404.html',
        '<meta name="robots" content="index">',
        /404.html must be noindex/,
      );
      await mutation('ru/api/registry/catalog', '{}', /Localized API copy|Draft output/);
      await mutation(
        'plugins/not-reviewed/index.html',
        '<h1>Fake plugin</h1>',
        /Unexpected\/draft\/fake HTML/,
      );
      await mutation('api/releases/latest', '[]', /Invalid release metadata/);
      await mutation(
        'index.html',
        home.replace(
          'property="og:locale" content="en_US"',
          'property="og:locale" content="ru_RU"',
        ),
        /Expected values/,
      );
      const create = await fs.readFile(path.join(root, 'create-plugin/index.html'), 'utf8');
      await mutation(
        'create-plugin/index.html',
        create.replace('noindex, follow', 'index, follow'),
        /Robots/,
      );
      const community = await fs.readFile(path.join(root, 'plugins/community/index.html'), 'utf8');
      await mutation(
        'plugins/community/index.html',
        community.replace('<head>', '<head><link rel="canonical" href="https://example.test/">'),
        /Canonical/,
      );
      const sitemap = await fs.readFile(path.join(root, 'sitemap.xml'), 'utf8');
      await mutation(
        'sitemap.xml',
        sitemap.replace('hreflang="x-default"', 'hreflang="es"'),
        /Expected values/,
      );
      assert(publishedLocales.length >= 1);
    }
  } finally {
    await fs.rm(temp, { recursive: true, force: true });
  }
});

test('dictionary gate rejects missing/empty keys, placeholder changes and invalid message syntax', async () => {
  const { validateMessages } = await import('../../scripts/check-i18n.mjs');
  const reference = {
    message: 'Install {name}',
    count: '{count} item | {count} items',
    npm: "package{'@'}latest",
    security: 'SEC324 --target',
  };
  validateMessages(reference, reference, 'en');
  assert.throws(() => validateMessages(reference, { ...reference, message: '' }, 'uk'), /empty/);
  assert.throws(
    () => validateMessages(reference, { ...reference, message: 'Install {other}' }, 'uk'),
    /placeholders/,
  );
  assert.throws(
    () => validateMessages(reference, { ...reference, npm: 'package@latest' }, 'uk'),
    /compilation errors/,
  );
  assert.throws(
    () => validateMessages(reference, { ...reference, security: 'SEC325 --target' }, 'uk'),
    /protected literals/,
  );
  const missing = { ...reference };
  delete missing.count;
  assert.throws(() => validateMessages(reference, missing, 'uk'), /keyset mismatch/);
});

test('publication copy gate rejects English clones while preserving exact brands and commands', async () => {
  const { assertTranslatedCopy } = await import('../../scripts/check-i18n.mjs');
  const source = {
    action: 'Install plugins',
    brand: 'Agent Plugins 1.0',
    command: 'npx universal-agent-plugins add context7',
  };
  assertTranslatedCopy(source, source, 'en');
  assert.throws(() => assertTranslatedCopy(source, source, 'uk'), /unchanged English copy/);
  // Deliberately nonlinguistic fixture: this proves brand exceptions, not translation quality.
  assertTranslatedCopy(source, { ...source, action: 'fixture-changed' }, 'uk');
});

test('plural contracts preserve per-branch multiplicity across EN two and Slavic three forms', async () => {
  const { validateMessages } = await import('../../scripts/check-i18n.mjs');
  const reference = { agents: '{count} agent {count} | {count} agents {count}' };
  for (const locale of ['ru', 'uk']) {
    validateMessages(reference, { agents: '{count} ONE {count} | {count} FEW {count} | {count} MANY {count}' }, locale);
    for (const agents of [
      '{count} ONE {count} | {count} FEW {count} | {count} MANY',
      '{count} ONE {count} | {count} FEW {count} | {other} MANY {count}',
      '{count} ONE {count} {count} | FEW {count} | {count} MANY {count}',
    ]) assert.throws(() => validateMessages(reference, { agents }, locale), /placeholders branch/);
    assert.throws(() => validateMessages(reference, { agents: '{count} only {count}' }, locale), /plural branches/);
    const distinct = { count: 'one {name} | many {count}' };
    validateMessages(distinct, { count: 'ONE {name} | FEW {count} | MANY {count}' }, locale);
    assert.throws(() => validateMessages(distinct, { count: 'ONE {count} | FEW {name} | MANY {count}' }, locale), /placeholders branch/);
    validateMessages({ literal: "literal {'|'} {name}" }, { literal: "text {'|'} {name}" }, locale);
  }
});

test('unchanged Windows PowerShell allowance is restricted to the exact technical overlay caption', async () => {
  const { assertTranslatedCopy } = await import('../../scripts/check-i18n.mjs');
  const overlay = { shell: { download: { channels: { powershell: { title: 'Windows PowerShell' } } } } };
  for (const locale of ['ru', 'uk']) {
    assertTranslatedCopy(overlay, overlay, locale);
    assert.throws(() => assertTranslatedCopy({ prose: 'Windows PowerShell' }, { prose: 'Windows PowerShell' }, locale), /unchanged English/);
    const prose = { shell: { download: { channels: { powershell: { title: 'Use Windows PowerShell' } } } } };
    assert.throws(() => assertTranslatedCopy(prose, prose, locale), /unchanged English/);
  }
});
