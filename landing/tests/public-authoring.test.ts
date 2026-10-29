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

test('all authoring messages compile and distinguish installer, source tools and historical API', (context) => {
  const errors = context.mock.method(console, 'error', () => {});
  for (const locale of locales) {
    const source = JSON.parse(read(`landing/locales/${locale}.json`)).publicAuthoring;
    const rendered = renderedCopy(locale);
    for (const key of Object.keys(source)) {
      assert.equal(rendered[key], source[key].replaceAll("{'@'}", '@'), `${locale}:${key}`);
    }
    assert.ok(rendered.versions.includes('universal-agent-plugins'), locale);
    assert.ok(rendered.versions.includes('agentplugins-v*'), locale);
    for (const fact of ['0.1.94', 'agentplugins author', '0.1.65'])
      assert.ok(rendered.releaseScope.includes(fact), `${locale}:${fact}`);
    assert.doesNotMatch(rendered.releaseScope, /E2E|production-ready/i, locale);
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
  assert.ok(page.includes('npx -y universal-agent-plugins add context7'));
  const keys = [...page.matchAll(/(?:t|usePageSeo)\('publicAuthoring\.([^']+)'/g)].map((m) => m[1]);
  for (const locale of locales) {
    const copy = renderedCopy(locale);
    for (const key of [...keys, 'intro', 'availability', 'releaseScope', 'buildGuideLink'])
      assert.equal(typeof copy[key], 'string', `${locale}:${key}`);
    assert.ok(copy.standard.includes('plugin.json'));
    assert.doesNotMatch(copy.releaseScope, /plugin-kit-ai|PyPI|pipx|1\.2\.4/i);
    assert.ok(copy.versions.includes('agentplugins-v*'));
    assert.ok(copy.versions.includes('0.1.94'));
    assert.ok(copy.releaseScope.includes('0.1.65'));
    assert.ok(copy.releaseScope.includes('agentplugins author'));
    assert.equal(Object.hasOwn(copy, 'history'), false);
    assert.equal(Object.hasOwn(copy, 'historyTitle'), false);
    assert.doesNotMatch(copy.versions, /plugin-kit-ai|PyPI|pipx/i);
    for (const channel of ['npm', 'Homebrew', 'GitHub'])
      assert.ok(copy.releaseScope.includes(channel), `${locale}:${channel}`);
    assert.doesNotMatch(copy.releaseScope, /(?:0\.1\.61|2\.0\.1)/);
    assert.ok(copy.limitations.includes('SSE'));
  }
  for (const locale of locales) {
    assert.equal(
      resolveDocsLink('quickstart', locale).url,
      `https://777genius.github.io/universal-agent-plugins/docs/${locale}/guide/quickstart.html`,
    );
  }
});

test('landing FAQ translated copy avoids retired public docs', () => {
  const section = read('landing/components/sections/FAQSection.vue');
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

test('quickstarts use released installer commands and link source authoring separately', () => {
  for (const locale of locales) {
    const text = read(`website/source/${locale}/guide/quickstart.md`);
    const preservation = text.indexOf('<!-- locale-historical-source:start');
    const publishedText = preservation === -1 ? text : text.slice(0, preservation);
    assert.ok(text.includes('canonicalId: "page:guide:quickstart"'));
    assert.match(publishedText, /^description: .*Agent Plugins 1\.0.*$/m);
    assert.doesNotMatch(publishedText, /^description: .*plugin-kit-ai.*$/m);
    for (const fact of ['universal-agent-plugins', 'agentplugins-v0.1.94', 'agentplugins author'])
      assert.ok(publishedText.includes(fact), `${locale}:${fact}`);
    assert.ok(publishedText.includes('../build/#build-from-source'), locale);
    assert.doesNotMatch(publishedText, /plugin-kit-ai|PyPI|pipx|historical-v1/i);
    const commands = [...publishedText.matchAll(/```bash\n([\s\S]*?)```/g)].map((m) => m[1]);
    for (const command of [
      'npx universal-agent-plugins add context7',
      'npx universal-agent-plugins deps export',
      'npx universal-agent-plugins deps import',
      'npx universal-agent-plugins install --locked --scope user',
      'npx universal-agent-plugins validate ./my-plugin',
    ]) assert.ok(commands.some((block) => block.includes(command)), `${locale}:${command}`);
    assert.ok(commands.some((block) => block.includes('--scope project') && block.includes('--dry-run')), locale);
    // Installer onboarding must not run commands omitted from the released binary.
    assert.ok(commands.every((block) => !/agentplugins author/.test(block)), locale);
    assert.doesNotMatch(publishedText, /npm install[^\n]*universal-agent-plugins[\s\S]{0,80}agentplugins author init/);
    for (const match of publishedText.matchAll(/\]\(\/(en|ru|uk|zh|es|hi|ar|pt|fr)\/([^#)]+)(?:#[^)]*)?\)/g)) {
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
      '[Read the documentation](https://universal-agent-plugins.com/docs/en/)',
    ),
  );
  assert.ok(text.includes('Native Agent Plugins releases'));
  assert.match(
    text,
    /(?:^|\n)[^\n]*\b[Ii]nstall\b[^\n]*\blatest\b[^\n]*Node\.js 22\+[^\n]*:\n\n```bash\nnpm install (?:--global|-g) universal-agent-plugins\n```/,
  );
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
  assert.ok(text.includes('[Go SDK integration guide](sdk/README.md)'));
  assert.ok(existsSync(fileURLToPath(new URL('sdk/README.md', root))));
  const showcase = read('landing/components/sections/BuiltWithSection.vue');
  assert.ok(showcase.includes('https://github.com/777genius/universal-agent-plugins/blob/main/sdk/README.md'));
  assert.ok(showcase.includes("docsLabel(t('shell.builtWith.buildWithSdk'), locale.value !== 'en')"));
});
