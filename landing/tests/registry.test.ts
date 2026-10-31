import assert from 'node:assert/strict';
import type { ClientID } from '../types/registry.ts';
import { readFileSync } from 'node:fs';
import { describe, it } from 'node:test';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { pluginCommands } from '../utils/commands.ts';
import { discoveryPlugin } from '../utils/discovery.ts';
import {
  catalogVisiblePlugins,
  catalogTrustCounts,
  filterPlugins,
  groupCatalogPlugins,
  catalogQuery,
  restoreCatalogQuery,
} from '../utils/filter.ts';
import { clients } from '../data/clients.ts';
import {
  mirroredIconPath,
  parseDirectoryData,
  resolveDistribution,
  vendorInstallSource,
} from '../utils/registry.ts';
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
  it('accepts Directory target policies for newly advertised native clients without enabling undeclared targets', () => {
    const fixture = structuredClone(snapshot) as {
      distributions: {
        id: string;
        release_policies: {
          targets: { client: string; authentication: string; delivery: string; scopes: string[] }[];
        }[];
      }[];
    };
    const nativeClients = clients.filter((client) => ['grok', 'kimi', 'qoder', 'pi'].includes(client.id));
    const distribution = fixture.distributions.find((item) => item.id === '777genius/context7')!;
    for (const policy of distribution.release_policies) {
      policy.targets = nativeClients.map((client) => ({
        client: client.id,
        authentication: 'not_required',
        delivery: 'managed',
        scopes: ['user'],
      }));
    }
    const parsed = parseDirectoryData(fixture, 'published_snapshot');
    const context7 = parsed.plugins.find((plugin) => plugin.name === 'context7')!;
    const community = context7.distributions.find((item) => item.id === distribution.id)!;
    assert.deepEqual(
      community.releases[0]!.targets.map((target) => target.client),
      nativeClients.map((client) => client.id),
    );
    for (const policy of distribution.release_policies) {
      policy.targets = policy.targets.filter((target) => target.client !== 'qoder');
    }
    const withoutQoder = parseDirectoryData(fixture, 'published_snapshot');
    const unchanged = withoutQoder.plugins
      .find((plugin) => plugin.name === 'context7')!
      .distributions.find((item) => item.id === distribution.id)!;
    assert.equal(
      unchanged.releases[0]!.targets.some((target) => target.client === 'qoder'),
      false,
    );
    distribution.release_policies[0]!.targets[0]!.client = 'unknown-client';
    assert.throws(
      () => parseDirectoryData(fixture, 'published_snapshot'),
      /invalid or duplicate client/,
    );
  });

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

  it('intro counts match the grouped catalog total', () => {
    const counts = catalogTrustCounts(registry.plugins);
    assert.equal(counts.total, groupCatalogPlugins(catalogVisiblePlugins(registry.plugins)).length);
    assert.equal(counts.reviewed + counts.community, counts.total);
  });

  it('keeps temporarily unlisted products out of every catalog surface', () => {
    const hidden = registry.plugins.find((plugin) => plugin.name === 'agent-notifications')!;
    assert.ok(hidden, 'the held-back product is still published in the Directory feed');
    assert.equal(catalogVisiblePlugins([hidden]).length, 0);
    assert.equal(catalogVisiblePlugins(registry.plugins).includes(hidden), false);
    assert.equal(catalogTrustCounts([hidden]).total, 0);
    const visible = registry.plugins.find(
      (plugin) => plugin.name !== 'agent-notifications' && catalogVisiblePlugins([plugin]).length,
    )!;
    assert.equal(groupCatalogPlugins(catalogVisiblePlugins([hidden, visible])).length, 1);
  });

  it('derives product authentication from signed release target policies', () => {
    const fixture = structuredClone(snapshot) as {
      distributions: {
        id: string;
        product_id: string;
        release_policies: {
          targets: { client: string; authentication: string; delivery: string; scopes: string[] }[];
        }[];
      }[];
    };
    const distributions = fixture.distributions.filter((item) => item.product_id === 'context7');
    assert.ok(distributions.length, 'context7 policies are present in the Directory feed');
    for (const distribution of distributions)
      for (const policy of distribution.release_policies)
        for (const target of policy.targets) target.authentication = 'not_required';
    const parse = () =>
      parseDirectoryData(fixture, 'published_snapshot').plugins.find(
        (plugin) => plugin.name === 'context7',
      )!;
    assert.equal(parse().authentication, 'none');
    const parsed = parse();
    const alternative = distributions.find((item) => item.id !== parsed.default_distribution)!;
    assert.ok(alternative, 'context7 has an alternative distribution in the Directory feed');
    for (const policy of alternative.release_policies)
      policy.targets[0]!.authentication = 'required';
    assert.equal(
      parse().authentication,
      'none',
      'the default install path decides the product answer',
    );
    for (const policy of distributions.find((item) => item.id === parsed.default_distribution)!
      .release_policies)
      for (const target of policy.targets) target.authentication = 'required';
    assert.equal(parse().authentication, 'unknown');
  });

  it('promotes reviewed-external materialization evidence into materialized clients', () => {
    const fixture = structuredClone(snapshot) as {
      snapshot?: unknown;
      distributions: {
        id: string;
        releases: { sequence: number; tree_digest: string }[];
        release_policies: {
          release_sequence: number;
          current_evidence: string[];
          targets: { client: ClientID }[];
        }[];
      }[];
      evidence: Record<string, unknown>[];
    };
    const raw = (fixture.snapshot ?? fixture) as typeof fixture;
    assert.ok(raw.distributions.length, 'the Directory feed has distributions');
    const pre = parseDirectoryData(fixture, 'published_snapshot');
    let promoted = false;
    outer: for (const distribution of raw.distributions) {
      const plugin = pre.plugins.find((item) =>
        item.distributions.some((view) => view.id === distribution.id),
      );
      if (!plugin) continue;
      const view = plugin.distributions.find((item) => item.id === distribution.id)!;
      for (const release of distribution.releases) {
        const policy = distribution.release_policies.find(
          (item) => item.release_sequence === release.sequence,
        );
        const releaseView = view.releases.find(
          (item) => item.release_sequence === release.sequence,
        );
        if (!policy || !releaseView) continue;
        for (const target of policy.targets) {
          if (releaseView.materialized_clients.includes(target.client)) continue;
          const id = 'test/reviewed-external-materialization';
          raw.evidence.push({
            id,
            distribution_id: distribution.id,
            release_sequence: release.sequence,
            level: 'materialization',
            outcome: 'passed',
            client: target.client,
            client_version: 'isolated-configuration-fixture',
            installer_version: '0.1.87',
            os: 'linux',
            architecture: 'amd64',
            observed_at: '2026-10-19T20:29:14Z',
            package_tree_digest: release.tree_digest,
            artifact: {
              repository: '777genius/universal-agent-plugins-registry',
              revision: 'a'.repeat(40),
              path: 'evidence/test.json',
              digest: `sha256:${'b'.repeat(64)}`,
            },
            trust: { kind: 'reviewed_external' },
          });
          policy.current_evidence.push(id);
          const parsed = parseDirectoryData(fixture, 'published_snapshot');
          const parsedView = parsed.plugins
            .find((item) => item.distributions.some((view) => view.id === distribution.id))!
            .distributions.find((item) => item.id === distribution.id)!;
          const parsedRelease = parsedView.releases.find(
            (item) => item.release_sequence === release.sequence,
          )!;
          assert.ok(
            parsedRelease.materialized_clients.includes(target.client),
            'reviewed-external passed materialization counts for eligibility like the signed resolver',
          );
          promoted = true;
          break outer;
        }
      }
    }
    assert.ok(promoted, 'the feed has a target without materialization evidence to promote');
  });

  it('resolves the declared default once reviewed materialization evidence is trusted', () => {
    const context7 = registry.plugins.find((plugin) => plugin.name === 'context7')!;
    assert.ok(
      context7.distributions.length > 1,
      'context7 has multiple distributions in the Directory feed',
    );
    const resolved = resolveDistribution(context7, ['cursor']);
    assert.equal(resolved.distribution?.id, context7.declared_default_distribution);
    assert.equal(resolved.fallback_reason, undefined);
  });

  it('intro counts collapse duplicate sources instead of counting raw records', () => {
    const reviewed = registry.plugins.find(
      (plugin) => plugin.installable && catalogVisiblePlugins([plugin]).length,
    )!;
    const alias = {
      ...reviewed,
      install_source: 'discovery:alias/plugin',
      trust_state: 'conformant_unreviewed' as const,
      source: { ...reviewed.source, repository: 'alias/plugin' },
      distributions: reviewed.distributions.map((item) => ({
        ...item,
        id: `${item.id}-alias`,
      })),
    };
    const community = {
      ...alias,
      install_source: 'discovery:solo/plugin',
      name: 'solo-community',
      source: { ...reviewed.source, repository: 'solo/plugin' },
    };
    const counts = catalogTrustCounts([reviewed, alias, community]);
    assert.equal(counts.total, 2);
    assert.equal(counts.reviewed, 1);
    assert.equal(counts.community, 1);
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
    assert.match(commands.add, /^npx -y universal-agent-plugins add /);
    for (const action of ['add', 'update', 'repair', 'switch', 'remove'] as const)
      assert.match(commands[action], /^npx -y universal-agent-plugins /);
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

  it('reserves a local logo for the newly published marketing kit', () => {
    const plugin = { ...registry.plugins[0]!, name: 'marketing-kit', built_in: true };
    assert.equal(mirroredIconPath(plugin), 'plugin-icons/marketing-kit.svg');
    const svg = readFileSync(resolve(root, 'public/plugin-icons/marketing-kit.svg'), 'utf8');
    assert.match(svg, /<svg\b[^>]*\bviewBox=/);
    assert.doesNotMatch(
      svg,
      /<(?:script|foreignObject|image)\b|\bon\w+\s*=|(?:href|xlink:href)\s*=/i,
    );
    assert.equal(mirroredIconPath({ ...plugin, built_in: false }), undefined);
  });

  it('gives every reviewed plugin its own local plugin or publisher logo', () => {
    const publisherAliases: Record<string, string> = {
      'chrome-devtools': 'googlechrome',
      'docker-hub': 'docker',
    };
    for (const plugin of registry.plugins) {
      const iconPath = mirroredIconPath(plugin);
      assert.ok(iconPath, `${plugin.name} must have a local logo`);
      const publisher = plugin.name.startsWith('cloudflare-')
        ? 'cloudflare'
        : plugin.name.startsWith('hubspot-')
          ? 'hubspot'
          : (publisherAliases[plugin.name] ?? plugin.name);
      assert.equal(iconPath, `plugin-icons/${publisher}.svg`, `${plugin.name} logo identity`);
      assert.doesNotThrow(
        () => readFileSync(resolve(root, 'public', iconPath)),
        `${plugin.name} logo must exist`,
      );
      const svg = readFileSync(resolve(root, 'public', iconPath), 'utf8');
      assert.match(svg, /<svg\b[^>]*\bviewBox="[^"]+"/, `${plugin.name} scalable logo`);
      assert.match(svg, /<(?:path|polygon|circle|rect)\b/, `${plugin.name} visible artwork`);
      assert.doesNotMatch(
        svg,
        /<(?:script|foreignObject|image)\b|\bon\w+\s*=|(?:href|xlink:href)\s*=|url\(['"]?https?:/i,
        `${plugin.name} logo has no executable or external content`,
      );
    }

    assert.equal(
      mirroredIconPath(registry.plugins.find((plugin) => plugin.name === 'agent-code-navigator')!),
      'plugin-icons/agent-code-navigator.svg',
    );
    assert.equal(
      mirroredIconPath(registry.plugins.find((plugin) => plugin.name === 'statsig')!),
      'plugin-icons/statsig.svg',
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

  it('does not let external or unknown packages select a local logo', () => {
    const reviewed = registry.plugins.find((plugin) => plugin.name === 'github')!;
    const external = discoveryPlugin({ ...discoveryRecord, name: 'github' }, discoverySnapshot);
    assert.equal(mirroredIconPath({ ...external, icon: reviewed.icon }), undefined);
    assert.equal(
      mirroredIconPath({ ...reviewed, name: 'unknown', icon: reviewed.icon }),
      undefined,
    );
    assert.equal(
      mirroredIconPath({
        ...reviewed,
        icon: { ...reviewed.icon!, path: 'assets/plugin-icons/stripe.svg' },
      }),
      'plugin-icons/github.svg',
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

  it('keeps retired registry mirrors out of the catalog without removing current sources', () => {
    const reviewed = registry.plugins.find((plugin) => catalogVisiblePlugins([plugin]).length)!;
    const current = discoveryPlugin(
      {
        ...discoveryRecord,
        slug: 'discovery:example/current',
        components: { extensions: 0, mcp: 0, skills: 1 },
        compatible_clients: ['codex'],
      },
      discoverySnapshot,
    );
    const retired = {
      ...current,
      install_source:
        'discovery:777genius/universal-agent-plugins-registry//plugins/agent-code-navigator',
      source: {
        ...current.source,
        repository: '777genius/universal-agent-plugins-registry',
        path: 'plugins/agent-code-navigator',
      },
    };

    assert.deepEqual(catalogVisiblePlugins([retired, current, reviewed]), [current, reviewed]);
  });

  it('hides only fully retired products; suspended ones stay in the list', () => {
    const reviewed = registry.plugins.find((plugin) => catalogVisiblePlugins([plugin]).length)!;
    const retired = {
      ...reviewed,
      name: 'retired-product',
      distributions: reviewed.distributions.map((distribution) => ({
        ...distribution,
        releases: distribution.releases.map((release) => ({
          ...release,
          release_status: 'revoked' as const,
        })),
      })),
    };
    const paused = {
      ...reviewed,
      name: 'paused-product',
      distributions: reviewed.distributions.map((distribution) => ({
        ...distribution,
        releases: distribution.releases.map((release, index) => ({
          ...release,
          release_status: (index === 0 ? 'active' : 'revoked') as 'active' | 'revoked',
        })),
      })),
    };

    assert.deepEqual(catalogVisiblePlugins([retired, paused, reviewed]), [paused, reviewed]);
  });

  it('offers the vendor installer override for agent-notifications only', () => {
    const reviewed = registry.plugins[0]!;
    assert.deepEqual(vendorInstallSource({ ...reviewed, name: 'agent-notifications' }), {
      url: 'https://agent-notifications.com/#install',
      command: '(set -o pipefail; curl -fsSL https://agent-notifications.com/install.sh | bash)',
    });
    assert.equal(vendorInstallSource(reviewed), undefined);
  });
});
