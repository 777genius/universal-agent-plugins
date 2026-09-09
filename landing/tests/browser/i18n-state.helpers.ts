import { readFileSync } from 'node:fs';
import { expect, type Page, type Locator } from '@playwright/test';
import { createI18n, type LocaleMessageDictionary, type VueMessageType } from 'vue-i18n';
import { publishedLocales, type PublishedLocale } from '../../data/i18n';
import { localizedPath } from '../../utils/localizedRoutes';
import type { DownloadOverlay } from '../../types/download';
import { slavicPluralRule } from '../../utils/slavicPluralRule';
import type { RegistryIndex } from '../../types/registry';

export const locales: readonly PublishedLocale[] = publishedLocales;
export function wording(locale: PublishedLocale) {
  const messages = JSON.parse(readFileSync(new URL(`../../locales/${locale}.json`, import.meta.url), 'utf8'));
  const i18n = createI18n<[LocaleMessageDictionary<VueMessageType>], PublishedLocale, false>({ legacy: false, locale, fallbackLocale: false, pluralRules: { ru: slavicPluralRule, uk: slavicPluralRule }, messages: Object.fromEntries(publishedLocales.map(code => [code, code === locale ? messages : {}])) as Record<PublishedLocale, LocaleMessageDictionary<VueMessageType>> });
  return (key: string, params: Record<string, string | number> = {}) => {
    expect(i18n.global.te(key), `${locale}: missing ${key}`).toBe(true);
    return i18n.global.t(key, params);
  };
}

export function downloadHeading(locale: PublishedLocale) {
  const overlay: DownloadOverlay = JSON.parse(readFileSync(new URL(`../../content/download/${locale}.json`, import.meta.url), 'utf8'));
  return overlay.shell.download.heading.title;
}

export function downloadChannelTitle(locale: PublishedLocale, channel: string) {
  const overlay: DownloadOverlay = JSON.parse(readFileSync(new URL(`../../content/download/${locale}.json`, import.meta.url), 'utf8'));
  return overlay.shell.download.channels[channel]!.title;
}

export async function hydrated(page: Page) {
  await page.waitForFunction(() => {
    const root = document.querySelector('#__nuxt') as HTMLElement & {
      __vue_app__?: { config: { globalProperties: { $nuxt?: { isHydrating: boolean } } } };
    };
    return root?.__vue_app__?.config.globalProperties.$nuxt?.isHydrating === false;
  });
}

export async function navigate(page: Page, family: string, locale: PublishedLocale = 'en', suffix = '') {
  await hydrated(page);
  await page.waitForFunction(() => Boolean((document.querySelector('#__nuxt') as HTMLElement & {
    __vue_app__?: { config: { globalProperties: { $router?: unknown } } };
  })?.__vue_app__?.config.globalProperties.$router));
  // Only access the installed router, never component refs, stores, or async data.
  await page.evaluate(async (path) => {
    const root = document.querySelector('#__nuxt') as HTMLElement & {
      __vue_app__: { config: { globalProperties: { $router: { push: (path: string) => Promise<unknown> } } } };
    };
    await root.__vue_app__.config.globalProperties.$router.push(path);
  }, localizedPath(family, locale) + suffix);
}

export function deferred() {
  let release!: () => void;
  const promise = new Promise<void>((resolve) => { release = resolve; });
  return { promise, release };
}

export async function offline(page: Page) {
  const errors: string[] = [];
  page.on('pageerror', (error) => errors.push(error.message));
  page.on('console', (message) => {
    if (/hydration.*(mismatch|failed)/i.test(message.text())) errors.push(message.text());
  });
  await page.addInitScript(() => {
    let copied = '';
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: {
      writeText: async (text: string) => { copied = text; }, readText: async () => copied,
    } });
  });
  // Same release schema as migration-copy; every other non-local request is blocked.
  await page.route('**/*', (route) => {
    const url = new URL(route.request().url());
    if (url.hostname === '127.0.0.1' || url.hostname === 'localhost') return route.fallback();
    if (url.hostname === 'api.github.com' && url.pathname.endsWith('/releases/latest'))
      return route.fulfill({ json: { tag_name: 'agentplugins-v1.2.3', published_at: '2026-09-01T00:00:00Z', assets: [] } });
    return route.abort();
  });
  return errors;
}

