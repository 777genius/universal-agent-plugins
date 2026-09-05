import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { describe, it } from 'node:test';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { pluginCommands } from '../utils/commands.ts';
import { discoveryPlugin } from '../utils/discovery.ts';
import {
  catalogVisiblePlugins,
  filterPlugins,
  groupCatalogPlugins,
  catalogQuery,
  restoreCatalogQuery,
} from '../utils/filter.ts';
import { mirroredIconPath, parseDirectoryData } from '../utils/registry.ts';
import { projectRegistry } from '../utils/registryProjection.ts';
import type { DiscoveryRecord, DiscoverySnapshot } from '../types/discovery.ts';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const pointer = JSON.parse(
  readFileSync(resolve(root, 'public/registry/schemas/1/latest.json'), 'utf8'),
) as { snapshot_path: string };
const snapshot = JSON.parse(
  readFileSync(resolve(root, 'public/registry/schemas/1', pointer.snapshot_path), 'utf8'),
) as unknown;
const registry = parseDirectoryData(snapshot, 'published_snapshot');
const discoverySnapshot = {
  sequence: 1,
  generated_at: '2026-01-01T00:00:00Z',
  expires_at: '2027-01-01T00:00:00Z',
} as DiscoverySnapshot;
const discoveryRecord = {
  slug: 'discovery:example/plugin',
  name: 'example',
  description: 'Example package',
  owner: 'example',
  repository: 'example/plugin',
  package_path: '',
  revision: '1'.repeat(40),
  version: '1.0.0',
  license: 'Apache-2.0',
  schema_version: '1.0.0',
  components: { extensions: 0, mcp: 0, skills: 0 },
  mcp_transports: [],
  compatible_clients: [],
  authentication: 'not_required',
  status: 'conformant_unreviewed',
  runtime_reviewed: false,
  tree_digest: `sha256:${'2'.repeat(64)}`,
  manifest_digest: `sha256:${'3'.repeat(64)}`,
  stars: 1,
  repository_updated_at: '2026-01-01T00:00:00Z',
  reviewed_distribution_id: null,
  availability: 'available',
} satisfies DiscoveryRecord;

