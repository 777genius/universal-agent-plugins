import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import { test } from 'node:test';
import { stripTypeScriptTypes } from 'node:module';
import { createI18n } from 'vue-i18n';
import { computed, ref, watch, effectScope, nextTick } from 'vue';
import { createPinia, setActivePinia } from 'pinia';
import { assembleDownloadContent, downloadTechnical } from '../data/download.ts';
import { docsAvailability, resolveDocsLink, ownedDocsRoot } from '../data/docsAvailability.ts';
import { registryFaqItems } from '../data/registryFaq.ts';
import { clientLandingPages } from '../data/clients.ts';
import { isKnownLocale } from '../data/i18n.ts';
import { localizedPath } from '../utils/localizedRoutes.ts';
import { useInstallPreferencesStore } from '../stores/installPreferences.ts';
import { normalizeInstallPlatform, recommendedInstallChannelId } from '../utils/installPlatform.ts';
import type { DownloadOverlay } from '../types/download.ts';

const read = (path: string) => readFileSync(new URL(`../${path}`, import.meta.url), 'utf8');
const messages: typeof import('../locales/en.json') = JSON.parse(read('locales/en.json'));
const overlay = JSON.parse(read('content/download/en.json')) as DownloadOverlay;
const legacy = JSON.parse(read('content/en.json'));

function leaves(value: unknown, prefix = ''): Array<[string, string]> {
  if (typeof value === 'string') return [[prefix, value]];
  return Object.entries(value as Record<string, unknown>).flatMap(([key, child]) =>
    leaves(child, prefix ? `${prefix}.${key}` : key),
  );
}

test('shell English messages compile, interpolate and retain baseline FAQ facts', (context) => {
  const errors = context.mock.method(console, 'error', () => {});
  const { t } = createI18n<[typeof messages], 'en', false>({ legacy: false, locale: 'en', messages: { en: messages } }).global;
  for (const [key, value] of leaves(messages.shell, 'shell')) {
    assert.ok(value.trim(), key);
    assert.notEqual(
      t(key, {
        label: 'Docs',
        name: 'context7',
        count: 2,
        manifest: 'plugin.json',
        reason: 'original',
      }),
      key,
    );
  }
  assert.equal(t('shell.hero.install', { name: 'context7' }), 'Install context7');
  assert.equal(
    t('shell.docs.englishLabel', { label: 'Quickstart' }),
    'Quickstart (documentation in English)',
  );
  for (const count of [0, 1, 2, 5, 11, 21, 22, 25, 101]) {
    assert.equal(
      t('shell.hero.exploreCount', { count }, count),
      `Explore ${count} ${count === 1 ? 'plugin' : 'plugins'}`,
    );
  }
  assert.deepEqual(
    Object.values((messages.shell!.faq as { items: Record<string, unknown> }).items),
    [...registryFaqItems],
  );
  assert.equal(errors.mock.callCount(), 0);
});

