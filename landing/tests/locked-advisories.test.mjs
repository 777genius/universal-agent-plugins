import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
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

test('rejects duplicate snapshot mappings instead of hiding a reachable vulnerable child', () => {
  const duplicate = `lockfileVersion: '9.0'\nimporters:\n  .:\n    dependencies:\n      app:\n        version: 1.0.0\nsnapshots:\n  app@1.0.0:\n    dependencies:\n      glob: 13.0.6\n  app@1.0.0: {}\n  glob@13.0.6: {}\n`;
  assert.throws(() => auditLock(duplicate), /Duplicate pnpm snapshot: app@1\.0\.0/);
});

test('rejects YAML-valid alternate snapshot indentation instead of silently omitting vulnerable edges', () => {
  const alternate = fixture.replace('    dependencies:\n      glob: 13.0.6\n',
    '   dependencies:\n     glob: 13.0.6\n');
  assert.notEqual(alternate, fixture);
  assert.throws(() => auditLock(alternate), /Unsupported pnpm indentation in snapshots/);
});

test('ignores unusually indented YAML comments without changing graph reachability', () => {
  const commented = fixture.replace('  glob@13.0.6:\n', '   # comment at arbitrary indentation\n  glob@13.0.6:\n');
  assert.deepEqual(auditLock(commented), auditLock(fixture));
});

test('recognizes YAML-equivalent category spaces before a snapshot colon', () => {
  const alternate = fixture.replace('    dependencies:\n      glob: 13.0.6\n',
    '    dependencies  :\n      glob: 13.0.6\n');
  assert.notEqual(alternate, fixture);
  assert.deepEqual(auditLock(alternate), auditLock(fixture));
});

test('recognizes YAML-equivalent category spaces before an importer colon', () => {
  const alternate = fixture.replace("    dependencies:\n      '@a/entry':\n",
    "    dependencies :\n      '@a/entry':\n");
  assert.notEqual(alternate, fixture);
  assert.deepEqual(auditLock(alternate), auditLock(fixture));
});

test('rejects YAML sequence dependency mappings instead of hiding vulnerable edges', () => {
  const snapshotSequence = fixture.replace('    dependencies:\n      glob: 13.0.6\n',
    '    dependencies:\n      - glob@13.0.6\n');
  assert.throws(() => auditLock(snapshotSequence), /Unsupported pnpm dependency sequence/);

  const importerSequence = fixture.replace("    dependencies:\n      '@a/entry':\n        specifier: ^1.0.0\n        version: 1.0.0(peer@2.0.0)\n",
    "    dependencies:\n      - '@a/entry@1.0.0(peer@2.0.0)'\n");
  assert.throws(() => auditLock(importerSequence), /Unsupported pnpm dependency sequence/);
});


test('rejects YAML-escaped glob package names instead of returning a false-clean advisory result', () => {
  const disguised = fixture
    .replace('      glob: 13.0.6', String.raw`      "\u0067lob": 13.0.6`)
    .replace('  glob@13.0.6:', String.raw`  "\u0067lob@13.0.6":`);
  assert.throws(() => auditLock(disguised), /Unsupported escaped YAML scalar/);
});

test('rejects YAML-escaped minimatch package names instead of omitting the vulnerable edge', () => {
  const disguised = fixture
    .replace('      minimatch: 10.2.6', String.raw`      "minim\u0061tch": 10.2.6`)
    .replace('  minimatch@10.2.6:', String.raw`  "minim\u0061tch@10.2.6":`);
  assert.throws(() => auditLock(disguised), /Unsupported escaped YAML scalar/);
});

test('rejects explicit YAML dependency keys instead of silently reporting a clean graph', () => {
  const explicitChild = fixture.replace('      glob: 13.0.6\n', '      ? glob\n      : 13.0.6\n');
  assert.notEqual(explicitChild, fixture);
  assert.throws(() => auditLock(explicitChild), /Unsupported explicit pnpm YAML mapping/);
});

test('rejects explicit YAML dependency categories instead of dropping all child edges', () => {
  const explicitCategory = fixture.replace('    dependencies:\n      glob: 13.0.6\n',
    '    ? dependencies\n    :\n      glob: 13.0.6\n');
  assert.notEqual(explicitCategory, fixture);
  assert.throws(() => auditLock(explicitCategory), /Unsupported explicit pnpm YAML mapping/);
});


