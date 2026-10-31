import assert from 'node:assert/strict';
import { describe, it } from 'node:test';
import { authenticationIndicator } from '../utils/authentication.ts';
import type { DistributionView } from '../types/registry.ts';
const release: Pick<DistributionView, 'id' | 'release_sequence' | 'targets'> = {
  id: 'upstream/example',
  release_sequence: 3,
  targets: [
    { client: 'codex', authentication: 'required', delivery: 'managed', scopes: ['user'] },
    { client: 'cursor', authentication: 'not_required', delivery: 'managed', scopes: ['user'] },
    { client: 'kiro', authentication: 'unknown', delivery: 'managed', scopes: ['user'] },
  ],
};
const t = (key: string, params?: Record<string, string | number>) =>
  `${key}${params ? JSON.stringify(params) : ''}`;
const groupStates = (indicator: ReturnType<typeof authenticationIndicator>) =>
  indicator.groups.map((group) => [group.authentication, group.clients]);
describe('card authentication indicator', () => {
  it('uses resolved release target policy instead of contradictory legacy hints', () => {
    const plugin = { authentication: 'none', distribution: release };
    assert.equal(
      authenticationIndicator(plugin.distribution, ['codex'], false, t).state,
      'required',
    );
    const legacyRequired = { authentication: 'oauth', distribution: release };
    assert.equal(
      authenticationIndicator(legacyRequired.distribution, ['cursor'], false, t).state,
      'not_required',
    );
    assert.match(
      authenticationIndicator(release, ['cursor'], false, t).release,
      /upstream\/example/,
    );
  });
  it('resolves automatic selection across every supported agent', () => {
    const uniform = {
      ...release,
      targets: release.targets.map((target) => ({
        ...target,
        authentication: 'not_required' as const,
      })),
    };
    const uniformResult = authenticationIndicator(uniform, ['cursor'], true, t);
    assert.equal(uniformResult.state, 'not_required');
    assert.equal(uniformResult.varies, false);
    assert.deepEqual(groupStates(uniformResult), [
      ['not_required', ['codex', 'cursor', 'kiro']],
    ]);
    const required = {
      ...release,
      targets: release.targets.map((target) => ({
        ...target,
        authentication: 'required' as const,
      })),
    };
    assert.equal(authenticationIndicator(required, ['cursor'], true, t).state, 'required');
    const mixed = authenticationIndicator(release, ['cursor'], true, t);
    assert.equal(mixed.state, 'unknown');
    assert.equal(mixed.varies, true);
    assert.match(mixed.label, /varies/);
    assert.deepEqual(groupStates(mixed), [
      ['required', ['codex']],
      ['unknown', ['kiro']],
      ['not_required', ['cursor']],
    ]);
    assert.equal(mixed.summary, 'registryUi.authenticationIndicator.unknownDetail');
    assert.equal(mixed.note, 'registryUi.authenticationIndicator.automatic');
  });
  it('keeps unresolved selections unknown', () => {
    const automatic = authenticationIndicator(undefined, ['cursor'], true, t);
    assert.equal(automatic.state, 'unknown');
    assert.equal(automatic.summary, 'registryUi.authenticationIndicator.automatic');
    assert.deepEqual(automatic.groups, []);
    const manual = authenticationIndicator(undefined, ['cursor'], false, t);
    assert.equal(manual.state, 'unknown');
    assert.equal(manual.summary, 'registryUi.authenticationIndicator.unresolved');
    assert.equal(authenticationIndicator(release, [], false, t).state, 'unknown');
  });
  it('keeps per-agent policy disagreements visible for automatic installs', () => {
    const divergent = {
      id: 'example',
      targets: [
        { client: 'cursor', authentication: 'not_required', delivery: 'managed', scopes: ['user'] },
        { client: 'cursor', authentication: 'required', delivery: 'managed', scopes: ['user'] },
      ],
    } as Pick<DistributionView, 'id' | 'release_sequence' | 'targets'>;
    const result = authenticationIndicator(divergent, ['codex'], true, t);
    assert.equal(result.state, 'unknown');
    assert.equal(result.varies, true);
    assert.match(result.label, /varies/);
    assert.deepEqual(groupStates(result), [
      ['required', ['cursor']],
      ['not_required', ['cursor']],
    ]);
    assert.equal(result.note, 'registryUi.authenticationIndicator.automatic');
  });
  it('explains mixed and unknown requirements without an open-lock claim', () => {
    const mixed = authenticationIndicator(release, ['cursor', 'codex'], false, t);
    assert.equal(mixed.state, 'unknown');
    assert.equal(mixed.varies, true);
    assert.deepEqual(groupStates(mixed), [
      ['required', ['codex']],
      ['not_required', ['cursor']],
    ]);
    const unknown = authenticationIndicator(release, ['kiro'], false, t);
    assert.equal(unknown.state, 'unknown');
    assert.equal(unknown.varies, false);
    assert.deepEqual(groupStates(unknown), [['unknown', ['kiro']]]);
  });
});