test('narrow overlay preserves every active English field and technical byte', () => {
  const actual = assembleDownloadContent(overlay);
  assert.deepEqual(actual.download, legacy.download);
  assert.deepEqual(
    actual.installChannels,
    legacy.installChannels.filter((channel: { id: string }) =>
      ['brew', 'npm', 'script', 'powershell'].includes(channel.id),
    ),
  );
  assert.deepEqual(actual.quickstartSteps, legacy.quickstartSteps);
  assert.deepEqual(
    Object.keys(overlay.shell.download.channels),
    downloadTechnical.installChannels.map((c) => c.id),
  );
  assert.deepEqual(
    Object.keys(overlay.shell.download.steps),
    downloadTechnical.quickstartSteps.map((c) => c.id),
  );
  for (const entry of Object.values(overlay.shell.download.channels)) {
    assert.deepEqual(Object.keys(entry).sort(), ['description', 'note', 'title']);
  }
  for (const entry of Object.values(overlay.shell.download.steps)) {
    assert.deepEqual(Object.keys(entry).sort(), ['note', 'title']);
  }
  const untrusted = structuredClone(overlay);
  Object.assign(untrusted.shell.download.channels.npm!, {
    command: 'changed',
    invocation: 'changed',
    href: 'https://invalid.test',
    id: 'changed',
    recommended: true,
  });
  assert.deepEqual(assembleDownloadContent(untrusted), actual);
  const loader = read('composables/useDownloadContent.ts');
  assert.ok(loader.includes('import(`../content/download/${language}.json`)'));
  for (const file of [
    'composables/useDownloadContent.ts',
    'data/download.ts',
    'components/sections/DownloadSection.vue',
  ]) {
    const source = read(file);
    assert.doesNotMatch(
      source,
      /useLandingContent|(?:from|import\()\s*['"](?:~\/|\.\.\/)data\/content|content\/(?:en|ru|es|fr|zh)\.json/,
    );
  }
});

test('owned docs routes preserve extensions and history; UK is explicitly English', () => {
  for (const id of Object.keys(docsAvailability) as Array<keyof typeof docsAvailability>) {
    for (const locale of ['en', 'ru', 'uk']) {
      const resolved = resolveDocsLink(id, locale);
      const language = locale === 'ru' ? 'ru' : 'en';
      assert.equal(resolved.url, `${ownedDocsRoot}${language}/${docsAvailability[id]}`);
      assert.equal(resolved.englishFallback, locale === 'uk');
      const path = docsAvailability[id].replace(/\.html$/, '.md') || 'index.md';
      assert.ok(
        existsSync(new URL(`../../website/source/${language}/${path}`, import.meta.url)),
        resolved.url,
      );
    }
  }
  for (const locale of ['en', 'ru', 'uk']) {
    const result = resolveDocsLink(
      'quickstart',
      locale,
      `${ownedDocsRoot}en/guide/quickstart.html?source=x#historical-v1`,
    );
    assert.ok(result.url.endsWith('.html?source=x#historical-v1'));
    const language = locale === 'ru' ? 'ru' : 'en';
    assert.ok(
      readFileSync(
        new URL(`../../website/source/${language}/guide/quickstart.md`, import.meta.url),
        'utf8',
      ).includes('{#historical-v1}'),
    );
  }
  for (const url of [
    'https://example.test/en/guide/quickstart.html#history',
    `${ownedDocsRoot}en/unknown.html`,
    `${ownedDocsRoot}en/guide/quickstart.html/extra`,
  ]) {
    assert.equal(resolveDocsLink('quickstart', 'ru', url).url, url);
  }
});

test('channel adapter retains manual IDs across locale remount and reconciles removed choices', async () => {
  // Execute the actual adapter with Nuxt auto-imports and lifecycle supplied by the harness.
  const source = read('composables/useInstallChannelSelection.ts')
    .replace(/import[\s\S]*?from\s*['"][^'"]+['"];\n/g, '')
    .replace('export function', 'function');
  const factory = new Function(
    'computed',
    'ref',
    'watch',
    'onMounted',
    'window',
    'useInstallPreferencesStore',
    'normalizeInstallPlatform',
    'recommendedInstallChannelId',
    `${stripTypeScriptTypes(source)}\nreturn useInstallChannelSelection;`,
  );
  const mounted: Array<() => void> = [];
  const useSelection = factory(
    computed,
    ref,
    watch,
    (callback: () => void) => mounted.push(callback),
    { navigator: { userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)' } },
    useInstallPreferencesStore,
    normalizeInstallPlatform,
    recommendedInstallChannelId,
  );
  setActivePinia(createPinia());
  const translated = ref(assembleDownloadContent(overlay).installChannels);
  const channels = computed(() => translated.value);
  const firstScope = effectScope();
  const first = firstScope.run(() => useSelection(channels))!;
  assert.equal(first.selectedInstallChannelId.value, 'brew');
  await mounted[0]!();
  first.detectedInstallPlatform.value = 'windows';
  await nextTick();
  assert.equal(first.selectedInstallChannelId.value, 'powershell');
  first.selectInstallChannel('npm');
  firstScope.stop();
  translated.value = translated.value.map((channel) => ({
    ...channel,
    title: `fixture ${channel.id}`,
  }));
  const secondScope = effectScope();
  const second = secondScope.run(() => useSelection(channels))!;
  await mounted[1]!();
  second.detectedInstallPlatform.value = 'linux';
  await nextTick();
  assert.equal(second.selectedInstallChannelId.value, 'npm');
  assert.equal(
    channels.value.find((c) => c.id === second.selectedInstallChannelId.value)?.title,
    'fixture npm',
  );
  second.selectInstallChannel('invalid');
  assert.equal(second.selectedInstallChannelId.value, 'npm');
  translated.value = translated.value.filter((c) => c.id !== 'npm');
  await nextTick();
  assert.equal(second.selectedInstallChannelId.value, 'script');
  second.detectedInstallPlatform.value = 'mobile';
  await nextTick();
  assert.equal(second.recommendedChannelId.value, null);
  secondScope.stop();
  assert.equal(mounted.length, 2); // Detection is scheduled, never run during SSR.
  assert.equal(useInstallPreferencesStore(createPinia()).channelId, null);
});

test('hero localizes structured fallback diagnostics and retains unknown original reasons', () => {
  const source = read('components/registry/RegistryHero.vue');
  const adapter = source.slice(
    source.indexOf('type ShellDiagnostic'),
    source.indexOf('const fallbackReason'),
  );
  const { t } = createI18n<[typeof messages], 'en', false>({ legacy: false, locale: 'en', messages: { en: messages } }).global;
  const render = new Function('t', `${stripTypeScriptTypes(adapter)}\nreturn diagnosticText;`)(t);
  assert.equal(
    render({
      code: 'defaultIneligible',
      params: { distribution: 'upstream' },
      reasons: [
        {
          code: 'reasons',
          reasons: [
            { code: 'releaseStatus', params: { sequence: 2, status: 'revoked' } },
            { code: 'unsupportedTargets', params: { sequence: 1, targets: 'codex,cursor' } },
          ],
        },
      ],
    }),
    'declared default upstream was ineligible: release 2 is revoked; release 1 does not support codex,cursor',
  );
  assert.equal(render({ code: 'future-code' }), null);
  assert.equal(render({ code: 'defaultIneligible', reasons: [{ code: 'future-code' }] }), null);
  assert.match(source, /reason: resolution\.value\.fallback_reason/);
  assert.doesNotMatch(source, /fallback_reason\.(?:match|replace|split|includes)/);
});

// The original migration-copy browser assertions also exercise these rows after generation.
test('download and client strip retain every original support qualification', () => {
  for (const file of ['components/registry/ClientStrip.vue', 'components/sections/DownloadSection.vue']) {
    const source = read(file);
    assert.match(source, /registryUi\.clients\.\$\{client.id\}\.status/);
    assert.doesNotMatch(source, /shell\.clients/);
  }
  const source = read('components/sections/DownloadSection.vue');
  for (const field of ['note', 'activation']) assert.ok(source.includes(`registryUi.clients.${'${client.id}'}.${field}`));
  assert.equal(clientLandingPages.length, 11);
});

test('shell agent links keep language prefixes and canonical trailing slashes', () => {
  for (const file of ['components/registry/ClientStrip.vue', 'components/sections/DownloadSection.vue']) {
    const expression = read(file).match(/const clientPath = ([\s\S]*?);/)![1];
    const locale = ref('en');
    const clientPath = new Function('locale', 'localizedPath', 'isKnownLocale',
      `${stripTypeScriptTypes(`const clientPath = ${expression};`)} return clientPath;`)(locale, localizedPath, isKnownLocale);
    for (const code of ['en', 'ru', 'uk']) {
      locale.value = code;
      assert.equal(clientPath('codex'), `${code === 'en' ? '' : '/' + code}/agents/codex/`);
    }
  }
});


test('visible FAQ and schema share the exact same localized page value', () => {
  const page = read('pages/index.vue');
  assert.match(page, /<RegistryFaq :items="registryFaqItems"/);
  assert.match(page, /mainEntity: registryFaqItems.value.map/);
  assert.match(read('components/registry/RegistryFaq.vue'), /v-for="item in items"/);
  const { tm, rt } = createI18n<[typeof messages], 'en', false>({ legacy: false, locale: 'en', messages: { en: messages } }).global;
  assert.deepEqual(Object.values(tm('shell.faq.items') as Record<string, { question: string; answer: string }>).map((item) => ({
    question: rt(item.question), answer: rt(item.answer),
  })), [...registryFaqItems]);
});

test('active shared controls have English reference labels', () => {
  const { t } = createI18n<[typeof messages], 'en', false>({ legacy: false, locale: 'en', messages: { en: messages } }).global;
  for (const key of ['language.switchError', 'language.retry', 'theme.light', 'theme.dark',
    'download.copy', 'download.copied', 'footer.tagline', 'shell.accessibility.license']) {
    assert.notEqual(t(key), key);
  }
  const header = read('components/layout/AppHeader.vue');
  assert.match(header, /const navItems = computed/);
  assert.match(header, /publishedLocales.length > 1/);
  for (const key of Object.keys(messages.shell!.navigation as object)) {
    assert.ok(header.includes(`shell.navigation.${key}`), key);
  }
});


test('download overlay loader awaits available languages and safely falls back when absent', async () => {
  const source = read('composables/useDownloadContent.ts');
  const loadSource = source.slice(source.indexOf('  const load ='), source.indexOf('  const { data }'));
  const executable = stripTypeScriptTypes(loadSource).replace(
    "(await import('../content/download/en.json')).default", 'english',
  ).replace('import(`../content/download/${language}.json`)', 'importOverlay(language)');
  const available = { '../content/download/en.json': async () => overlay };
  const load = new Function('importOverlay', 'english', `${executable} return load;`)(async (language: string) => {
    const loader = (available as Record<string, () => Promise<DownloadOverlay>>)[`../content/download/${language}.json`];
    if (!loader) throw new Error('Missing overlay');
    return { default: await loader() };
  }, overlay);
  assert.deepEqual(await load('en'), overlay);
  assert.deepEqual(await load('uk'), overlay);
  assert.deepEqual(await load('ru'), overlay);
  const fixture = structuredClone(overlay);
  fixture.shell.download.heading.title = 'Future overlay fixture';
  Object.assign(available, { '../content/download/uk.json': async () => fixture });
  assert.equal((await load('uk')).shell.download.heading.title, 'Future overlay fixture');
  assert.deepEqual(await load('es'), overlay);
});
