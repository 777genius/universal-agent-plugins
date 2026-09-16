import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { test } from 'node:test';
import { createI18n, type LocaleMessageDictionary, type VueMessageType } from 'vue-i18n';
import { resolveDocsLink } from '../data/docsAvailability.ts';

const root = new URL('../../', import.meta.url);
const read = (path: string) => readFileSync(new URL(path, root), 'utf8');
const locales = ['en', 'ru', 'uk', 'es', 'fr', 'zh', 'ar', 'hi', 'pt'] as const;
const renderedCopy = (locale: (typeof locales)[number]) => {
  const messages = JSON.parse(read(`landing/locales/${locale}.json`));
  // Render each dictionary in an isolated EN test host.
  const { t } = createI18n<[LocaleMessageDictionary<VueMessageType>], 'en', false>({
    legacy: false,
    locale: 'en',
    fallbackLocale: false,
    messages: { en: messages },
  }).global;
  return Object.fromEntries(
    Object.keys(messages.publicAuthoring).map((key) => [key, t(`publicAuthoring.${key}`)]),
  );
};

test('all public authoring messages compile and expose only Agent Plugins release facts', (context) => {
  const errors = context.mock.method(console, 'error', () => {});
  for (const locale of locales) {
    const source = JSON.parse(read(`landing/locales/${locale}.json`)).publicAuthoring;
    const rendered = renderedCopy(locale);
    for (const key of Object.keys(source)) {
      assert.equal(rendered[key], source[key].replaceAll("{'@'}", '@'), `${locale}:${key}`);
    }
    assert.ok(rendered.releaseScope.includes('universal-agent-plugins'), locale);
    assert.ok(rendered.releaseScope.includes('agentplugins-v*'), locale);
    assert.doesNotMatch(rendered.releaseScope, /universal-agent-plugins@\d+\.\d+\.\d+/, locale);
    assert.doesNotMatch(rendered.releaseScope, /plugin-kit-ai|PyPI|pipx/i, locale);
    assert.equal(errors.mock.callCount(), 0, `${locale}: message compilation errors`);
  }
});

