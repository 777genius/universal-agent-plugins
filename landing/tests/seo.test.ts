import { productRoutes } from '../data/routes.ts';
import { candidateLocales } from '../data/i18n.ts';
import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import {
  canonicalPath,
  pageRouteSeo,
  noindexStaticFallback,
  productSoftwareSchema,
  seoDescription,
  spdxLicenseUrl,
} from '../utils/seo.ts';

describe('SEO foundations', () => {
  it('normalizes every indexable page to its GitHub Pages directory URL', () => {
    assert.equal(canonicalPath('/'), '/');
    assert.equal(canonicalPath('/plugins'), '/plugins/');
    assert.equal(canonicalPath('plugins/gitlab/'), '/plugins/gitlab/');
    assert.equal(canonicalPath('//plugins//gitlab//'), '/plugins/gitlab/');
  });

  it('keeps generated plugin descriptions complete and within snippet guidance', () => {
    const description = seoDescription([
      'Install the Example Agent Plugin for Codex, Claude Code, and Cursor.',
      'A deliberately long explanation '.repeat(8),
    ]);
    assert.ok(description.length <= 160);
    assert.ok(description.endsWith('…'));
    assert.equal(description.includes('  '), false);
  });

  it('publishes complete free software application data', () => {
    const schema = productSoftwareSchema({
      siteUrl: 'https://example.com/product/',
      githubUrl: 'https://github.com/example/product',
      releasesUrl: 'https://github.com/example/product/releases',
      docsUrl: 'https://example.com/product/docs/',
      description: 'One CLI for Agent Plugins 1.0.',
    });
    assert.equal(schema.url, 'https://example.com/product/');
    assert.deepEqual(schema.offers, {
      '@type': 'Offer',
      price: '0',
      priceCurrency: 'USD',
    });
    assert.equal(schema.isAccessibleForFree, true);
    assert.equal(schema.installUrl, 'https://www.npmjs.com/package/universal-agent-plugins');
    assert.equal(schema.screenshot, 'https://example.com/product/og-image.png');
    assert.deepEqual(schema.publisher, { '@id': 'https://example.com/product/#organization' });
    assert.equal(spdxLicenseUrl('Apache-2.0'), 'https://spdx.org/licenses/Apache-2.0.html');
    assert.equal(spdxLicenseUrl('invalid license'), undefined);
  });
});

describe('manifest-driven localized SEO', () => {
  const routes = productRoutes(['context7'], ['github-copilot-cli']);
  for (const base of ['/', '/universal-agent-plugins/']) {
    it(`keeps reciprocal locale identity and applies base once at ${base}`, () => {
      const site = `https://example.test${base}`;
      for (const locale of candidateLocales) {
        const relative = `${locale === 'en' ? '' : `/${locale}`}/plugins/context7/`;
        for (const input of [relative, `${base}${relative.slice(1)}`]) {
          const seo = pageRouteSeo(
            `${input}?q=hello#security`,
            routes,
            site,
            base,
            candidateLocales,
          );
          assert.equal(seo.canonical, `${site}${relative.slice(1)}`);
          assert.deepEqual(
            seo.alternates.map((link) => link.hreflang),
            ['en', 'ru', 'uk', 'x-default'],
          );
          assert.equal(
            seo.alternates.find((link) => link.hreflang === locale)?.href,
            seo.canonical,
          );
          assert.equal(seo.ogLocale, { en: 'en_US', ru: 'ru_RU', uk: 'uk_UA' }[locale]);
          assert.equal(seo.ogAlternates.length, 2);
        }
      }
    });
  }
  it('preserves canonical distinctions without inventing routes or publishing drafts', () => {
    const get = (path: string) => pageRouteSeo(path, routes, 'https://example.test', '/');
    assert.equal(get('/create-plugin/').canonical, 'https://example.test/create-plugin/');
    assert.deepEqual(get('/create-plugin/').alternates, []);
    for (const path of [
      '/plugins/community/?source=test',
      '/plugins/missing/',
      '/agents/',
      '/agents/copilot/',
      '/es/',
      '/api/registry/catalog',
    ]) {
      assert.equal(get(path).canonical, undefined, path);
      assert.deepEqual(get(path).alternates, [], path);
    }
    assert.equal(
      get('/agents/github-copilot-cli/').canonical,
      'https://example.test/agents/github-copilot-cli/',
    );
    assert.throws(() => productRoutes(['community'], []), /Reserved/);
    assert.throws(() => productRoutes(['same', 'same'], []), /duplicate/);
  });
});

it('marks shared static fallbacks noindex without rewriting script or asset bytes', () => {
  const html =
    '<!doctype html><html><head><link rel="stylesheet" href="/_nuxt/style.css"><script>window.fixture="<meta>";</script></head><body><script src="/_nuxt/app.js"></script></body></html>';
  const result = noindexStaticFallback(html);
  assert.equal(result.replace('<meta name="robots" content="noindex, follow">', ''), html);
  assert.equal(noindexStaticFallback(result), result);
  assert(
    !noindexStaticFallback(
      html.replace('<head>', '<head><meta name="robots" content="index">'),
    ).includes('content="index"'),
  );
  assert.throws(() => noindexStaticFallback('not html'), /no HTML head/);
});

it('accepts localized software captions without changing machine identity', () => {
  const schema = productSoftwareSchema({
    siteUrl: 'https://example.test/',
    githubUrl: 'https://github.com/example/product',
    releasesUrl: 'https://example.test/releases',
    docsUrl: 'https://example.test/docs/en/',
    description: 'fixture-description',
    applicationSubCategory: 'fixture-category',
    softwareRequirements: 'fixture-requirements',
    featureList: 'fixture-features',
  });
  assert.equal(schema.applicationSubCategory, 'fixture-category');
  assert.equal(schema.softwareRequirements, 'fixture-requirements');
  assert.equal(schema.featureList, 'fixture-features');
  assert.equal(schema['@id'], 'https://example.test/#software');
  assert.equal(schema.installUrl, 'https://www.npmjs.com/package/universal-agent-plugins');
});
