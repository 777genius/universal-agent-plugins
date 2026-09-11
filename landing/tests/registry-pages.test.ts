import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { test } from 'node:test';
import { runInNewContext } from 'node:vm';
import { computed, ref, watch, toValue } from 'vue';
import { clients, clientLandingBySlug, clientLandingById } from '../data/clients.ts';
import { localizedPath } from '../utils/localizedRoutes.ts';
import { productRootUrl, seoDescription, spdxLicenseUrl } from '../utils/seo.ts';

const fixture = JSON.parse(readFileSync(new URL('./fixtures/registry-responses/gitlab.json', import.meta.url), 'utf8'));
async function page(filename: string, parameters = { slug: 'gitlab', client: 'codex' }) {
  const locale = ref('en');
  let seo: any;
  const route = { params: parameters, query: { source: fixture.plugins[0].install_source } };
  const source = readFileSync(new URL(`../pages/${filename}`, import.meta.url), 'utf8')
    .match(/<script setup lang="ts">([\s\S]*?)<\/script>/)![1]
    .replace(/^import[\s\S]*?from '[^']+';\n/gm, '');
  await runInNewContext(`(async () => { ${stripTypeScriptTypes(source)} })()`, {
    computed, ref, watch, clients, clientLandingBySlug, clientLandingById, localizedPath, productRootUrl, seoDescription, spdxLicenseUrl,
    // Synthetic key markers test reactivity without creating translations.
    useI18n: () => ({ locale, t: (key: string) => `${locale.value}:${key}`, n: (value: number) => String(value) }),
    useLocalePath: () => (path: string) => localizedPath(path, locale.value as 'en' | 'uk'),
    useRoute: () => route,
    useRuntimeConfig: () => ({ app: { baseURL: '/universal-agent-plugins/' }, public: { siteUrl: 'https://example.test/universal-agent-plugins/' } }),
    useSite: () => ({ asset: (path: string) => path, pluginIcon: () => '', sourceUrl: () => 'https://github.com/original/source' }),
    useDiscoveryStatus: () => ref({ state: 'current' }),
    useRegistryPage: async () => structuredClone(fixture),
    usePageSeo: (title: unknown, description: unknown, options: unknown) => { seo = { title, description, options }; },
    createError: (options: object) => Object.assign(new Error(), options),
  });
  return { locale, seo };
}

test('registry index/detail/client structured data follows locale and keeps original plugin descriptions', async () => {
  for (const filename of ['plugins/index.vue', 'plugins/[slug].vue', 'agents/[client].vue']) {
    const { locale, seo } = await page(filename);
    assert.ok(toValue(seo.title).startsWith('en:'));
    locale.value = 'uk';
    assert.ok(toValue(seo.title).startsWith('uk:'));
    const structured = toValue(seo.options.structuredData) as any[];
    assert.ok(structured[0]['@id'].startsWith('https://example.test/universal-agent-plugins/uk/'));
    assert.ok(!JSON.stringify(structured).includes('/universal-agent-plugins/universal-agent-plugins/'));
    assert.equal(seo.options.canonicalPath, undefined, 'shared SEO helper owns current canonical');
    if (filename.includes('[slug]')) {
      assert.equal(structured[0].description, fixture.plugins[0].description);
      assert.ok(structured[0].url.endsWith('/uk/plugins/gitlab/'));
      assert.equal(
        toValue(seo.description),
        seoDescription(['uk:registryUi.detail.descriptionLead', fixture.plugins[0].description]),
        'meta retains the original description through the existing length limit',
      );
    } else {
      for (const entry of structured[0].itemListElement) assert.ok(entry.url.includes('/uk/plugins/'));
    }
  }
});

test('community keeps noindex, no canonical and no WebPage identity; unknown reviewed routes stay 404', async () => {
  const { locale, seo } = await page('plugins/community.vue');
  assert.equal(seo.options.robots, 'noindex, follow');
  assert.equal(seo.options.canonical, false); assert.equal(seo.options.includeWebPage, false);
  locale.value = 'uk'; assert.ok(toValue(seo.title).startsWith('uk:'));
  for (const filename of ['plugins/[slug].vue', 'agents/[client].vue']) {
    await assert.rejects(page(filename, { slug: 'missing', client: 'missing' }), { statusCode: 404 });
  }
});