test('the authoring front door renders Use/Build and preserves its indexing policy', () => {
  const page = read('landing/pages/create-plugin.vue');
  for (const id of ['use-plugins', 'build-plugins']) {
    assert.ok(page.includes(`id="${id}"`));
  }
  for (const legacy of ['HeroSection', 'FeaturesSection', 'DownloadSection', 'FAQSection']) {
    assert.ok(!page.includes(legacy));
  }
  assert.ok(page.includes("robots: 'noindex, follow'"));
  assert.ok(page.includes('npx universal-agent-plugins add context7'));
  const keys = [...page.matchAll(/(?:t|usePageSeo)\('publicAuthoring\.([^']+)'/g)].map((m) => m[1]);
  for (const locale of ['en', 'ru', 'uk', 'es', 'fr', 'zh'] as const) {
    const copy = renderedCopy(locale);
    for (const key of [...keys, 'intro', 'availability', 'releaseScope', 'buildGuideLink'])
      assert.equal(typeof copy[key], 'string', `${locale}:${key}`);
    assert.ok(copy.standard.includes('plugin.json'));
    assert.doesNotMatch(copy.releaseScope, /plugin-kit-ai|PyPI|pipx|1\.2\.4/i);
    assert.ok(copy.versions.includes('agentplugins-v*'));
    assert.equal(Object.hasOwn(copy, 'history'), false);
    assert.equal(Object.hasOwn(copy, 'historyTitle'), false);
    assert.doesNotMatch(copy.versions, /plugin-kit-ai|PyPI|pipx/i);
    for (const channel of ['npm', 'Homebrew', 'GitHub'])
      assert.ok(copy.releaseScope.includes(channel), `${locale}:${channel}`);
    assert.doesNotMatch(copy.releaseScope, /(?:0\.1\.61|2\.0\.1)/);
    assert.ok(copy.limitations.includes('SSE'));
  }
  for (const locale of ['en', 'ru', 'uk', 'es', 'fr', 'zh']) {
    assert.equal(
      resolveDocsLink('quickstart', locale).url,
      `https://777genius.github.io/universal-agent-plugins/docs/${locale === 'uk' ? 'en' : locale}/guide/quickstart.html`,
    );
  }
});

test('landing FAQ sends readers only to maintained public docs routes', () => {
  const section = read('landing/components/sections/FAQSection.vue');
  for (const route of ['/guide/quickstart.html', '/build/', '/use/']) {
    assert.ok(section.includes(route), route);
  }
  assert.doesNotMatch(section, /guide\/python-runtime|reference\/support-boundary/);
  for (const locale of locales) {
    const links = JSON.parse(read(`landing/locales/${locale}.json`)).faq.quickLinks;
    for (const key of ['quickstartTitle', 'quickstartBody', 'buildTitle', 'buildBody', 'useTitle', 'useBody']) {
      assert.equal(typeof links[key], 'string', `${locale}:${key}`);
      assert.ok(links[key].length > 0, `${locale}:${key}`);
    }
    for (const retiredKey of ['pythonTitle', 'pythonBody', 'boundaryTitle', 'boundaryBody']) {
      assert.equal(Object.hasOwn(links, retiredKey), false, `${locale}:${retiredKey}`);
    }
  }
});

test('all current quickstarts expose Agent Plugins without a legacy product journey', () => {
  for (const locale of ['en', 'ru', 'es', 'fr', 'zh'] as const) {
    const text = read(`website/source/${locale}/guide/quickstart.md`);
    const preservation = text.indexOf('<!-- locale-historical-source:start');
    const publishedText = preservation === -1 ? text : text.slice(0, preservation);
    const copy = renderedCopy(locale);
    assert.ok(text.includes('canonicalId: "page:guide:quickstart"'));
    assert.match(publishedText, /^description: .*Agent Plugins 1\.0.*$/m);
    assert.doesNotMatch(publishedText, /^description: .*plugin-kit-ai.*$/m);
    for (const key of ['standard', 'limitations']) {
      assert.ok(text.includes(copy[key]), `${locale}:${key}`);
    }
    for (const releaseFact of ['universal-agent-plugins', 'agentplugins-v*']) {
      assert.ok(publishedText.includes(releaseFact), `${locale}:${releaseFact}`);
    }
    assert.doesNotMatch(publishedText, /plugin-kit-ai|PyPI|pipx|historical-v1/i);
    assert.doesNotMatch(publishedText, /(?:0\.1\.61|2\.0\.1|not released|release candidate)/i);
    const expectedCommands = ['npx universal-agent-plugins add context7'];
    if (locale === 'en') {
      expectedCommands.push(`npm install --global universal-agent-plugins
agentplugins author init ./my-plugin --template skill --name my-plugin \\
  --description 'Instructions for a repeatable agent task'
agentplugins author validate ./my-plugin
agentplugins author inspect ./my-plugin
agentplugins author test ./my-plugin`);
    }
    assert.deepEqual(
      [...publishedText.matchAll(/```bash\n([\s\S]*?)```/g)].map((m) => m[1].trim()),
      expectedCommands,
    );
    for (const match of publishedText.matchAll(/\]\(\/(en|ru|es|fr|zh)\/([^#)]+)(?:#[^)]*)?\)/g)) {
      const target = `website/source/${match[1]}/${match[2]}`.replace(/\/$/, '');
      assert.ok(
        existsSync(fileURLToPath(new URL(`${target}.md`, root))) ||
          existsSync(fileURLToPath(new URL(`${target}/index.md`, root))),
        match[0],
      );
    }
  }
});

test('README retains the available installer and canonical client limitations', () => {
  const text = read('README.md');
  assert.ok(text.includes('## Use plugins'));
  assert.ok(text.includes('## Build plugins'));
  assert.ok(text.includes('### Quick start'));
  assert.ok(text.includes('[Build plugins](#build-plugins)'));
  assert.ok(
    text.includes(
      '[Read the documentation](https://777genius.github.io/universal-agent-plugins/docs/en/)',
    ),
  );
  assert.ok(text.includes('Native Agent Plugins releases'));
  assert.ok(text.includes('Install the latest npm release'));
  assert.doesNotMatch(text, /npm install (?:--global|-g) universal-agent-plugins@\d+\.\d+\.\d+/);
  assert.doesNotMatch(
    text,
    /Milestone A|Historical authoring and development|plugin-kit-ai|PyPI|pipx/i,
  );
  assert.doesNotMatch(
    text,
    /Availability is unverified|candidate commands are not current installation advice/,
  );
  assert.ok(text.includes('For Codex, declared MCP SSE is unsupported'));
  assert.ok(text.includes('docs/CODEX_TRANSPORT_EVIDENCE.md'));
});
