import assert from 'node:assert/strict';
import { readFileSync, realpathSync } from 'node:fs';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';
import { runInNewContext } from 'node:vm';
import { createSSRApp, h, type ComponentOptions } from 'vue';
import { renderToString } from 'vue/server-renderer';
import { parse } from 'vue/compiler-sfc';
import { publishedLocales } from '../data/i18n.ts';
import { loadI18nMessages } from '../scripts/load-i18n-messages.ts';

const runtimeModulePath = new URL(
  '../node_modules/vue-i18n/dist/vue-i18n.runtime.mjs',
  import.meta.url,
).href;
const { createI18n, useI18n } = (await import(runtimeModulePath)) as typeof import('vue-i18n');

// Use the same installed compiler Nuxt uses, including its external SFC block
// path. The runtime-only composer proves this does not rely on a message compiler.
const requireI18n = createRequire(
  realpathSync(new URL('../node_modules/@nuxtjs/i18n/dist/module.mjs', import.meta.url)),
);
const { unplugin } = await import(requireI18n.resolve('@intlify/unplugin-vue-i18n'));
const compiler = unplugin.vite({ include: [], runtimeOnly: true });
const filename = fileURLToPath(
  new URL('../components/registry/UnifiedCatalog.vue', import.meta.url),
);
const { descriptor } = parse(readFileSync(filename, 'utf8'), { filename });
const resources: Record<string, NonNullable<ComponentOptions['__i18n']>> = {};
for (const block of descriptor.customBlocks) {
  if (block.type !== 'i18n') continue;
  const path = new URL(
    block.src!,
    new URL('../components/registry/UnifiedCatalog.vue', import.meta.url),
  );
  const locale = String(block.attrs.locale);
  const request = `${fileURLToPath(path)}?vue&type=custom&index=0&src=true&blockType=i18n&locale=${locale}&lang.json`;
  const result = await compiler.transform.call(
    {
      error: (error: unknown) => {
        throw error;
      },
      warn() {},
    },
    readFileSync(path, 'utf8'),
    request,
  );
  const component: ComponentOptions = {};
  const attach = runInNewContext(result.code.replace('export default', '(') + ')');
  attach(component);
  resources[locale] = component.__i18n!;
}

test('Skills resources compile outside global locale includes and render in runtime-only local scope', async () => {
  assert.deepEqual(Object.keys(resources).sort(), [...publishedLocales].sort());
  for (const locale of publishedLocales) {
    const full = loadI18nMessages(locale);
    const base = JSON.parse(
      readFileSync(new URL(`../i18n/locales/${locale}.json`, import.meta.url), 'utf8'),
    );
    assert.equal(
      base.registryUi.skills,
      undefined,
      `${locale}: feature excluded from global payload`,
    );
    for (const key of [
      'explorePlugins',
      'filterByCapabilitySourceAuthenticationOrSupportedAgent',
      'pluginDirectory',
      'findAgentPlugins10',
      'useThemEverywhere',
      'searchReviewedPackagesAndCommunityPluginsDiscoveredFromPublicGithubRepositories',
    ]) {
      assert.equal(
        base.registryUi.directoryPage[key],
        undefined,
        `${locale}: catalog presentation excluded globally`,
      );
      assert.equal(
        typeof full.registryUi.directoryPage[key as keyof typeof full.registryUi.directoryPage],
        'string',
        `${locale}: shared gate includes presentation`,
      );
    }
    for (const key of ['title', 'description', 'listName'] as const) {
      assert.equal(
        full.registryUi.directoryPage[key],
        base.registryUi.directoryPage[key],
        `${locale}: shared gate preserves global SEO`,
      );
    }
    const i18n = createI18n<[{ shared: () => string }], string, false>({
      legacy: false,
      locale,
      fallbackLocale: 'en',
      messages: { [locale]: { shared: () => 'Shared plugin label' } },
    });
    let entryComposer: unknown;
    const child = {
      setup() {
        const inherited = useI18n({ useScope: 'parent' });
        assert.equal(inherited, entryComposer, `${locale}: children reuse the entry composer`);
        return () => h('span', inherited.t('registryUi.skills.chooseInTerminal'));
      },
    };
    const component = {
      __i18n: publishedLocales.flatMap((code) => resources[code]!),
      setup() {
        const local = useI18n({ useScope: 'local' });
        local.missingWarn = false;
        local.fallbackWarn = false;
        entryComposer = local;
        return () =>
          h('main', [
            h('h1', local.t('registryUi.skills.heading')),
            h('h2', local.t('registryUi.directoryPage.explorePlugins')),
            h('p', local.t('shared')),
            h(child),
          ]);
      },
    };
    const app = createSSRApp(component);
    app.use(i18n);
    const html = await renderToString(app);
    const expected = loadI18nMessages(locale).registryUi.skills;
    assert.ok(html.includes(expected.heading), locale);
    assert.ok(
      html.includes(full.registryUi.directoryPage.explorePlugins),
      `${locale}: scoped plugin heading`,
    );
    assert.ok(html.includes(expected.chooseInTerminal), locale);
    assert.ok(html.includes('Shared plugin label'), `${locale}: global fallback preserved`);
    assert.equal(
      Object.hasOwn(i18n.global.getLocaleMessage(locale), 'registryUi'),
      false,
      `${locale}: local mount never merges feature into global messages`,
    );
    i18n.dispose();
  }
});
