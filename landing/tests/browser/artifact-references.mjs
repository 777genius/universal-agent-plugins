import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { test } from 'node:test';
import { checkHtmlReferences } from '../../scripts/check-pages-artifact.mjs';
import { productRootUrl, productImageUrl, pageRouteSeo } from '../../utils/seo.ts';
import { productRoutes } from '../../data/routes.ts';

const routes = productRoutes(['example.plugin'], ['codex']);
test('F1 origin-only and prefixed configuration share one product root', () => {
  for (const base of ['/', '/universal-agent-plugins/']) {
    const expected = `https://example.test${base}`;
    for (const site of ['https://example.test', expected]) {
      assert.equal(productRootUrl(site, base), expected);
      assert.equal(
        pageRouteSeo('/plugins/example.plugin/', routes, site, base).canonical,
        `${expected}plugins/example.plugin/`,
      );
      for (const image of ['og-image.png', '/og-image.png', `${base}og-image.png`])
        assert.equal(productImageUrl(image, site, base), `${expected}og-image.png`);
      assert.equal(
        productImageUrl('https://images.test/custom.png', site, base),
        'https://images.test/custom.png',
      );
      assert.equal(
        productImageUrl('https://example.test/custom.png', site, base),
        'https://example.test/custom.png',
      );
    }
  }
});

test('F1 actual robots handler uses app base and preserves explicit docs sitemap', async () => {
  // Supply only Nuxt auto-imports; execute the actual handler without starting Nuxt.
  const { stripTypeScriptTypes } = await import('node:module');
  const source = await fs.readFile(
    new URL('../../server/routes/robots.txt.ts', import.meta.url),
    'utf8',
  );
  const executable = stripTypeScriptTypes(source.replace(/^import .*;\n/m, '')).replace(
    'export default ',
    'return ',
  );
  const run = (docsSitemapUrl) =>
    Function(
      'productRootUrl',
      'defineEventHandler',
      'useRuntimeConfig',
      'setHeader',
      executable,
    )(
      productRootUrl,
      (handler) => handler,
      () => ({
        public: { siteUrl: 'https://example.test', docsSitemapUrl },
        app: { baseURL: '/universal-agent-plugins/' },
      }),
      () => {},
    )({});
  assert.match(run(), /Sitemap: https:\/\/example.test\/universal-agent-plugins\/sitemap.xml\n/);
  assert.match(
    run(),
    /Sitemap: https:\/\/example.test\/universal-agent-plugins\/docs\/sitemap.xml\n/,
  );
  assert.match(run('https://docs.test/map.xml'), /Sitemap: https:\/\/docs.test\/map.xml\n/);
});

test('F3 Directory product grammar accepts dots without broadening client grammar', () => {
  for (const name of ['example.plugin', 'a', 'a.b-c', 'a.-b'])
    assert(productRoutes([name], ['codex']).some((route) => route.path === `/plugins/${name}/`));
  for (const name of [
    '.',
    '..',
    '../escape',
    'a..b',
    'a--b',
    '.a',
    'a.',
    'A',
    'a/b',
    'a%2fb',
    'a_b',
    'community',
  ])
    assert.throws(() => productRoutes([name], ['codex']), /Invalid|Reserved/);
  assert.throws(() => productRoutes(['a.b', 'a.b'], ['codex']), /duplicate/);
  assert.throws(() => productRoutes(['a.b'], ['codex', 'codex']), /duplicate/);
  assert.throws(() => productRoutes(['a.b'], ['client.name']), /Invalid/);
});

