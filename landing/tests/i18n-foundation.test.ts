import assert from 'node:assert/strict';
import { test } from 'node:test';
import { createPinia, setActivePinia } from 'pinia';
import { publishedLocales, candidateLocales, isPublishedLocale, isKnownLocale } from '../data/i18n.ts';
import { productRoutes } from '../data/routes.ts';
import { expandLocalizedRoutes, isIndexablePath, withAppBase } from '../utils/localizedRoutes.ts';
import { useLocaleStore } from '../stores/locale.ts';
import { useInstallPreferencesStore } from '../stores/installPreferences.ts';
import { useCatalogUiStore } from '../stores/catalogUi.ts';
import { readManualLocaleCookie, useLocation, switchLocaleTransaction } from '../composables/useLocation.ts';

test('production EN only, candidates explicit, legacy drafts known but unpublished', () => {
  assert.deepEqual(publishedLocales, ['en']);
  for (const code of ['ru', 'uk', 'es', 'fr', 'zh']) {
    assert.equal(isKnownLocale(code), true);
    assert.equal(isPublishedLocale(code), false);
  }
  for (const code of ['EN', 'ua', '__proto__', '', null]) assert.equal(isKnownLocale(code), false);
});
test('HTML manifest expansion, exclusions, collisions and base boundaries', () => {
  const routes = productRoutes(['gitlab'], ['codex', 'github-copilot-cli']);
  assert.equal(expandLocalizedRoutes(routes).length, 8);
  const expanded = expandLocalizedRoutes(routes, candidateLocales);
  assert.equal(expanded.length, 24);
  assert.equal(expanded.filter(r => r.indexable).length, 18);
  assert.ok(expanded.some(r => r.path === '/uk/plugins/gitlab/'));
  for (const path of ['/agents/', '/api/registry/catalog/', '/plugins/unknown/', '/ru/', '/create-plugin/', '/plugins/community/?source=x']) {
    assert.equal(isIndexablePath(path, routes), false, path);
  }
  assert.equal(isIndexablePath('/uk/plugins/gitlab/?x=y#security', routes, candidateLocales), true);
  assert.throws(() => productRoutes(['community'], []));
  assert.throws(() => productRoutes(['x', 'x'], []));
  assert.throws(() => productRoutes(['../x'], []));
  assert.equal(withAppBase('/uk/download/?x=y#install', '/universal-agent-plugins/'), '/universal-agent-plugins/uk/download/?x=y#install');
  assert.equal(withAppBase('/universal-agent-plugins/uk/', '/universal-agent-plugins/'), '/universal-agent-plugins/uk/');
  assert.equal(withAppBase('/universal-agent-plugins-other/', '/universal-agent-plugins/'), '/universal-agent-plugins/universal-agent-plugins-other/');
  assert.equal(withAppBase('/uk/', '/'), '/uk/');
});
test('Pinia request isolation and preferences do not own route state', () => {
  const one = useLocaleStore(createPinia());
  const two = useLocaleStore(createPinia());
  one.rememberChoice('en');
  one.rememberChoice('ru');
  assert.equal(one.preferredLocale, 'en');
  assert.equal(two.preferredLocale, null);
  assert.equal('current' in one, false);
});
test('bounded UI selections survive remount and reject stale identities/IDs', () => {
  const pinia = createPinia();
  const install = useInstallPreferencesStore(pinia);
  install.selectChannel('brew', ['brew', 'npm']);
  install.selectChannel('npm', ['brew', 'npm'], false);
  assert.equal(install.channelId, 'brew');
  const choice = { targetIds: ['codex'], autoDetect: false, expanded: true };
  install.selectPackage('gitlab@source', choice, ['codex']);
  assert.deepEqual(useInstallPreferencesStore(pinia).readPackage('gitlab@source', ['codex']).targetIds, ['codex']);
  assert.deepEqual(install.readPackage('other@source', ['codex']).targetIds, []);
  install.reconcilePackage('gitlab@source', ['cursor']);
  assert.equal(install.package.identity, '');
  install.reconcileChannels(['npm']);
  assert.equal(install.channelId, null);
  const catalog = useCatalogUiStore(pinia);
  assert.equal(catalog.displayLimit('home', 'q=one', 12), 12);
  assert.equal(catalog.home, null);
  catalog.showMore('home', 'q=one', 24);
  assert.equal(useCatalogUiStore(pinia).displayLimit('home', 'q=one', 12), 24);
  assert.equal(catalog.displayLimit('catalog', 'q=one', 12), 12);
  catalog.reconcile('home', 'q=two');
  assert.equal(catalog.home, null);
  assert.equal(useInstallPreferencesStore(createPinia()).channelId, null);
});
function fixture() {
  let active = 'en';
  let remembered = '';
  let writes = 0;
  let calls = 0;
  const adapter = {
    valid: (code: unknown): code is string => typeof code === 'string' && (candidateLocales as readonly string[]).includes(code),
    active: () => active,
    pending: { value: false }, error: { value: false },
    destination: (code: string) => `/${code}/plugins/gitlab/?source=a%2Fb&target=codex&target=cursor#security`,
    navigate: async (path: string): Promise<unknown> => { calls++; assert.ok(path.endsWith('?source=a%2Fb&target=codex&target=cursor#security')); active = 'uk'; return undefined; },
    remember: (code: string) => { remembered = code; },
    persist: () => { writes++; },
    track: () => {},
  };
  return { adapter, state: () => ({ active, remembered, writes, calls }) };
}
test('one successful navigation commits manual preference, query and hash', async () => {
  const f = fixture();
  assert.equal(await switchLocaleTransaction('uk', f.adapter), true);
  assert.deepEqual(f.state(), { active: 'uk', remembered: 'uk', writes: 1, calls: 1 });
});
test('navigation failures and wrong resolved locale never persist; retry is explicit', async () => {
  for (const navigate of [async () => { throw new Error('chunk failed'); }, async () => ({ type: 4 }), async () => undefined]) {
    const f = fixture(); f.adapter.navigate = navigate;
    assert.equal(await switchLocaleTransaction('uk', f.adapter), false);
    assert.equal(f.state().remembered, ''); assert.equal(f.state().writes, 0);
    assert.equal(f.adapter.pending.value, false); assert.equal(f.adapter.error.value, true);
  }
});
test('pending transitions serialize across callers; denied storage/analytics are isolated', async () => {
  const f = fixture();
  const navigate = f.adapter.navigate;
  let finish!: () => void;
  f.adapter.navigate = async path => { await new Promise<void>(resolve => { finish = resolve; }); return navigate(path); };
  f.adapter.persist = () => { throw new Error('denied'); };
  f.adapter.track = () => { throw new Error('analytics'); };
  const first = switchLocaleTransaction('uk', f.adapter);
  assert.equal(await switchLocaleTransaction('ru', f.adapter), false);
  finish(); assert.equal(await first, true);
  assert.equal(f.state().calls, 1); assert.equal(f.state().remembered, 'uk');
});

