import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { test } from 'node:test';
import { runInNewContext } from 'node:vm';
import { computed, ref, watch, toValue } from 'vue';
import { clients, clientLandingBySlug, clientLandingById } from '../data/clients.ts';
import { installTargets, canonicalInstallTarget } from '../data/installTargets.ts';
import { hasPluginInstallTarget, pluginInstallAvailability } from '../utils/pluginInstallAvailability.ts';
import { agentInstallExample } from '../utils/agentInstallExample.ts';
import { pluginCommands } from '../utils/commands.ts';
import { pinnedCatalogCommands, pinnedCatalogRepairLimitation } from '../utils/projectCatalogSource.ts';
import { localizedPath } from '../utils/localizedRoutes.ts';
import { productRootUrl, seoDescription, spdxLicenseUrl } from '../utils/seo.ts';
import { catalogVisiblePlugins, isFullyRetired } from '../utils/filter.ts';
import { pluginBenefit } from '../data/catalogCuration.ts';
import { resolveDistribution, vendorInstallSource } from '../utils/registry.ts';

const fixture = JSON.parse(readFileSync(new URL('./fixtures/registry-responses/gitlab.json', import.meta.url), 'utf8'));
const discoverySource = stripTypeScriptTypes(readFileSync(new URL('../composables/useDiscoveryStatus.ts', import.meta.url), 'utf8')).replace(/^export /gm, '');
async function page(
  filename: string,
  parameters = { slug: 'gitlab', client: 'codex' },
  registry: unknown = fixture,
) {
  const locale = ref('en');
  const directoryCurrent = ref(true);
  let registryInitialized = false;
  const discoveryStatus = ref({ state: 'current' });
  let seo: any;
  const route = { params: parameters, query: { source: fixture.plugins[0].install_source } };
  const source = readFileSync(new URL(`../pages/${filename}`, import.meta.url), 'utf8')
    .match(/<script setup lang="ts">([\s\S]*?)<\/script>/)![1]
    .replace(/^import[\s\S]*?from '[^']+';\n/gm, '');
  const exposed = await runInNewContext(`(async () => { ${discoverySource}\n${stripTypeScriptTypes(source)}
    return { installCommand: typeof installCommand === 'undefined' ? undefined : installCommand,
      pinnedRepairLimitation: typeof pinnedRepairLimitation === 'undefined' ? undefined : pinnedRepairLimitation };
  })()`, {
    computed, ref, watch, clients, clientLandingBySlug, clientLandingById, localizedPath, productRootUrl, seoDescription, spdxLicenseUrl, catalogVisiblePlugins,
    resolveDistribution, vendorInstallSource, isFullyRetired, pluginBenefit,
    installTargets, canonicalInstallTarget, hasPluginInstallTarget, pluginInstallAvailability, agentInstallExample, pluginCommands, pinnedCatalogCommands, pinnedCatalogRepairLimitation,
    useDirectoryStatus: () => {
      assert.ok(registryInitialized, 'Directory status requires an awaited page projection on a cold render');
      return { current: directoryCurrent, expired: ref(false), published: ref(true) };
    },
    // Synthetic key markers test reactivity without creating translations.
    useI18n: () => ({ locale, t: (key: string) => `${locale.value}:${key}`, n: (value: number) => String(value) }),
    useLocalePath: () => (path: string) => localizedPath(path, locale.value as 'en' | 'uk'),
    useRoute: () => route,
    useRuntimeConfig: () => ({ app: { baseURL: '/universal-agent-plugins/' }, public: { siteUrl: 'https://example.test/universal-agent-plugins/' } }),
    useSite: () => ({ asset: (path: string) => path, pluginIcon: () => '', sourceUrl: () => 'https://github.com/original/source' }),
    useState: () => discoveryStatus,
    useRegistryPage: async () => {
      await Promise.resolve();
      registryInitialized = true;
      return structuredClone(registry);
    },
    usePageSeo: (title: unknown, description: unknown, options: unknown) => { seo = { title, description, options }; },
    createError: (options: object) => Object.assign(new Error(), options),
  });
  return { locale, seo, directoryCurrent, installCommand: exposed.installCommand,
    pinnedRepairLimitation: exposed.pinnedRepairLimitation };
}

