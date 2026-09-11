import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { createI18n, type LocaleMessageDictionary, type VueMessageType } from 'vue-i18n';
import {
  securityAssessmentLabel,
  securityAssessmentHeading,
  securityAssessmentTooltip,
  formatSecurityDate,
} from '../utils/securityPresentation.ts';
import type { RegistryPlugin } from '../types/registry.ts';

const messages = JSON.parse(
  readFileSync(new URL('../locales/en.json', import.meta.url), 'utf8'),
);
const { t } = createI18n<[LocaleMessageDictionary<VueMessageType>], string, false>({
  legacy: false,
  locale: 'en',
  fallbackLocale: false,
  messages: { en: messages },
}).global;
const translate = (key: string, params: Record<string, string | number> = {}, plural?: number) =>
  plural === undefined ? t(key, params) : t(key, params, plural);

test('registry English messages compile and interpolate every declared placeholder', (context) => {
  const errors = context.mock.method(console, 'error', () => {});
  function visit(value: unknown, path: string) {
    if (typeof value === 'object' && value) {
      for (const [key, child] of Object.entries(value)) visit(child, `${path}.${key}`);
      return;
    }
    assert.equal(typeof value, 'string', path);
    const source = value as string;
    const params = Object.fromEntries(
      [...source.matchAll(/\{(\w+)\}/g)].map((match) => [
        match[1],
        match[1] === 'count' ? 22 : 'EXAMPLE',
      ]),
    );
    const rendered = t(path, params, 22);
    assert.notEqual(rendered, path);
    assert.ok(!/\{\w+\}/.test(rendered), path);
  }
  visit(messages.registryUi, 'registryUi');
  assert.equal(errors.mock.callCount(), 0);
});

test('localized security presentation preserves English outcomes and original finding text', () => {
  const plugin = {
    source: { revision: 'abcdef1234567890' },
    security: {
      scanner: { version: '1.0' },
      counts: { blocking: 0, warnings: 1, total: 1 },
      findings: [{ code: 'SEC324', message: 'Original external finding', path: 'workflow.yml' }],
    },
  } as RegistryPlugin;
  for (const count of [0, 1, 2, 5, 11, 21, 22, 25, 101]) {
    plugin.security!.counts.warnings = count;
    assert.equal(
      securityAssessmentLabel(plugin.security!, translate),
      securityAssessmentLabel(plugin.security!),
    );
    assert.equal(
      securityAssessmentHeading(plugin.security!, translate),
      securityAssessmentHeading(plugin.security!),
    );
    plugin.security!.counts.blocking = count;
    assert.equal(
      securityAssessmentLabel(plugin.security!, translate),
      securityAssessmentLabel(plugin.security!),
    );
    plugin.security!.counts.blocking = 0;
  }
  assert.deepEqual(securityAssessmentTooltip(plugin, translate), securityAssessmentTooltip(plugin));
  assert.equal(
    formatSecurityDate('2026-09-08T23:30:00-04:00', 'en'),
    new Intl.DateTimeFormat('en', {
      day: 'numeric',
      month: 'short',
      year: 'numeric',
      timeZone: 'UTC',
    }).format(new Date('2026-09-09T03:30:00Z')),
  );
});

test('all owned presentation keys exist and technical client content remains the English source', async () => {
  const { clientLandingPages, clients, clientPresentationKey } = await import('../data/clients.ts');
  for (const client of clients) {
    assert.equal(t(`${clientPresentationKey(client.id)}.note`), client.note);
    assert.equal(t(`${clientPresentationKey(client.id)}.status`), client.status);
  }
  for (const client of clientLandingPages) {
    for (const field of ['note', 'status', 'intro', 'delivery', 'activation'] as const) {
      assert.equal(t(`${client.presentationKey}.${field}`), client[field]);
    }
  }
  assert.equal(t('registryUi.directoryPage.title'), 'Agent Plugins 1.0 Directory | Search 2,500+ Plugins');
  assert.equal(t('registryUi.detail.title', { name: 'GitLab' }), 'GitLab Agent Plugin | Universal Agent Plugins');
  assert.equal(t('registryUi.agentPage.tryPlugin', { source: 'context7' }), "Try Context7, or replace context7 with another compatible plugin's reviewed short name or pinned GitHub package source.");
  for (const count of [0, 1, 2, 5, 11, 21, 22, 25, 101]) {
    assert.equal(t('registryUi.multi.agents', count), `${count} agent${count === 1 ? '' : 's'}`);
    assert.equal(t('registryUi.directory.communityCount', count), `${count} community package${count === 1 ? '' : 's'}`);
  }
});

test('owned Vue presentation compiles with no untranslated text or missing static keys', async () => {
  const { parse, compileScript, compileTemplate } = await import('vue/compiler-sfc');
  const files = ['AppSelect', 'AppCombobox', 'AppMultiSelect', 'CommandSnippet', 'RegistryDirectory', 'PluginCatalog', 'RegistryPluginCard', 'InstallPanel', 'SecurityAssessmentBadge', 'SecurityAssessmentPanel']
    .map(name => `components/registry/${name}.vue`)
    .concat(['pages/plugins/index.vue', 'pages/plugins/[slug].vue', 'pages/plugins/community.vue', 'pages/agents/[client].vue']);
  for (const filename of files) {
    const source = readFileSync(new URL(`../${filename}`, import.meta.url), 'utf8');
    const { descriptor, errors } = parse(source, { filename });
    assert.deepEqual(errors, [], filename);
    const script = compileScript(descriptor, { id: filename });
    const template = compileTemplate({ source: descriptor.template!.content, filename, id: filename, compilerOptions: { bindingMetadata: script.bindings } });
    assert.deepEqual(template.errors, [], filename);
    for (const match of source.matchAll(/(?:t\('|(keypath)="?)(registryUi\.[\w.]+)/g)) {
      const key = match[2];
      let value = messages;
      for (const part of key.split('.')) value = value?.[part];
      assert.equal(typeof value, 'string', `${filename}: ${key}`);
    }
    function visit(node: any) {
      if (node.type === 2 && /[A-Za-z]/.test(node.content)) {
        assert.ok(['i', 'context7'].includes(node.content.trim()), `${filename}: raw text ${node.content}`);
      }
      for (const prop of node.props ?? []) {
        if (prop.type === 6 && ['aria-label', 'title', 'alt', 'placeholder', 'label'].includes(prop.name)) {
          assert.ok(!prop.value?.content, `${filename}: raw ${prop.name} ${prop.value?.content}`);
        }
      }
      for (const child of node.children ?? []) visit(child);
    }
    visit(descriptor.template!.ast);
  }
});
