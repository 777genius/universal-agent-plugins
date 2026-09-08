import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { test } from 'node:test';
import { runInNewContext } from 'node:vm';
import * as Vue from 'vue';
import { parse, compileScript } from 'vue/compiler-sfc';
import { computed, effectScope, nextTick, reactive, ref, watch } from 'vue';
import { createPinia, setActivePinia } from 'pinia';
import { createI18n, type LocaleMessageDictionary, type VueMessageType } from 'vue-i18n';
import { useInstallPreferencesStore } from '../stores/installPreferences.ts';
import { useCatalogUiStore } from '../stores/catalogUi.ts';
import { clients } from '../data/clients.ts';
import * as registryDomain from '../utils/registry.ts';
import * as filters from '../utils/filter.ts';
import { pluginCommands } from '../utils/commands.ts';

const fixture = JSON.parse(
  readFileSync(new URL('./fixtures/registry-responses/gitlab.json', import.meta.url), 'utf8'),
).plugins[0];
const messages = JSON.parse(
  readFileSync(new URL('../locales/en.json', import.meta.url), 'utf8'),
);
const i18n = createI18n<[LocaleMessageDictionary<VueMessageType>], string, false>({ legacy: false, locale: 'en', messages: { en: messages } }).global;
const common = {
  computed,
  ref,
  watch,
  clients,
  ...registryDomain,
  ...filters,
  pluginCommands,
  useI18n: () => i18n,
  useLocalePath: () => (path: string) => path,
  useInstallPreferencesStore,
  useCatalogUiStore,
  useSite: () => ({
    asset: (path: string) => path,
    pluginIcon: () => '',
    sourceUrl: () => '',
    repositoryUrl: '',
  }),
  useDirectoryStatus: () => ({ current: ref(true), published: ref(true), expired: ref(false) }),
};
function setup(name: string, globals: object, returns: string) {
  const source = readFileSync(
    new URL(`../components/registry/${name}.vue`, import.meta.url),
    'utf8',
  )
    .match(/<script setup lang="ts">([\s\S]*?)<\/script>/)![1]
    .replace(/^import[\s\S]*?from '[^']+';\n/gm, '');
  const scope = effectScope();
  const state = scope.run(() =>
    runInNewContext(stripTypeScriptTypes(source) + `\n({ ${returns} });`, {
      ...common,
      ...globals,
    }),
  );
  return { state, stop: () => scope.stop() };
}
const packageProps = (plugin = fixture) =>
  reactive({ plugin: structuredClone(plugin), alternatives: [] });
function card(props = packageProps()) {
  return setup(
    'RegistryPluginCard',
    { defineProps: () => props, withDefaults: (value: unknown) => value },
    'targets, autoDetect, installExpanded, command, updateTargets, updateAutoDetect, showAuthentication, authLabel, toggleInstall',
  );
}

test('catalog SSR defaults do not populate install preferences; valid manual selection survives locale remount', async () => {
  setActivePinia(createPinia());
  const store = useInstallPreferencesStore();
  const before = JSON.stringify(store.$state);
  const cards = Array.from({ length: 40 }, (_, i) =>
    card(packageProps({ ...fixture, install_source: `fixture:${i}` })),
  );
  assert.equal(JSON.stringify(store.$state), before);
  cards.forEach((instance) => instance.stop());
  const first = card();
  first.state.updateTargets(['codex']);
  first.state.updateAutoDetect(false);
  first.state.toggleInstall();
  await nextTick();
  const command = first.state.command.value;
  first.stop();
  const remount = card();
  assert.deepEqual(Array.from(remount.state.targets.value), ['codex']);
  assert.equal(remount.state.autoDetect.value, false);
  assert.equal(remount.state.installExpanded.value, true);
  assert.equal(remount.state.command.value, command);
  remount.stop();
});

test('invalid target preferences are discarded and a new package does not inherit another package selection', async () => {
  setActivePinia(createPinia());
  const first = card();
  first.state.updateTargets(['codex']);
  first.state.updateAutoDetect(false);
  await nextTick();
  first.stop();
  const changed = card(
    packageProps({
      ...fixture,
      client_support: { ...fixture.client_support, clients: ['cursor'] },
    }),
  );
  assert.deepEqual(Array.from(changed.state.targets.value), ['cursor']);
  assert.equal(changed.state.autoDetect.value, true);
  changed.stop();
  const other = card(packageProps({ ...fixture, install_source: 'different' }));
  assert.equal(other.state.autoDetect.value, true);
  other.stop();
});

