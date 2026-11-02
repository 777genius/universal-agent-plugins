import { docsLocales } from "../config/locales.mjs";
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { createRequire } from 'node:module';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { test } from 'node:test';
import { isRetiredArchive } from '../lib/public-routes.mjs';
import { documentContent } from './check-output.mjs';

const root = new URL('../../../', import.meta.url);
const read = (path) => readFileSync(new URL(path, root), 'utf8');
// Use the site's installed VitePress, or a read-only matching cache in isolated reviews.
// VITEPRESS_TEST_MODULE is an absolute path to vitepress/dist/node/index.js.
const websiteRequire = createRequire(new URL('website/package.json', root));
const { createMarkdownRenderer } = await import(pathToFileURL(
  process.env.VITEPRESS_TEST_MODULE || websiteRequire.resolve('vitepress'),
).href);
const md = await createMarkdownRenderer(fileURLToPath(new URL('website/', root)));

test('current journeys stay searchable while retired product pages are archived', () => {
  for (const locale of docsLocales) {
    for (const page of ['index.md', 'use/index.md', 'use/install.md', 'build/index.md', 'build/skill.md', 'guide/quickstart.md'])
      assert.equal(isRetiredArchive(`${locale}/${page}`), false, `${locale}/${page}`);
  }
  for (const page of ['en/guide/installation.md', 'en/concepts/why-plugin-kit-ai.md',
    'en/api/cli/plugin-kit-ai.md', 'en/legacy/v1/index.md'])
    assert.equal(isRetiredArchive(page), true, page);
  assert.equal(isRetiredArchive('en/index.html'), false);
  assert.equal(isRetiredArchive('en/build/index.html'), false);
  assert.equal(isRetiredArchive('en/legacy/v1/index.html'), true);
});

test('published source never claims it is on a non-deploying branch', () => {
  const sourceRoot = fileURLToPath(new URL('website/source/', root));
  for (const relative of readdirSync(sourceRoot, { recursive: true })) {
    if (!relative.endsWith('.md')) continue;
    const source = readFileSync(`${sourceRoot}/${relative}`, 'utf8');
    assert.doesNotMatch(source, /non-deploying preparation branch/, relative);
  }
});

test('user dependency management never lets install infer a project from the working directory', async () => {
  for (const locale of docsLocales) {
    const source = read(`website/source/${locale}/use/manage.md`);
    const boundary = '<a id="restore-project-dependencies"></a>';
    assert.ok(source.includes(boundary), `${locale}: project section boundary`);
    const userJourney = source.split(boundary, 1)[0];
    const html = await md.renderAsync(userJourney);
    const { commands, parseErrors } = documentContent(html);
    assert.deepEqual(parseErrors, [], `${locale}: rendered user journey HTML`);
    const installs = commands
      .flatMap((command) => command.replace(/\\\n\s*/g, ' ').split('\n'))
      .map((line) => line.trim())
      .filter((line) => /^(?:(?:agentplugins|npx(?: -y)? universal-agent-plugins(?:@[^ ]+)?) )?install(?: |$)/.test(line));
    assert.ok(installs.length >= 2, `${locale}: user initialization and restoration commands`);
    for (const command of installs) {
      assert.match(command, /--scope(?:=| +)user(?: |$)/,
        `${locale}: user-environment installation must not infer a current project: ${command}`);
    }
  }
});

test('current Build journey uses Agent Plugins as its single documented command surface', () => {
  const buildRoot = fileURLToPath(new URL('website/source/en/build/', root));
  const command = /\bplugin-kit-ai (?:init|validate|inspect|test|compat|doctor|capabilities|skills)\b/;
  for (const relative of readdirSync(buildRoot)) {
    if (!relative.endsWith('.md')) continue;
    const source = readFileSync(`${buildRoot}/${relative}`, 'utf8');
    assert.match(source, /agentplugins author/, relative);
    assert.doesNotMatch(source, command, relative);
  }

  const index = read('website/source/en/build/index.md');
  assert.match(index, /npm install -g universal-agent-plugins/);
  assert.doesNotMatch(index, /npm install -g universal-agent-plugins@\d+\.\d+\.\d+/);
  assert.doesNotMatch(index, /plugin-kit-ai|plugin\.yaml|PyPI|pipx|YAML migration/i);
});

test('current locale journeys and top navigation do not expose the retired product', () => {
  for (const locale of docsLocales) {
    for (const section of ['use', 'build']) {
      const directory = fileURLToPath(new URL(`website/source/${locale}/${section}/`, root));
      for (const relative of readdirSync(directory)) {
        if (!relative.endsWith('.md')) continue;
        const visible = readFileSync(`${directory}/${relative}`, 'utf8')
          .split('<!-- locale-historical-source:start', 1)[0];
        assert.doesNotMatch(visible, /plugin-kit-ai|\/legacy\/v1\//i, `${locale}/${section}/${relative}`);
      }
    }
    const localeConfig = read(`website/.vitepress/config/locales.${locale}.ts`);
    for (const match of localeConfig.matchAll(/link:\s*"([^"]+)"/g)) {
      assert.equal(isRetiredArchive(match[1]), false, `${locale}: ${match[1]}`);
    }
  }
});

const ids = (html) => [...html.matchAll(/\bid="([^"]+)"/g)].map(m => m[1]);
const quickstartIds = { en: 'quickstart', ru: 'быстрыи-старт', es: 'inicio-rapido', fr: 'demarrage-rapide', zh: '快速入门', uk: 'quickstart', hi: 'quickstart', ar: 'quickstart', pt: 'quickstart' };
for (const locale of docsLocales) {
  test(`${locale}: current quickstart fragments resolve exactly once`, async () => {
    const source = read(`website/source/${locale}/guide/quickstart.md`);
    const html = await md.renderAsync(source);
    const current = ids(html);
    for (const id of [quickstartIds[locale], 'use-plugins', 'build-plugins']) {
      assert.equal(current.filter(value => value === id).length, 1, `${locale}: #${id}`);
    }
    assert.ok(html.indexOf(`id="${quickstartIds[locale]}"`) < html.indexOf('<h1 '));
    assert.ok(html.indexOf('id="use-plugins"') < html.indexOf('id="build-plugins"'));
  });
}

test('README promotes the canonical quickstart and current build section', () => {
  const source = read('README.md');
  const canonical = 'https://universal-agent-plugins.com/docs/en/guide/quickstart.html';
  const links = [...source.matchAll(/\[([^\]]+)\]\(([^)]+)\)/g)];
  for (const label of ['Use / Build quickstart']) {
    assert.deepEqual(links.filter(m => m[1] === label).map(m => m[2]), [canonical]);
  }
  assert.match(source, /^### Quick start$/m);
  assert.match(source, /^## Build plugins$/m);
  assert.match(source, /\[Build plugins\]\(#build-plugins\)/);
});
