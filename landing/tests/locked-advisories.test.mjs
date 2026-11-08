import assert from 'node:assert/strict';
import { test } from 'node:test';
import { auditLock, parsePnpmLock, vulnerabilityFor } from '../scripts/check-locked-advisories.mjs';

const fixture = `---
lockfileVersion: '9.0'
importers:
  .:
    packageManagerDependencies:
      pnpm:
        version: 12.3.4
---
lockfileVersion: '9.0'
importers:
  .:
    dependencies:
      '@a/entry':
        specifier: ^1.0.0
        version: 1.0.0(peer@2.0.0)
    devDependencies:
      test-tool:
        specifier: 1.0.0
        version: 1.0.0
snapshots:
  '@a/entry@1.0.0(peer@2.0.0)':
    dependencies:
      glob: 13.0.6
  glob@13.0.6:
    dependencies:
      minimatch: 10.2.6
  minimatch@10.2.6: {}
  test-tool@1.0.0:
    dependencies:
      minimatch: 3.1.5
      glob-alias: glob@10.5.0
  glob@10.5.0: {}
  minimatch@3.1.5: {}
`;

test('finds advisory reachability through real root and peer-qualified snapshots', () => {
  const findings = auditLock(fixture);
  assert.deepEqual(findings.map(({ scope, package: name }) => [scope, name]), [
    ['dependencies', 'glob@13.0.6'], ['dependencies', 'minimatch@10.2.6'],
    ['devDependencies', 'glob@10.5.0'],
  ]);
  assert.deepEqual(findings[0].path, ['@a/entry@1.0.0(peer@2.0.0)', 'glob@13.0.6']);
});

test('does not flag unaffected 3/5 or fixed 11/14 majors', () => {
  for (const key of ['minimatch@3.1.5', 'minimatch@5.1.9', 'minimatch@11.0.1', 'glob@14.0.2']) {
    assert.equal(vulnerabilityFor(key), null, key);
  }
  assert.equal(vulnerabilityFor('glob@10.5.0'), 'GHSA-3g3m-2fjr-6pw4');
  assert.equal(vulnerabilityFor('minimatch@9.0.9'), 'GHSA-g3hv-75wg-m249');
});

test('fails closed for a lockfile without snapshots', () => {
  assert.throws(() => parsePnpmLock('importers:\n  .:\n    dependencies: {}\n'));
});

test('refuses to report false clean when snapshots are missing', () => {
  assert.throws(() => auditLock(fixture.replace('  glob@10.5.0: {}\n', '')), /Unresolved pnpm dependency edge/);
});

test('fails closed on unrecognized vulnerable-version syntax', () => {
  assert.throws(() => vulnerabilityFor('glob@next'), /Unsupported advisory package version/);
});
