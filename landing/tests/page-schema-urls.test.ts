import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { test } from 'node:test';
import { runInNewContext } from 'node:vm';
import { computed, ref } from 'vue';
import { isKnownLocale } from '../data/i18n.ts';
import { localizedPath } from '../utils/localizedRoutes.ts';
import {
  productRootUrl,
  productSoftwareSchema,
  seoDescription,
  spdxLicenseUrl,
} from '../utils/seo.ts';
import { checkPageSchemaUrls } from '../scripts/check-pages-artifact.mjs';

const cases = [
  ['index.vue', 'home', '/'],
  ['download.vue', 'download', '/download/'],
  ['plugins/index.vue', 'catalog', '/plugins/'],
  ['plugins/[slug].vue', 'plugin', '/plugins/context7/'],
  ['agents/[client].vue', 'agent', '/agents/codex/'],
] as const;

// Execute each actual page setup with data/composable adapters; inspect its emitted
// schema callback, rather than duplicating the page URL construction in the test.
for (const base of ['/', '/universal-agent-plugins/']) {
  for (const siteUrl of new Set(['https://example.test', `https://example.test${base}`])) {
    for (const locale of ['en', 'ru', 'uk'] as const) {
      for (const [file, family, path] of cases) {
        test(`${file}: ${locale}, base ${base}, site ${siteUrl}`, async () => {
          const source = readFileSync(new URL(`../pages/${file}`, import.meta.url), 'utf8')
            .split('<script setup lang="ts">')[1]!
            .split('</script>')[0]!
            .replace(/^import[\s\S]*?from '[^']+';\n/gm, '');
          let options: any;
          const client = { id: 'codex', slug: 'codex', name: 'Codex', presentationKey: 'fixture' };
          const plugin = {
            name: 'context7',
            display_name: 'Context7',
            trust_state: 'reviewed',
            description: 'Fixture',
            version: '1.0.0',
            license: 'MIT',
            keywords: [],
            author: { name: 'Fixture', url: 'https://author.test/' },
            client_support: { clients: ['codex'], delivery: { codex: 'managed' } },
          };
          await runInNewContext(`(async () => {${stripTypeScriptTypes(source)}\n})()`, {
            computed,
            ref,
            isKnownLocale,
            localizedPath,
            productRootUrl,
            productSoftwareSchema,
            seoDescription,
            spdxLicenseUrl,
            useI18n: () => ({ t: (key: string) => key, locale: ref(locale) }),
            useLocalePath: () => (value: string) => value,
            useRuntimeConfig: () => ({
              app: { baseURL: base },
              public: {
                siteUrl,
                githubRepo: 'fixture/repo',
                githubReleasesUrl: 'https://github.com/fixture/repo/releases',
              },
            }),
            useRegistryPage: async () => ({ plugins: [plugin] }),
            useRoute: () => ({ params: { slug: 'context7', client: 'codex' } }),
            useDocsLinks: () => ({ docsUrl: ref('https://docs.test/') }),
            useSite: () => ({
              pluginIcon: () => '',
              sourceUrl: () => 'https://github.com/fixture/repo',
            }),
            clients: [client],
            clientLandingBySlug: new Map([['codex', client]]),
            usePageSeo: (_title: unknown, _description: unknown, value: unknown) => {
              options = value;
            },
          });
          const graphs = JSON.parse(JSON.stringify(options.structuredData()));
          const route = { family, path: localizedPath(path, locale), locale };
          checkPageSchemaUrls(graphs, route, siteUrl, base);
          const root = `https://example.test${base}`;
          const canonical = root + localizedPath(path, locale).slice(1);
          const properties =
            typeof options.pageProperties === 'function'
              ? options.pageProperties()
              : options.pageProperties;
          const identity =
            family === 'home' || family === 'download'
              ? `${root}#software`
              : `${canonical}#${family === 'plugin' ? 'plugin' : 'plugin-list'}`;
          assert.equal((properties.mainEntity || properties.about)['@id'], identity);
          // Every emitted owned schema URL gets an independent root-omission
          // mutation. External repository/author/docs URLs are deliberately excluded.
          if (base !== '/') {
            const mutate = (value: any, keys: (string | number)[] = []) => {
              for (const [key, child] of Object.entries(value)) {
                const trail = [...keys, key];
                if (typeof child === 'string' && child.startsWith(root)) {
                  const broken = structuredClone(graphs);
                  let parent: any = broken;
                  for (const part of trail.slice(0, -1)) parent = parent[part];
                  parent[key] = child.replace(base, '/');
                  assert.throws(
                    () => checkPageSchemaUrls(broken, route, siteUrl, base),
                    assert.AssertionError,
                    trail.join('.'),
                  );
                } else if (child && typeof child === 'object') mutate(child, trail);
              }
            };
            mutate(graphs);
          }
        });
      }
    }
  }
}