test('registry index/detail/client structured data follows locale and uses benefit copy without rewriting source metadata', async () => {
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
      assert.equal(structured[0].description, pluginBenefit(fixture.plugins[0]));
      assert.ok(structured[0].url.endsWith('/uk/plugins/gitlab/'));
      assert.equal(
        toValue(seo.description),
        seoDescription(['uk:registryUi.detail.descriptionLead', pluginBenefit(fixture.plugins[0])]),
        'meta renders benefit-led presentation text through the existing length limit',
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

test('retired products never reach catalog ItemList structured data', async () => {
  const retired = structuredClone(fixture.plugins[0]);
  retired.name = 'retired-demo';
  retired.display_name = 'Retired Demo';
  retired.default_distribution = '777genius/retired-demo';
  retired.declared_default_distribution = '777genius/retired-demo';
  retired.distributions = retired.distributions.map((distribution: any) => ({
    ...distribution,
    id: '777genius/retired-demo',
    releases: distribution.releases.map((release: any) => ({ ...release, release_status: 'revoked' })),
    release_status: 'revoked',
  }));
  const registry = { ...fixture, plugins: [...fixture.plugins, retired] };
  for (const filename of ['plugins/index.vue', 'agents/[client].vue']) {
    const { seo } = await page(filename, { slug: 'gitlab', client: 'codex' }, registry);
    const structured = toValue(seo.options.structuredData) as any[];
    const names = structured[0].itemListElement.map((entry: any) => entry.name);
    assert.ok(names.includes('GitLab'), `${filename} keeps live products`);
    assert.ok(!names.includes('Retired Demo'), `${filename} drops retired products`);
    assert.equal(structured[0].numberOfItems, names.length);
  }
});

test('temporarily unlisted products never reach catalog surfaces', async () => {
  const hidden = structuredClone(fixture.plugins[0]);
  hidden.name = 'agent-notifications';
  hidden.display_name = 'Agent Notifications';
  const registry = { ...fixture, plugins: [...fixture.plugins, hidden] };
  for (const filename of ['plugins/index.vue', 'agents/[client].vue']) {
    const { seo } = await page(filename, { slug: 'gitlab', client: 'codex' }, registry);
    const structured = toValue(seo.options.structuredData) as any[];
    const names = structured[0].itemListElement.map((entry: any) => entry.name);
    assert.ok(names.includes('GitLab'), `${filename} keeps listed products`);
    assert.ok(!names.includes('Agent Notifications'), `${filename} drops held-back products`);
    assert.equal(structured[0].numberOfItems, names.length);
  }
});

test('agent pages use the exact source command and stop offering it when the Directory expires', async () => {
  const registry = structuredClone(fixture);
  registry.plugins[0].trust_state = 'reviewed';
  for (const client of ['hermes', 'antigravity', 'pi'] as const) {
    const { installCommand, directoryCurrent, pinnedRepairLimitation } = await page(
      'agents/[client].vue', { slug: 'gitlab', client }, registry,
    );
    assert.equal(toValue(installCommand), pinnedCatalogCommands(registry.plugins[0], client, 'user').add);
    assert.ok(toValue(installCommand).includes('@0.1.108 add github:'));
    assert.equal(toValue(pinnedRepairLimitation)?.key,
      client === 'antigravity' ? 'registryUi.install.antigravityRepairUnavailable' : undefined);
    directoryCurrent.value = false;
    assert.equal(toValue(installCommand), '', 'Expired source must never become a short-name command');
    assert.equal(toValue(pinnedRepairLimitation)?.key,
      client === 'antigravity' ? 'registryUi.install.antigravityRepairUnavailable' : undefined,
      'Expired Directory must not resurrect a broken public repair claim');
  }
});
