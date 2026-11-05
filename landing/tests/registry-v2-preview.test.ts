import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { describe, it } from 'node:test';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseDirectoryData } from '../utils/registry.ts';

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
    assert.throws(() => parseDirectoryData(fixture, 'published_snapshot'),
      /schema version 1|published snapshot/);
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
