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
      authenticationIndicator(release, ['cursor'], false, t).detail,
      /upstream\/example/,
    );
  });
  it('keeps automatic and unresolved selection unknown', () => {
    assert.equal(authenticationIndicator(release, ['cursor'], true, t).state, 'unknown');
    assert.match(authenticationIndicator(release, ['cursor'], true, t).detail, /automatic/);
    assert.equal(authenticationIndicator(undefined, ['cursor'], false, t).state, 'unknown');
    assert.equal(authenticationIndicator(release, [], false, t).state, 'unknown');
  });
  it('explains mixed and unknown requirements without an open-lock claim', () => {
    const mixed = authenticationIndicator(release, ['cursor', 'codex'], false, t);
    assert.equal(mixed.state, 'unknown');
    assert.match(mixed.label, /varies/);
    assert.match(mixed.detail, /cursor: .*not_required; codex: .*required/);
    assert.equal(authenticationIndicator(release, ['kiro'], false, t).state, 'unknown');
  });
});