test('preserves vulnerable boot importer reachability for YAML-valid top-level comments and quoted keys', () => {
  const boot = [
    "lockfileVersion: '9.0'", 'importers:', '  .:', '    packageManagerDependencies:',
    '      boot-tool:', '        version: 1.0.0', 'snapshots:', '  boot-tool@1.0.0:',
    '    dependencies:', '      glob: 13.0.6', '  glob@13.0.6: {}', '',
  ].join('\n');
  const project = [
    '---', "lockfileVersion: '9.0'", 'importers:', '  .:', '    dependencies:',
    '      clean-app:', '        version: 1.0.0', 'snapshots:', '  clean-app@1.0.0: {}', '',
  ].join('\n');
  for (const declaration of [
    'importers: # boot graph', 'importers :', "'importers':", '"importers":',
    'snapshots: # boot graph', 'snapshots :',
  ]) {
    const target = declaration.startsWith('snapshots') ? 'snapshots:' : 'importers:';
    const document = boot.replace(target, declaration) + project;
    assert.ok(auditLock(document).some(({ scope, package: pkg }) =>
      scope === 'packageManagerDependencies' && pkg === 'glob@13.0.6'), declaration);
  }
});

test('fails closed on unsupported pnpm section anchors instead of hiding the boot graph', () => {
  const anchored = fixture.replace('importers:\n', 'importers: &boot\n');
  assert.throws(() => auditLock(anchored), /Unsupported pnpm section declaration/);
});

test('rejects escaped YAML top-level keys in boot graph', () => {
  const boot = [
    "lockfileVersion: '9.0'", 'importers:', '  .:', '    packageManagerDependencies:',
    '      boot-tool:', '        version: 1.0.0', 'snapshots:', '  boot-tool@1.0.0:',
    '    dependencies:', '      glob: 13.0.6', '  glob@13.0.6: {}', '',
  ].join('\n');
  const project = [
    '---', "lockfileVersion: '9.0'", 'importers:', '  .:', '    dependencies:',
    '      clean-app:', '        version: 1.0.0', 'snapshots:', '  clean-app@1.0.0: {}', '',
  ].join('\n');
  const key = '"import' + String.fromCharCode(92) + 'u0065rs":';
  assert.throws(() => auditLock(boot.replace('importers:', key) + project),
    /Unsupported pnpm top-level YAML mapping/);
  const snapKey = '"snapsh' + String.fromCharCode(92) + 'u006fts":';
  assert.throws(() => auditLock(boot.replace('snapshots:', snapKey) + project),
    /Unsupported pnpm top-level YAML mapping/);
});

test('rejects explicit YAML top-level keys in boot graph', () => {
  const boot = [
    "lockfileVersion: '9.0'", 'importers:', '  .:', '    packageManagerDependencies:',
    '      boot-tool:', '        version: 1.0.0', 'snapshots:', '  boot-tool@1.0.0:',
    '    dependencies:', '      glob: 13.0.6', '  glob@13.0.6: {}', '',
  ].join('\n');
  const project = [
    '---', "lockfileVersion: '9.0'", 'importers:', '  .:', '    dependencies:',
    '      clean-app:', '        version: 1.0.0', 'snapshots:', '  clean-app@1.0.0: {}', '',
  ].join('\n');
  assert.throws(() => auditLock(boot.replace('importers:', '? importers\n:') + project),
    /Unsupported pnpm top-level YAML mapping/);
  assert.throws(() => auditLock(boot.replace('snapshots:', '? snapshots\n:') + project),
    /Unsupported pnpm top-level YAML mapping/);
});

test('rejects YAML-tagged or anchored mapping keys that conceal installed boot dependencies', () => {
  const boot = [
    "lockfileVersion: '9.0'", 'importers:', '  .:', '    packageManagerDependencies:',
    '      boot-tool:', '        version: 1.0.0', 'snapshots:', '  boot-tool@1.0.0:',
    '    dependencies:', '      glob: 13.0.6', '  glob@13.0.6: {}', '',
  ].join('\n');
  const cleanProject = [
    '---', "lockfileVersion: '9.0'", 'importers:', '  .:', '    dependencies:',
    '      clean-app:', '        version: 1.0.0', 'snapshots:', '  clean-app@1.0.0: {}', '',
  ].join('\n');
  assert.ok(auditLock(boot + cleanProject).some(({ package: name }) => name === 'glob@13.0.6'));
  for (const [original, disguised] of [
    ['importers:', '!!str importers:'],
    ['importers:', '&boot importers:'],
    ['snapshots:', '!!str snapshots:'],
    ['    packageManagerDependencies:', '    !!str packageManagerDependencies:'],
    ['    dependencies:', '    &edges dependencies:'],
  ]) {
    const input = boot.replace(original, disguised) + cleanProject;
    assert.throws(() => auditLock(input), /Unsupported pnpm YAML tag\/anchor key/, disguised);
  }
});

