import assert from 'node:assert/strict';
import { test } from 'node:test';
import { catalogCategories } from '../utils/catalogCategories.ts';

test('Marketing covers the kit, CRM and two exact upstream package paths', () => {
  for (const [repository, path] of [
    ['777genius/universal-agent-plugins', 'examples/plugins/marketing-kit'],
    ['777genius/universal-agent-plugins', 'plugins/hubspot-crm'],
    ['wondelai/skills', 'plugins/marketing-cro'],
    ['sickn33/agentic-awesome-skills', 'plugins/agentic-bundle-aas-marketing-seo-growth'],
  ]) assert.ok(catalogCategories({ repository, path }).includes('marketing'));
});

test('categories preserve metadata and never classify forks or unrelated packages', () => {
  assert.deepEqual(catalogCategories({ repository: 'wondelai/skills', path: 'plugins/marketing-cro' }, ['marketing', 'business']), ['marketing', 'business']);
  assert.deepEqual(catalogCategories({ repository: 'wondelai/skills', path: 'plugins/MARKETING-CRO' }), []);
  assert.deepEqual(catalogCategories({ repository: 'someone/skills', path: 'plugins/marketing-cro' }), []);
  assert.deepEqual(catalogCategories({ repository: 'wondelai/skills', path: 'plugins/code-craftsmanship' }, ['coding']), ['coding']);
});
