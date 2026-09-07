import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';
import { createI18n } from 'vue-i18n';

const root = new URL('../../', import.meta.url);
const read = (path: string) => readFileSync(new URL(path, root), 'utf8');
const locales = ['en', 'ru', 'es', 'fr', 'zh'];
const renderedCopy = (locale: string) => {
  const messages = JSON.parse(read(`landing/locales/${locale}.json`));
  const { t } = createI18n({ legacy: false, locale, fallbackLocale: false,
    messages: { [locale]: messages } }).global;
  return Object.fromEntries(Object.keys(messages.publicAuthoring).map(key =>
    [key, t(`publicAuthoring.${key}`)]));
};

test('all public authoring messages compile and render literal npm latest tags', (context) => {
  const errors = context.mock.method(console, 'error', () => {});
  for (const locale of locales) {
    const source = JSON.parse(read(`landing/locales/${locale}.json`)).publicAuthoring;
    const rendered = renderedCopy(locale);
    for (const key of Object.keys(source)) {
      assert.equal(rendered[key], source[key].replaceAll("{'@'}", '@'), `${locale}:${key}`);
    }
    assert.ok(rendered.unreleased.includes('plugin-kit-ai@latest'), locale);
    assert.equal(errors.mock.callCount(), 0, `${locale}: message compilation errors`);
  }
});

test('the authoring front door renders Use/Build and preserves its indexing policy', () => {
  const page = read('landing/pages/create-plugin.vue');
  for (const id of ['use-plugins', 'build-plugins', 'historical-v1']) {
    assert.ok(page.includes(`id="${id}"`));
  }
  for (const legacy of ['HeroSection', 'FeaturesSection', 'DownloadSection', 'FAQSection']) {
    assert.ok(!page.includes(legacy));
  }
  assert.ok(page.includes("robots: 'noindex, follow'"));
  assert.ok(page.includes('npx universal-agent-plugins add context7'));
  const keys = [...page.matchAll(/(?:t|usePageSeo)\('publicAuthoring\.([^']+)'/g)].map(m => m[1]);
  for (const locale of locales) {
    const copy = renderedCopy(locale);
    for (const key of [...keys, 'intro']) assert.equal(typeof copy[key], 'string', `${locale}:${key}`);
    assert.ok(copy.standard.includes('plugin.json'));
    assert.ok(copy.unreleased.includes('1.2.4'));
    assert.ok(copy.unreleased.includes('plugin-kit-ai@latest'));
    assert.ok(copy.versions.includes('agentplugins-v0.1.51'));
    assert.ok(copy.limitations.includes('SSE'));
  }
  // docsUrl already ends in /en/: exercise the exact expression used in the page.
  const expression = page.match(/const base = ([\s\S]*?);/)![1];
  for (const locale of locales) {
    const base = Function('config', `return ${expression}`)({
      public: { docsUrl: 'https://777genius.github.io/universal-agent-plugins/docs/en/' },
    });
    assert.equal(`${base}/${locale}/guide/quickstart.html`,
      `https://777genius.github.io/universal-agent-plugins/docs/${locale}/guide/quickstart.html`);
  }
});

test('all quickstarts separate installation, preparation and historical commands', () => {
  for (const locale of locales) {
    const text = read(`website/source/${locale}/guide/quickstart.md`);
    const copy = renderedCopy(locale);
    assert.ok(text.includes('canonicalId: "page:guide:quickstart"'));
    for (const key of ['standard', 'unreleased', 'versions', 'limitations', 'history']) {
      assert.ok(text.includes(copy[key]), `${locale}:${key}`);
    }
    const history = text.indexOf('{#historical-v1}');
    assert.ok(history > text.indexOf('{#build-plugins}'));
    const front = text.slice(0, history);
    assert.deepEqual([...front.matchAll(/```bash\n([\s\S]*?)```/g)].map(m => m[1].trim()),
      ['npx universal-agent-plugins add context7']);
    assert.ok(!front.includes('plugin-kit-ai init'));
    assert.ok(!text.includes('npx plugin-kit-ai@latest add notion'));
    for (const command of ['plugin-kit-ai init my-plugin', 'plugin-kit-ai generate',
      'plugin-kit-ai validate', '--runtime node --typescript', '--runtime python']) {
      assert.ok(text.slice(history).includes(command), `${locale}:${command}`);
    }
    for (const match of text.matchAll(/\]\(\/(en|ru|es|fr|zh)\/([^#)]+)(?:#[^)]*)?\)/g)) {
      assert.ok(existsSync(fileURLToPath(new URL(`website/source/${match[1]}/${match[2]}.md`, root))), match[0]);
    }
  }
});

test('README retains the available installer and canonical client limitations', () => {
  const text = read('README.md');
  assert.ok(text.includes('## Use plugins'));
  assert.ok(text.includes('## Build plugins'));
  assert.ok(text.includes('### Quick start'));
  assert.ok(text.includes('id="authoring-and-development"'));
  assert.ok(text.includes('standard-first authoring CLI is not released'));
  assert.ok(text.includes('For Codex, declared MCP SSE is unsupported'));
  assert.ok(text.includes('docs/CODEX_TRANSPORT_EVIDENCE.md'));
});