export function registryFixture(): RegistryIndex {
  const index: RegistryIndex = JSON.parse(readFileSync(new URL('../fixtures/registry-responses/gitlab.json', import.meta.url), 'utf8'));
  // Synthetic *projection* fixtures, not signed Directory/security evidence.
  // Derive full nested distributions/releases from the existing schema-correct fixture.
  index.expires_at = '2099-10-06T04:35:40Z';
  const seed = index.plugins[0]!;
  index.plugins = Array.from({ length: 60 }, (_, i) => {
    const plugin = structuredClone(seed);
    plugin.name = `i18n-state-${String(i).padStart(2, '0')}`;
    plugin.display_name = `I18n State ${String(i).padStart(2, '0')}`;
    plugin.install_source = plugin.name;
    plugin.description = 'Deterministic state regression package';
    plugin.source.path = `plugins/${plugin.name}`;
    plugin.categories = [i < 54 ? 'state-large' : 'state-small'];
    plugin.default_distribution = `777genius/${plugin.name}`;
    plugin.declared_default_distribution = plugin.default_distribution;
    for (const distribution of plugin.distributions) {
      distribution.id = plugin.default_distribution;
      distribution.source.path = plugin.source.path;
      for (const release of distribution.releases) release.source.path = plugin.source.path;
      for (const target of [...distribution.targets, ...distribution.releases.flatMap((release) => release.targets)])
        target.authentication = target.client === 'codex' ? 'not_required' : 'required';
    }
    return plugin;
  });
  return index;
}

export async function projections(page: Page, index: RegistryIndex) {
  // Enter through a hydrated non-registry page. Decline extracted payloads so real
  // Nuxt useAsyncData fetches the intercepted, locale-neutral projection endpoints.
  await page.route('**/api/registry/**', (route) => {
    const endpoint = new URL(route.request().url()).pathname;
    const slug = endpoint.split('/plugin/')[1];
    return route.fulfill({ json: { ...index, plugins: endpoint.endsWith('/empty') ? [] : slug ? index.plugins.filter((p) => p.name === slug) : index.plugins } });
  });
  await page.route('**/discovery/**', (route) => route.abort());
  await page.route('**/security/**', (route) => route.abort());
  await page.goto('./download/');
  await expect(page.getByRole('heading', { name: downloadHeading('en'), exact: true })).toBeVisible();
  await hydrated(page);
  await page.route('**/_payload.json*', (route) => route.fulfill({ status: 404, body: '' }));
}

export async function targets(page: Page, scope: Locator, names: string[]) {
  const trigger = scope.locator('.app-multiselect__trigger');
  await expect(trigger).toHaveAttribute('data-hydrated', 'true');
  await trigger.click();
  for (const name of names) await page.getByRole('checkbox', { name: new RegExp(`^${name}(?:\\s|$)`) }).click();
  await page.keyboard.press('Escape');
}

export async function automatic(page: Page, scope: Locator, locale: PublishedLocale) {
  await scope.locator('.app-multiselect__trigger').click();
  const auto = page.locator('.app-multiselect__auto');
  await expect(auto).toContainText(wording(locale)('registryUi.install.allInstalledAgentsRecommended'));
  await auto.click();
  await expect(auto).toHaveAttribute('aria-pressed', 'true');
  await page.keyboard.press('Escape');
}

export async function exactCommands(page: Page, scope: Locator, name: string, target = '') {
  for (const verb of ['add', 'update', 'repair', 'remove']) {
    const snippet = scope.locator(`.command-snippet--${verb}`);
    const expected = `npx universal-agent-plugins ${verb} ${name}${target ? ` --target ${target}` : ''}`;
    await expect(snippet.locator('code')).toHaveText(expected);
    await snippet.getByRole('button').click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(expected);
  }
}

// Preserve real signed mirror bytes and production trust verification. No fabricated signatures.
export async function delayedMirror(page: Page, kind: 'discovery' | 'security', fail = false) {
  const gate = deferred();
  const seen = deferred();
  const delivered = deferred();
  await page.route(`**/${kind}/latest.json`, async (route) => {
    seen.release();
    await gate.promise;
    if (fail) await route.fulfill({ status: 503, body: 'i18n-state delayed failure' });
    else {
      const response = await route.fetch(); // local static artifact only
      expect(response.status(), `${kind} mirror must exist in final artifact`).toBe(200);
      await route.fulfill({ response });
    }
    delivered.release();
  });
  return { ...gate, seen: seen.promise, delivered: delivered.promise };
}
