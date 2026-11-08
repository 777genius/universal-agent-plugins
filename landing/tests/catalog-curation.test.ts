import assert from 'node:assert/strict';
import { test } from 'node:test';
import type { RegistryPlugin } from '../types/registry.ts';
import type { SkillListing } from '../types/skillsCatalog.ts';
import {
  pluginBenefit,
  recommendedPluginNames,
  recommendedSkillKey,
  recommendedSkills,
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
