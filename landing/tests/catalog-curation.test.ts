import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import type { RegistryPlugin } from '../types/registry.ts';
import type { SkillListing } from '../types/skillsCatalog.ts';
import {
  pluginBenefit,
  recommendedPluginNames,
  recommendedSkillKey,
  recommendedSkillDescription,
  recommendedSkills,
  sortSkillsForCatalog,
} from '../data/catalogCuration.ts';
import { filterPlugins } from '../utils/filter.ts';

const plugin = (name: string, extra: Partial<RegistryPlugin> = {}): RegistryPlugin =>
  ({
    name,
    display_name: name,
    install_source: name,
    trust_state: 'reviewed',
    description: 'Community package for the official integration.',
    source: {
      repository: 'example/tools',
      revision: '1'.repeat(40),
      path: name,
      manifest_sha256: `sha256:${'2'.repeat(64)}`,
      tree_sha256: `sha256:${'3'.repeat(64)}`,
    },
    categories: ['developer'],
    keywords: [],
    components: ['mcp'],
    distributions: [],
    installable: true,
    author: { name: 'example' },
    ...extra,
  }) as unknown as RegistryPlugin;

test('editorial order prioritizes practical reviewed tools; alphabetical option is reversible', () => {
  const candidates = [
    plugin('cloudflare'), plugin('notion'), plugin('github'), plugin('playwright'),
    plugin('unknown-tool'), plugin('context7'),
  ];
  assert.deepEqual(
    filterPlugins(candidates, {}).map((item) => item.name),
    ['github', 'playwright', 'context7', 'notion', 'cloudflare', 'unknown-tool'],
  );
  assert.deepEqual(
    filterPlugins(candidates, { order: 'name' }).map((item) => item.name),
    ['cloudflare', 'context7', 'github', 'notion', 'playwright', 'unknown-tool'],
  );
  assert.equal(new Set(recommendedPluginNames).size, recommendedPluginNames.length);
});

test('uninstallable reviewed items and discovered packages cannot be promoted by curation', () => {
  const unready = plugin('github', { installable: false });
  const ready = plugin('notion');
  const discovered = plugin('github', {
    trust_state: 'conformant_unreviewed',
    source: { ...unready.source, repository: 'unreviewed/github', path: '' },
    install_source: 'discovery:unreviewed',
    description: 'Unreviewed community description',
  });
  assert.deepEqual(
    filterPlugins([unready, discovered, ready], {}).map((item) => item.install_source),
    ['notion', 'github', 'discovery:unreviewed'],
  );
  assert.equal(
    pluginBenefit(discovered),
    'Unreviewed community description',
    'unsigned community copy must never inherit reviewed editorial claims',
  );
  assert.equal(pluginBenefit(plugin('unknown-tool')), 'Community package for the official integration.');
  assert.match(pluginBenefit(plugin('github')), /pull requests/);
});

test('exact-name search stays relevance first regardless of curated ranking', () => {
  const github = plugin('github');
  const guide = plugin('github-guide');
  assert.deepEqual(filterPlugins([guide, github], { query: 'github' }).map(p => p.name),
    ['github', 'github-guide']);
  assert.deepEqual(filterPlugins([guide, github], { query: 'github', order: 'name' }).map(p => p.name),
    ['github', 'github-guide']);
  assert.equal(filterPlugins([plugin('github'), plugin('figma')], { query: 'browser' }).length, 0);
  assert.deepEqual(filterPlugins([plugin('github'), plugin('figma')], { query: 'issues' }).map(p => p.name), ['github']);
});

const skill = (repo: string, slug: string, id = `skills-sh:${repo}:${slug}`): SkillListing => ({
  id,
  providerId: 'skills-sh',
  providerUrl: 'https://www.skills.sh/',
  title: slug,
  skillNameHint: slug,
  source: { kind: 'github', repository: repo, url: 'https://github.com/' + repo },
  metrics: { installs: 1, window: 'all-time' },
  availability: 'unverified',
});