test('rejects YAML merge aliases that hide reachable snapshot children', () => {
  const mergedSnapshot = [
    "lockfileVersion: '9.0'", 'importers:', '  .:', '    dependencies:',
    '      app:', '        version: 1.0.0',
    'shared: &vulnerable', '  dependencies:', '    glob: 13.0.6',
    'snapshots:', '  app@1.0.0:', '    <<: *vulnerable', '  glob@13.0.6: {}', '',
  ].join('\n');
  assert.throws(() => auditLock(mergedSnapshot), /Unsupported pnpm YAML merge mapping/);
});

test('rejects YAML merge aliases that hide boot importer roots', () => {
  const mergedBoot = [
    "lockfileVersion: '9.0'", 'defaults: &vulnerable', '  dependencies:',
    '    app:', '      version: 1.0.0', 'importers:', '  .:',
    '    << : *vulnerable', 'snapshots:', '  app@1.0.0:',
    '    dependencies:', '      glob: 13.0.6', '  glob@13.0.6: {}',
    '---', "lockfileVersion: '9.0'", 'importers:', '  .:',
    '    dependencies:', '      safe:', '        version: 1.0.0',
    'snapshots:', '  safe@1.0.0: {}', '',
  ].join('\n');
  assert.throws(() => auditLock(mergedBoot), /Unsupported pnpm YAML merge mapping/);
});

test('colon-bearing YAML comments cannot hide snapshot dependency edges or importer roots', () => {
  const expected = auditLock(fixture);
  const cases = [
    fixture.replace(
      "  '@a/entry@1.0.0(peer@2.0.0)':\n",
      "  '@a/entry@1.0.0(peer@2.0.0)':\n  # misleading: snapshot\n",
    ),
    fixture.replace(
      '    dependencies:\n      glob: 13.0.6\n',
      '    dependencies:\n    # misleading: category\n      glob: 13.0.6\n',
    ),
    fixture.replace(
      "    dependencies:\n      '@a/entry':\n",
      "    dependencies:\n    # misleading: importer\n      '@a/entry':\n",
    ),
  ];
  for (const input of cases) {
    assert.notEqual(input, fixture, 'fixture must have changed');
    assert.deepEqual(auditLock(input), expected);
  }
});

// Test the checked-in lock as well as synthetic adversarial pnpm graph fixtures.
test('checked-in frozen landing lock contains no scoped advisory paths', () => {
  const lockfile = new URL('../pnpm-lock.yaml', import.meta.url);
  assert.deepEqual(auditLock(readFileSync(lockfile, 'utf8')), []);
});


test('finds scoped advisories in resolved peer qualifiers without child edges', () => {
  const withPeers = `lockfileVersion: '9.0'
importers:
  .:
    dependencies:
      consumer:
        version: 1.0.0(glob@13.0.6)(minimatch@9.0.9)
snapshots:
  consumer@1.0.0(glob@13.0.6)(minimatch@9.0.9): {}
  glob@13.0.6: {}
  minimatch@9.0.9: {}
`;
  assert.deepEqual(auditLock(withPeers).map((f) => [f.scope, f.package]), [
    ['dependencies', 'glob@13.0.6'],
    ['dependencies', 'minimatch@9.0.9'],
  ]);
});
test('includes nested peer qualifiers and does not flag patched peer packages', () => {
  const nested = `lockfileVersion: '9.0'
importers:
  .:
    dependencies:
      consumer:
        version: 1.0.0(another@2.0.0(minimatch@9.0.9))
snapshots:
  consumer@1.0.0(another@2.0.0(minimatch@9.0.9)): {}
  minimatch@9.0.9: {}
`;
  assert.deepEqual(auditLock(nested).map((f) => f.package), ['minimatch@9.0.9']);
  assert.deepEqual(auditLock(nested.replaceAll('minimatch@9.0.9', 'minimatch@11.0.5')), []);
});