describe('unified registry landing', () => {
  it('projects only the registry data required by each static route', () => {
    const plugin = registry.plugins[0]!;
    const client = plugin.client_support.clients[0]!;
    assert.deepEqual(projectRegistry(registry, { kind: 'empty' }).plugins, []);
    assert.deepEqual(
      projectRegistry(registry, { kind: 'plugin', value: plugin.name }).plugins.map(
        (item) => item.name,
      ),
      [plugin.name],
    );
    assert.ok(
      projectRegistry(registry, { kind: 'client', value: client }).plugins.every((item) =>
        item.client_support.clients.includes(client),
      ),
    );
    assert.equal(
      projectRegistry(registry, { kind: 'catalog' }).plugins.length,
      registry.plugins.filter((item) => item.trust_state !== 'conformant_unreviewed').length,
    );
  });

  it('groups alternate sources with a deterministic reviewed primary', () => {
    const reviewed = registry.plugins[0]!;
    const other = {
      ...reviewed,
      install_source: 'discovery:other/plugin',
      trust_state: 'conformant_unreviewed' as const,
      source: { ...reviewed.source, repository: 'other/plugin' },
    };
    const groups = groupCatalogPlugins([other, reviewed]);
    assert.equal(groups.length, 1);
    assert.equal(groups[0]?.primary, reviewed);
    assert.deepEqual(groups[0]?.alternatives, [other]);
    assert.equal(groupCatalogPlugins([reviewed, other])[0]?.primary, reviewed);
  });

  it('prefers upstream, available, non-test sources before stars', () => {
    const base = registry.plugins[0]!;
    const candidate = (id: string, upstream: boolean, installable: boolean, path: string) => ({
      ...base,
      install_source: id,
      trust_state: 'conformant_unreviewed' as const,
      installable,
      source: { ...base.source, repository: `${id}/plugin`, path },
      distributions: base.distributions.map((item) => ({
        ...item,
        kind: upstream ? ('upstream' as const) : ('community' as const),
      })),
    });
    const community = candidate('a', false, true, '');
    const unavailable = candidate('b', true, false, '');
    const test = candidate('c', true, true, 'fixtures/example');
    const real = candidate('d', true, true, 'plugin');
    assert.equal(groupCatalogPlugins([community, unavailable, test, real])[0]?.primary, real);
    assert.equal(groupCatalogPlugins([real, test, unavailable, community])[0]?.primary, real);
  });

  it('keeps different monorepo packages separate and groups canonical source aliases', () => {
    const base = { ...registry.plugins[0]!, trust_state: 'conformant_unreviewed' as const };
    const first = { ...base, name: 'first', source: { ...base.source, path: 'first' } };
    const second = {
      ...base,
      name: 'second',
      install_source: 'second',
      source: { ...base.source, path: 'second' },
    };
    const alias = { ...first, name: 'alias', install_source: 'alias' };
    assert.equal(groupCatalogPlugins([first, second, alias]).length, 2);
  });

  it('uses repository stars then install source as a stable primary tiebreaker', () => {
    const base = discoveryPlugin(discoveryRecord, discoverySnapshot);
    const popular = { ...base, install_source: 'z', discovery: { ...base.discovery!, stars: 100 } };
    const sameStars = { ...popular, install_source: 'a' };
    assert.equal(groupCatalogPlugins([base, popular])[0]?.primary, popular);
    assert.equal(groupCatalogPlugins([popular, sameStars])[0]?.primary, sameStars);
    assert.equal(groupCatalogPlugins([sameStars, popular])[0]?.primary, sameStars);
  });

  it('joins canonical aliases transitively without losing any source', () => {
    const base = { ...registry.plugins[0]!, trust_state: 'conformant_unreviewed' as const };
    const first = {
      ...base,
      name: 'first',
      install_source: 'first',
      source: { ...base.source, path: 'first' },
    };
    const second = {
      ...base,
      name: 'second',
      install_source: 'second',
      source: { ...base.source, path: 'second' },
    };
    const bridge = { ...first, name: 'second', install_source: 'bridge' };
    const groups = groupCatalogPlugins([first, second, bridge]);
    assert.equal(groups.length, 1);
    assert.equal(groups[0]?.alternatives.length, 2);
  });

  it('uses typo fallback only when exact search has no results, retaining filters', () => {
    const plugin = {
      ...registry.plugins[0]!,
      name: 'calendar',
      display_name: 'Calendar',
      categories: ['productivity'],
    };
    for (const query of ['calndar', 'calender', 'calenadr', 'callendar']) {
      assert.deepEqual(filterPlugins([plugin], { query }), [plugin]);
      assert.deepEqual(filterPlugins([plugin], { query, category: 'other' }), []);
    }
    const exact = {
      ...plugin,
      name: 'calender',
      display_name: 'Calender',
      install_source: 'exact',
    };
    assert.deepEqual(filterPlugins([plugin, exact], { query: 'calender' }), [exact]);
    assert.deepEqual(filterPlugins([plugin], { query: 'xyz' }), []);
  });

  it('round-trips every filter, preserves unrelated queries and clears only catalog state', () => {
    const values = [
      'calendar',
      'productivity',
      'mcp',
      'upstream',
      'reviewed',
      'codex',
      'none',
      'example',
    ];
    const query = catalogQuery(values, { campaign: 'launch' });
    assert.deepEqual(restoreCatalogQuery(query), values);
    assert.equal(query.campaign, 'launch');
    assert.deepEqual(catalogQuery(['', ...Array(7).fill('all')], query), { campaign: 'launch' });
    assert.deepEqual(restoreCatalogQuery({ q: ['calendar', 'ignored'], owner: null }), [
      'calendar',
      ...Array(7).fill('all'),
    ]);
  });
  it('loads the signed reviewed directory used by the site', () => {
    assert.ok(registry.plugins.length >= 20);
    assert.ok(registry.plugins.every((plugin) => plugin.trust_state !== 'conformant_unreviewed'));
  });

  it('omits --target for installed-agent detection', () => {
    const plugin = registry.plugins.find((item) => item.installable);
    assert.ok(plugin);
    const commands = pluginCommands(plugin);
    assert.equal(commands.add.includes('--target'), false);
    assert.match(commands.add, /^npx universal-agent-plugins add /);
    for (const command of Object.values(commands))
      assert.match(command, /^npx universal-agent-plugins /);
  });

  it('uses one comma-separated target flag for explicit multi-agent installation', () => {
    const plugin = registry.plugins.find(
      (item) =>
        item.client_support.clients.includes('codex') &&
        item.client_support.clients.includes('cursor'),
    );
    assert.ok(plugin);
    assert.match(pluginCommands(plugin, ['codex', 'cursor']).add, / --target codex,cursor$/);
  });

  it('uses only real local plugin or publisher logos without a generic fallback', () => {
    for (const plugin of registry.plugins) {
      const iconPath = mirroredIconPath(plugin);
      if (!iconPath) continue;
      assert.doesNotThrow(
        () => readFileSync(resolve(root, 'public', iconPath)),
        `${plugin.name} logo must exist`,
      );
    }

    assert.equal(
      mirroredIconPath(registry.plugins.find((plugin) => plugin.name === 'agent-code-navigator')!),
      undefined,
    );
    assert.equal(
      mirroredIconPath(registry.plugins.find((plugin) => plugin.name === 'statsig')!),
      undefined,
    );
    assert.equal(
      mirroredIconPath(registry.plugins.find((plugin) => plugin.name === 'cloudflare-docs')!),
      'plugin-icons/cloudflare.svg',
    );
    assert.equal(
      mirroredIconPath(registry.plugins.find((plugin) => plugin.name === 'heroku')!),
      'plugin-icons/heroku.svg',
    );
  });

  it('keeps reviewed results ahead of automatic discovery', () => {
    const reviewed = registry.plugins[0]!;
    const discovered = {
      ...reviewed,
      name: 'discovered-example',
      display_name: 'Discovered example',
      install_source: 'discovery:example/plugins//plugin',
      trust_state: 'conformant_unreviewed' as const,
      discovery: {
        sequence: 1,
        generated_at: '2026-01-01T00:00:00Z',
        expires_at: '2027-01-01T00:00:00Z',
        repository_updated_at: '2026-01-01T00:00:00Z',
        stars: 100_000,
        schema_version: '1.0.0' as const,
        manifest_digest: reviewed.source.manifest_sha256,
        tree_digest: reviewed.source.tree_sha256,
        mcp_transports: [],
        availability: 'available' as const,
      },
    };
    assert.equal(filterPlugins([discovered, reviewed], {})[0]?.name, reviewed.name);
  });

  it('keeps metadata-only discovery records out of the install catalog', () => {
    const metadataOnly = discoveryPlugin(discoveryRecord, discoverySnapshot);
    const portable = discoveryPlugin(
      {
        ...discoveryRecord,
        slug: 'discovery:example/portable',
        name: 'portable',
        components: { extensions: 0, mcp: 0, skills: 1 },
        compatible_clients: ['codex'],
      },
      discoverySnapshot,
    );

    assert.equal(metadataOnly.installable, false);
    assert.equal(metadataOnly.distributions[0]?.selectable, false);
    assert.equal(metadataOnly.distributions[0]?.releases[0]?.selectable, false);
    assert.equal(portable.installable, true);
    assert.deepEqual(catalogVisiblePlugins([metadataOnly, portable]), [portable]);
  });

  it('keeps every non-installable Discovery state out of the install catalog', () => {
    const unavailable = discoveryPlugin(
      {
        ...discoveryRecord,
        components: { extensions: 0, mcp: 1, skills: 0 },
        compatible_clients: ['codex'],
        availability: 'unavailable',
      },
      discoverySnapshot,
    );
    const unsupported = discoveryPlugin(
      {
        ...discoveryRecord,
        slug: 'discovery:example/unsupported',
        components: { extensions: 1, mcp: 0, skills: 0 },
      },
      discoverySnapshot,
    );
    const skillOnly = discoveryPlugin(
      {
        ...discoveryRecord,
        slug: 'discovery:example/skill-only',
        name: 'skill-only',
        components: { extensions: 0, mcp: 0, skills: 1 },
        compatible_clients: ['codex'],
      },
      discoverySnapshot,
    );

    assert.equal(unavailable.installable, false);
    assert.equal(unsupported.installable, false);
    assert.equal(skillOnly.installable, true);
    assert.deepEqual(catalogVisiblePlugins([unavailable, unsupported, skillOnly]), [skillOnly]);
  });
});