test('Nuxt adapter preserves router query arrays/hash, confirms route and scopes successful cookie', async () => {
  const { createRouter, createMemoryHistory } = await import('vue-router');
  const { ref } = await import('vue');
  const router = createRouter({ history: createMemoryHistory('/universal-agent-plugins/'), routes: [
    { path: '/plugins/:slug/', component: {} }, { path: '/ru/plugins/:slug/', component: {} },
  ] });
  await router.push('/ru/plugins/gitlab/?source=a%2Fb&target=codex&target=cursor#security');
  const active = ref('ru');
  const states = new Map<string, ReturnType<typeof ref>>();
  let calls = 0;
  let cookie = '';
  const globals = {
    useNuxtApp: () => ({ $i18n: { locale: active } }),
    useRoute: () => router.currentRoute.value,
    useRouter: () => router,
    useSwitchLocalePath: () => () => '/plugins/gitlab/',
    useRuntimeConfig: () => ({ app: { baseURL: '/universal-agent-plugins/' } }),
    useState: (key: string, init: () => boolean) => {
      if (!states.has(key)) states.set(key, ref(init()));
      return states.get(key);
    },
    useAnalytics: () => ({ trackLanguageSwitch: () => {} }),
    navigateTo: async (path: string) => { calls++; const result = await router.push(path); active.value = 'en'; return result; },
    document: { get cookie() { return cookie; }, set cookie(value: string) { cookie = value; } },
    location: { protocol: 'https:' },
  };
  const previous = new Map(Object.keys(globals).map(key => [key, Object.getOwnPropertyDescriptor(globalThis, key)]));
  try {
    for (const [key, value] of Object.entries(globals)) Object.defineProperty(globalThis, key, { value, configurable: true });
    setActivePinia(createPinia());
    const adapter = useLocation();
    assert.equal(await adapter.switchLocale('en'), true);
    assert.equal(calls, 1);
    assert.equal(router.currentRoute.value.params.slug, 'gitlab');
    assert.deepEqual(router.currentRoute.value.query, { source: 'a/b', target: ['codex', 'cursor'] });
    assert.equal(router.currentRoute.value.hash, '#security');
    assert.match(cookie, /^uap_locale=en; Path=\/universal-agent-plugins\/; Max-Age=31536000; SameSite=Lax; Secure$/);
    await router.back();
    assert.equal(useLocaleStore().preferredLocale, 'en');
  } finally {
    for (const [key, descriptor] of previous) {
      if (descriptor) Object.defineProperty(globalThis, key, descriptor);
      else Reflect.deleteProperty(globalThis, key);
    }
  }
});


test('cookie metadata ignores old, malformed and unpublished values and storage denial', () => {
  assert.equal(readManualLocaleCookie(() => 'other=x;uap_locale=en'), 'en');
  for (const cookie of ['i18n_redirected=en', 'uap_locale=ru', 'uap_locale=uk', 'uap_locale=es', 'uap_locale=%E0%A4%A', 'uap_locale=EN', '']) {
    assert.equal(readManualLocaleCookie(() => cookie), null);
  }
  assert.equal(readManualLocaleCookie(() => { throw new Error('denied'); }), null);
});