test('F2 independent references reject each review mutation at both deployment bases', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'uap-artifact-references-'));
  const fixture = {
    'index.html': '<h1 id="landing-anchor">Home</h1>',
    'plugins/index.html': '<h1 id="catalog">Catalog</h1>',
    'docs/en/guide/quickstart.html': '<h1 id="historical-v1">History</h1>',
    'assets/image.png': 'image fixture',
    'og-image.png': 'social fixture',
  };
  try {
    for (const [relative, bytes] of Object.entries(fixture)) {
      await fs.mkdir(path.dirname(path.join(root, relative)), { recursive: true });
      await fs.writeFile(path.join(root, relative), bytes);
    }
    for (const base of ['/', '/universal-agent-plugins/']) {
      const options = {
        root,
        fileSet: new Set(Object.keys(fixture)),
        routes,
        base,
        siteUrl: 'https://example.test',
        routePath: '/',
        allowedSiblingSites: ['https://example.test/other-site/'],
      };
      const check = (html) => checkHtmlReferences({ ...options, html });
      const valid = `<a href="${base}docs/en/guide/quickstart.html#historical-v1">History</a><a href="#landing-anchor">Home</a><a href="${base}plugins/#catalog">Catalog</a><img src="${base}assets/image.png"><source srcset="${base}assets/image.png 1x, https://cdn.test/image.png 2x"><a href="https://external.test/missing#anchor">External</a><a href="https://example.test/other-site/guide.html">Sibling</a><img src="data:image/png;base64,AAAA"><meta property="og:image" content="https://images.test/custom.png"><meta property="og:image" content="https://example.test/custom.png">`;
      await check(valid);
      await check('<a href="https://example.test/unrelated-project/manual.pdf">Other project</a>');
      for (const [name, addition, pattern] of [
        ['landing trailing slash', `<a href="${base}plugins?q=gitlab#catalog">Catalog</a>`, /Missing trailing slash/],
        ['missing image', `<img src="${base}missing.png">`, /Missing final artifact/],
        ['missing source', `<source src="${base}missing.webp">`, /Missing final artifact/],
        [
          'missing srcset image',
          `<img srcset="${base}assets/image.png 1x, ${base}missing.png 2x">`,
          /Missing final artifact/,
        ],
        [
          'missing source srcset',
          `<source srcset="${base}missing.webp 2x">`,
          /Missing final artifact/,
        ],
        ['landing anchor', '<a href="#missing-owned-anchor">Broken</a>', /Missing landing anchor/],
        [
          'cross-page landing anchor',
          `<a href="${base}plugins/#missing">Broken</a>`,
          /Missing landing anchor/,
        ],
        [
          'docs anchor',
          `<a href="${base}docs/en/guide/quickstart.html#missing">Broken</a>`,
          /Missing docs anchor/,
        ],
      ])
        await t.test(`${base} ${name}`, () => assert.rejects(check(valid + addition), pattern));
      if (base !== '/') {
        for (const href of [
          '/docs/en/guide/quickstart.html#historical-v1',
          '/plugins/',
          '/assets/image.png',
          '/og-image.png',
          'https://example.test/plugins/',
        ])
          await t.test(`missing base ${href}`, () =>
            assert.rejects(check(valid + `<a href="${href}">Broken</a>`), /Missing app base/),
          );
        await assert.rejects(check('<img src="/missing.png">'), /Missing app base/);
        await check('<img src="https://example.test/other-site/image.png">');
        await assert.rejects(
          checkHtmlReferences({
            ...options,
            allowedSiblingSites: ['https://example.test/'],
            html: '<a href="/docs/missing.html">Broken</a>',
          }),
          /Invalid sibling scope/,
        );
      }
    }
  } finally {
    await fs.rm(root, { recursive: true, force: true });
  }
});

test('F1 actual page composable emits based default OG and preserves absolute custom OG', async () => {
  const { stripTypeScriptTypes } = await import('node:module');
  const { computed, toValue } = await import('vue');
  const source = await fs.readFile(
    new URL('../../composables/usePageSeo.ts', import.meta.url),
    'utf8',
  );
  const executable =
    stripTypeScriptTypes(source.replace(/^import .*;\n/gm, '')).replace(
      'export const usePageSeo',
      'const usePageSeo',
    ) + '\nreturn usePageSeo;';
  for (const image of [
    undefined,
    { url: 'https://example.test/custom.png' },
    { url: 'https://cdn.test/custom.png' },
  ]) {
    let meta;
    const run = Function(
      'computed',
      'toValue',
      'pageRouteSeo',
      'productRootUrl',
      'productImageUrl',
      'useI18n',
      'useRoute',
      'useRuntimeConfig',
      'useSeoMeta',
      'useHead',
      executable,
    )(
      computed,
      toValue,
      pageRouteSeo,
      productRootUrl,
      productImageUrl,
      () => ({ t: (value) => value, locale: { value: 'en' } }),
      () => ({ path: '/plugins/example.plugin/' }),
      () => ({
        public: { siteUrl: 'https://example.test', seoRoutes: routes },
        app: { baseURL: '/universal-agent-plugins/' },
      }),
      (value) => {
        meta = value;
      },
      () => {},
    );
    run('Title', 'Description', { image });
    assert.equal(
      meta.ogUrl.value,
      'https://example.test/universal-agent-plugins/plugins/example.plugin/',
    );
    assert.equal(
      meta.ogImage.value,
      image?.url || 'https://example.test/universal-agent-plugins/og-image.png',
    );
    assert.equal(meta.twitterImage.value, meta.ogImage.value);
  }
});