test('curated Skill copy is bound to exact signed-like source identity, never to a title alone', () => {
  assert.equal(recommendedSkills.length, 6);
  assert.equal(new Set(recommendedSkills.map(s => s.repository + ':' + s.slug)).size, 6);
  assert.equal(recommendedSkillKey(skill('anthropics/skills', 'frontend-design')), 'frontend-design');
  assert.equal(recommendedSkillKey(skill('untrusted/skills', 'frontend-design')), undefined);
  assert.equal(recommendedSkillKey(skill('anthropics/skills', 'frontend-design',
    'skills-sh:anthropics/skills:another-skill')), undefined);
  const source = skill('anthropics/skills', 'frontend-design');
  assert.equal(recommendedSkillKey({ ...source, source: { kind: 'external', url: 'https://example.com' } }), undefined);
});

test('all nine locale dictionaries resolve only genuine featured Skill keys', () => {
  const codes = ['en', 'ru', 'uk', 'zh', 'es', 'hi', 'ar', 'pt', 'fr'];
  const slugs = recommendedSkills.map((skill) => skill.slug);
  for (const code of codes) {
    const locale = JSON.parse(readFileSync(
      new URL(`../features/skills-catalog/locales/${code}.json`, import.meta.url),
      'utf8',
    )) as { registryUi: { skills: { featured: { title: string; explanation: string;
      items: Record<string, string> } } } };
    const featured = locale.registryUi.skills.featured;
    assert.ok(featured.title && featured.explanation, `${code}: visible disclosure`);
    assert.deepEqual(Object.keys(featured.items).sort(), [...slugs].sort(), `${code}: exact picks`);
    for (const slug of slugs) {
      const value = recommendedSkillDescription(slug, (key) =>
        key.split('.').slice(4).reduce<string | Record<string, string>>((node, part) =>
          typeof node === 'string' ? '' : (node[part] ?? ''), featured.items) as string,
      );
      assert.equal(value, featured.items[slug], `${code}: ${slug}`);
      assert.ok(value.length > 10, `${code}: short actionable description`);
    }
  }
  assert.equal(recommendedSkillDescription('unknown', (key) => key), '');
  assert.equal(recommendedSkillDescription(undefined, (key) => key), '');
});

test('curated Skills sort in one source-backed list, preserving metrics and provider tail', () => {
  const ordinary = skill('example/skills', 'ordinary');
  const more = skill('example/skills', 'next');
  const react = skill('vercel-labs/agent-skills', 'vercel-optimize');
  const browser = skill('vercel-labs/agent-browser', 'agent-browser');
  const frontend = skill('anthropics/skills', 'frontend-design');
  const source = [ordinary, react, frontend, more, browser];
  const sorted = sortSkillsForCatalog(source);
  assert.deepEqual(
    sorted.map((item) => item.id),
    [browser.id, frontend.id, react.id, ordinary.id, more.id],
  );
  assert.deepEqual(source.map((item) => item.id), [
    ordinary.id, react.id, frontend.id, more.id, browser.id,
  ], 'sorting must not mutate the provider list');
  assert.equal(sorted[0], browser, 'use real provider DTO including installs/availability');
  assert.equal(sorted.length, source.length, 'do not add synthetic or duplicate cards');
});

test('lookalike publisher/title and unknown Skills never receive editorial rank', () => {
  const real = skill('anthropics/skills', 'frontend-design');
  const spoof = skill('unknown/skills', 'frontend-design');
  const mismatched = skill('anthropics/skills', 'frontend-design',
    'skills-sh:anthropics/skills:not-frontend');
  const other = skill('unknown/skills', 'other');
  assert.deepEqual(
    sortSkillsForCatalog([spoof, other, real, mismatched]).map((item) => item.id),
    [real.id, spoof.id, other.id, mismatched.id],
  );
  assert.deepEqual(sortSkillsForCatalog([other, spoof]), [other, spoof]);
  assert.deepEqual(sortSkillsForCatalog([]), []);
});
