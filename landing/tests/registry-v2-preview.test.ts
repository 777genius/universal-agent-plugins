import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { describe, it } from 'node:test';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseDirectoryData, reachableTargetPolicies, resolveDistribution } from '../utils/registry.ts';

type RawGrant = {
  client: string;
  scopes?: string[];
  scope?: string;
  current_evidence?: string[];
  canonical_closure?: { eligible: boolean };
  [key: string]: unknown;
};
type Fixture = {
  snapshot_schema_version: number;
  distributions: Array<{ release_policies: Array<{ targets: RawGrant[] }> }>;
};

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const pointer = JSON.parse(
  readFileSync(resolve(root, 'public/registry/schemas/1/latest.json'), 'utf8'),
) as { snapshot_path: string };
const publishedV1 = JSON.parse(
  readFileSync(resolve(root, 'public/registry/schemas/1', pointer.snapshot_path), 'utf8'),
) as Fixture;

function scopedV2Preview(): Fixture {
  const fixture = structuredClone(publishedV1);
  fixture.snapshot_schema_version = 2;
  let projectCreated = false;
  for (const distribution of fixture.distributions) {
    for (const policy of distribution.release_policies) {
      const targets: RawGrant[] = [];
      for (const target of policy.targets) {
        const scoped = { ...target, scope: 'user', current_evidence: [] };
        delete scoped.scopes;
        targets.push(scoped);
        if (!projectCreated && target.client === 'codex') {
          targets.push({
            ...scoped,
            scope: 'project',
            canonical_closure: { eligible: false },
          });
          projectCreated = true;
        }
      }
      policy.targets = targets;
    }
  }
  assert.ok(projectCreated, 'v1 fixture must contain Codex before v2 conversion');
  return fixture;
}

describe('Directory v2 untrusted preview boundary', () => {
  it('keeps project and user grants distinct without making v2 publicly installable', () => {
    const fixture = scopedV2Preview();
    const preview = parseDirectoryData(fixture, 'review_preview');
    const targets = preview.plugins.flatMap((plugin) =>
      plugin.distributions.flatMap((distribution) =>
        distribution.releases.flatMap((release) => release.targets),
      ),
    );
    assert.ok(targets.some((target) =>
      target.client === 'codex' && target.scopes.includes('project') &&
      target.canonical_closure?.eligible === false));
    assert.ok(targets.some((target) =>
      target.client === 'codex' && target.scopes.includes('user') &&
      target.canonical_closure === undefined));
    assert.ok(preview.plugins.every((plugin) =>
      plugin.trust_state === undefined && !plugin.installable));
    assert.ok(preview.plugins.every((plugin) =>
      Object.keys(plugin.client_support.delivery).length === 0 &&
      Object.keys(plugin.client_support.scopes).length === 0 &&
      Object.keys(plugin.client_support.app_bindings).length === 0 &&
      plugin.authentication === 'unknown' &&
      plugin.evidence.length === 0 &&
      plugin.package_evidence.length === 0 &&
      plugin.distributions.every((distribution) =>
        distribution.releases.every((release) =>
          release.evidence.length === 0 &&
          release.package_evidence.length === 0 &&
          release.blocking_clients.length === 0 &&
          release.materialized_clients.length === 0))),
    'v2 preview must not present unverified per-scope results as client-wide claims');
    // Both exported selection APIs must refuse preview policies, even for
    // community distributions that require no upstream materialization proof.
    for (const plugin of preview.plugins) {
      assert.deepEqual(reachableTargetPolicies(plugin.distributions), []);
      for (const distribution of plugin.distributions) {
        for (const release of distribution.releases) {
          for (const target of release.targets) {
            for (const scope of target.scopes) {
              if (scope === 'user' || scope === 'project') {
                assert.equal(resolveDistribution(plugin, [target.client], scope).distribution,
                  undefined, `${plugin.name} ${target.client}/${scope} preview was selectable`);
              }
            }
          }
        }
      }
    }
    assert.throws(() => parseDirectoryData(fixture, 'published_snapshot'),
      /schema version 1|published snapshot/);
    // Mixed-format preview data must not downgrade through the legacy-index
    // decoder, whose output is intentionally installable for old fixtures.
    assert.throws(() => parseDirectoryData({
      ...fixture, schema_version: 1, plugins: [],
    }, 'review_preview'), /legacy index cannot contain Directory snapshot fields/);
  });

  it('refuses v2 grants that try to borrow legacy arrays or assert unproven closure', () => {
    const legacy = scopedV2Preview();
    const project = legacy.distributions.flatMap((d) =>
      d.release_policies.flatMap((p) => p.targets),
    ).find((target) => target.scope === 'project')!;
    project.scopes = ['user', 'project'];
    assert.throws(() => parseDirectoryData(legacy, 'review_preview'),
      /legacy scopes/);

    const unproven = scopedV2Preview();
    const grant = unproven.distributions.flatMap((d) =>
      d.release_policies.flatMap((p) => p.targets),
    ).find((target) => target.scope === 'project')!;
    grant.canonical_closure = { eligible: true };
    assert.throws(() => parseDirectoryData(unproven, 'review_preview'),
      /closure_digest|invalid eligible v2 closure/);
  });
});
