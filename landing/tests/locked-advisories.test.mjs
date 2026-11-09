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
snapshots:
  pnpm@12.3.4: {}
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

test('includes root optional dependencies in production reachability', () => {
  const withOptional = fixture.replace('    devDependencies:',
    '    optionalDependencies:\n      optional-package:\n        version: 1.0.0\n    devDependencies:')
    .replace('  glob@10.5.0: {}', '  optional-package@1.0.0:\n    dependencies:\n      minimatch: 9.0.9\n  minimatch@9.0.9: {}\n  glob@10.5.0: {}');
  assert.ok(auditLock(withOptional).some(({ scope, package: pkg }) =>
    scope === 'dependencies' && pkg === 'minimatch@9.0.9'));
});

test('fails closed if one importer dependency has no pinned version, even when other roots exist', () => {
  const missingVersion = fixture.replace('        version: 1.0.0(peer@2.0.0)', '        resolution: 1.0.0(peer@2.0.0)');
  assert.throws(() => auditLock(missingVersion), /Missing pnpm importer version: @a\/entry/);
});

test('fails closed if final root dependency lacks version before snapshots', () => {
  const missingLast = fixture.replace('        version: 1.0.0\nsnapshots:', '        specifier-only: 1.0.0\nsnapshots:');
  assert.throws(() => auditLock(missingLast), /Missing pnpm importer version: test-tool/);
});

test('refuses a different lockfile schema even if graph keys look familiar', () => {
  const future = fixture.replaceAll("lockfileVersion: '9.0'", "lockfileVersion: '10.0'");
  assert.throws(() => auditLock(future), /Unsupported pnpm lockfile version/);
  assert.throws(() => auditLock(fixture.replaceAll("lockfileVersion: '9.0'\n", '')), /Unsupported pnpm lockfile version/);
});

test('includes dependencies from every pnpm v9 importer, not only the top-level project', () => {
  const multiImporter = fixture.replace("snapshots:\n  '@a/entry@1.0.0(peer@2.0.0)':",
    '  packages/site:\n    dependencies:\n      minimatch:\n        specifier: 9.0.9\n        version: 9.0.9\nsnapshots:\n  \'@a/entry@1.0.0(peer@2.0.0)\':')
    .replace('  minimatch@3.1.5: {}', '  minimatch@9.0.9: {}\n  minimatch@3.1.5: {}');
  const findings = auditLock(multiImporter);
  assert.ok(findings.some(({ scope, package: pkg, path }) =>
    scope === 'dependencies' && pkg === 'minimatch@9.0.9' && path[0] === 'minimatch@9.0.9'));
});

test('rejects inline snapshot dependency maps instead of omitting vulnerable children', () => {
  for (const category of ['dependencies', 'optionalDependencies']) {
    const inline = fixture.replace('    dependencies:\n      glob: 13.0.6\n',
      `    ${category}: { glob: 13.0.6 }\n`);
    assert.throws(() => auditLock(inline), /Unsupported inline pnpm map/);
  }
});

test('rejects inline snapshot nodes instead of treating reachable parents as leaf nodes', () => {
  const inline = fixture.replace("  '@a/entry@1.0.0(peer@2.0.0)':\n    dependencies:\n      glob: 13.0.6\n",
    "  '@a/entry@1.0.0(peer@2.0.0)': { dependencies: { glob: 13.0.6 } }\n");
  assert.throws(() => auditLock(inline), /Unsupported inline pnpm map/);
});

test('rejects inline importer dependency maps instead of missing production roots', () => {
  const inline = fixture.replace("    dependencies:\n      '@a/entry':\n        specifier: ^1.0.0\n        version: 1.0.0(peer@2.0.0)\n",
    "    dependencies: { '@a/entry': { version: 1.0.0(peer@2.0.0) } }\n");
  assert.throws(() => auditLock(inline), /Unsupported inline pnpm map/);
});

test('scans config dependencies in the pnpm 12 boot document', () => {
  const withConfig = fixture.replace('    packageManagerDependencies:',
    '    configDependencies:\n      config-plugin:\n        version: 1.0.0\n    packageManagerDependencies:')
    .replace('  pnpm@12.3.4: {}', '  config-plugin@1.0.0:\n    dependencies:\n      glob: 13.0.6\n  glob@13.0.6: {}\n  pnpm@12.3.4: {}');
  assert.ok(auditLock(withConfig).some(({ scope, package: pkg }) =>
    scope === 'configDependencies' && pkg === 'glob@13.0.6'));
});

test('scans package-manager dependencies instead of ignoring the pnpm boot graph', () => {
  const withManager = fixture.replace('  pnpm@12.3.4: {}',
    '  pnpm@12.3.4:\n    dependencies:\n      glob: 13.0.6\n  glob@13.0.6: {}');
  assert.ok(auditLock(withManager).some(({ scope, package: pkg }) =>
    scope === 'packageManagerDependencies' && pkg === 'glob@13.0.6'));
});

test('does not overwrite boot snapshot edges with matching keys in the project document', () => {
  const colliding = fixture.replace('    packageManagerDependencies:',
    '    configDependencies:\n      test-tool:\n        version: 1.0.0\n    packageManagerDependencies:')
    .replace('  pnpm@12.3.4: {}',
      '  test-tool@1.0.0:\n    dependencies:\n      glob: 13.0.6\n  glob@13.0.6: {}\n  pnpm@12.3.4: {}');
  assert.ok(auditLock(colliding).some(({ scope, package: pkg }) =>
    scope === 'configDependencies' && pkg === 'glob@13.0.6'));
});