test('bound detail v-model persists picker event order and exact commands across remounts', async () => {
  const pinia = createPinia();
  setActivePinia(pinia);
  const store = useInstallPreferencesStore();
  const source = readFileSync(new URL('../components/registry/InstallPanel.vue', import.meta.url), 'utf8')
    .replace('</script>', 'defineExpose({ commands, updateTargets, updateAutoDetect });</script>');
  const { descriptor } = parse(source);
  const compiled = compileScript(descriptor, { id: 'install-panel-regression' }).content
    .replace(/import \{([^}]+)\} from ["']vue["'];?/g, (_line, names: string) =>
      'const {' + names.replace(/ as /g, ': ') + '} = Vue;')
    .replace(/^import[\s\S]*?from '[^']+';\n/gm, '')
    .replace('export default', 'const Component =');
  const Child = runInNewContext(stripTypeScriptTypes(compiled) + '\nComponent', { ...common, Vue });
  Child.render = () => null;
  // A real Vue renderer runs parent render updates, child props and useModel emissions.
  const renderer = Vue.createRenderer<object, object>({
    patchProp() {}, insert() {}, remove() {}, createElement: () => ({}),
    createText: () => ({}), createComment: () => ({}), setText() {}, setElementText() {},
    parentNode: () => null, nextSibling: () => null,
  });
  async function mount() {
    const targets = ref(['cursor']);
    const autoDetect = ref(true);
    const panel = ref();
    const app = renderer.createApp({
      setup: () => () => Vue.h(Child, {
        ref: panel, plugin: structuredClone(fixture), targets: targets.value,
        autoDetect: autoDetect.value,
        'onUpdate:targets': (value: string[]) => { targets.value = value; },
        'onUpdate:autoDetect': (value: boolean) => { autoDetect.value = value; },
      }),
    });
    app.use(pinia);
    app.mount({});
    await nextTick();
    return { panel, targets, autoDetect, stop: () => app.unmount() };
  }
  let current = await mount();
  assert.equal(store.package.identity, '');
  for (const target of ['codex', 'cursor']) {
    // AppMultiSelect leaves automatic mode BEFORE emitting the chosen targets.
    current.panel.value.updateAutoDetect(false);
    current.panel.value.updateTargets([target]);
    await nextTick();
    assert.deepEqual(Array.from(store.package.targetIds), [target]);
    assert.equal(store.package.autoDetect, false);
    const commands = JSON.stringify(current.panel.value.commands);
    current.stop();
    current = await mount();
    assert.deepEqual(Array.from(current.targets.value), [target]);
    assert.equal(current.autoDetect.value, false);
    assert.equal(JSON.stringify(current.panel.value.commands), commands);
  }
  current.panel.value.updateAutoDetect(true);
  await nextTick();
  const automatic = JSON.stringify(current.panel.value.commands);
  assert.equal(store.package.autoDetect, true);
  current.stop();
  current = await mount();
  assert.equal(current.autoDetect.value, true);
  assert.equal(JSON.stringify(current.panel.value.commands), automatic);
  current.stop();
});

test('catalog Show more survives language remount and filter changes replace its fingerprint while URL remains owner', async () => {
  setActivePinia(createPinia());
  const store = useCatalogUiStore();
  const route = reactive({
    path: '/plugins/',
    query: {} as Record<string, string>,
    hash: '#plugins',
  });
  const navigations: any[] = [];
  const mount = () =>
    setup(
      'PluginCatalog',
      {
        defineProps: () => ({ plugins: [fixture] }),
        withDefaults: (value: unknown) => value,
        useRoute: () => route,
        useRouter: () => ({
          replace: (target: any) => {
            navigations.push(target);
            route.query = target.query;
          },
        }),
        useDiscoveryStatus: () => ref({ state: 'idle' }),
        canonicalPath: (path: string) => path,
      },
      'displayLimit, showMore, query, owner',
    );
  const first = mount();
  assert.equal(store.home, null);
  assert.equal(store.catalog, null);
  first.state.showMore();
  assert.equal(first.state.displayLimit.value, 96);
  first.stop();
  route.path = '/uk/plugins/';
  const remount = mount();
  assert.equal(remount.state.displayLimit.value, 96);
  remount.state.query.value = 'gitlab';
  await nextTick();
  assert.equal(remount.state.displayLimit.value, 48);
  assert.equal(route.query.q, 'gitlab');
  assert.equal(navigations[0].hash, '#plugins');
  remount.stop();
  const again = mount();
  assert.equal(again.state.query.value, 'gitlab');
  assert.equal(again.state.displayLimit.value, 48);
  again.state.showMore();
  again.stop();
  route.query = { q: 'different' };
  const changedInitial = mount();
  assert.equal(changedInitial.state.displayLimit.value, 48);
  changedInitial.state.query.value = 'gitlab';
  await nextTick();
  assert.equal(changedInitial.state.displayLimit.value, 48);
  changedInitial.stop();
});


test('distribution captions use stable kind and authentication visibility ignores translated wording', () => {
  setActivePinia(createPinia());
  const props = packageProps();
  for (const distribution of props.plugin.distributions) distribution.label = 'UNTRANSLATED DOMAIN LABEL';
  const panel = setup('InstallPanel', {
    defineProps: () => props,
    defineModel: (name: string) => name === 'targets' ? ref(['codex']) : ref(false),
  }, 'expectedSource, expectedSourceLabel');
  assert.ok(panel.state.expectedSource.value);
  assert.equal(panel.state.expectedSourceLabel.value, i18n.t(`registryUi.distribution.${panel.state.expectedSource.value.kind}`));
  assert.notEqual(panel.state.expectedSourceLabel.value, 'UNTRANSLATED DOMAIN LABEL');
  panel.stop();

  const instance = card();
  instance.state.updateTargets(['codex']);
  instance.state.updateAutoDetect(false);
  const visible = instance.state.showAuthentication.value;
  i18n.mergeLocaleMessage('en', { registryUi: { authentication: Object.fromEntries(
    ['not_required', 'required', 'unknown', 'varies', 'oauth', 'client_managed'].map(key => [key, 'SYNTHETIC CAPTION']),
  ) } });
  assert.equal(instance.state.authLabel.value, 'SYNTHETIC CAPTION');
  assert.equal(instance.state.showAuthentication.value, visible);
  i18n.setLocaleMessage('en', messages);
  instance.stop();
});
