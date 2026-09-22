const assert = require('node:assert/strict');
const { spawnSync } = require('node:child_process');
const { mkdtempSync, readFileSync, rmSync, writeFileSync } = require('node:fs');
const { tmpdir } = require('node:os');
const path = require('node:path');
const test = require('node:test');
const { assertLintPolicy, assertRequiredRoute, classifySourcePath } = require('./check-quality-scope.js');

const repositoryRoot = path.resolve(__dirname, '../..');
const manifest = JSON.parse(readFileSync(path.join(repositoryRoot, 'npm/agentplugins/package.json'), 'utf8'));
const qualityManifest = JSON.parse(readFileSync(path.join(repositoryRoot, 'npm/quality/package.json'), 'utf8'));
const declarationConfig = JSON.parse(readFileSync(path.join(repositoryRoot, 'npm/plugin-kit-ai-runtime/tsconfig.docs.json'), 'utf8'));
const makefile = readFileSync(path.join(repositoryRoot, 'Makefile'), 'utf8');
const config = JSON.parse(readFileSync(path.join(repositoryRoot, 'npm/agentplugins/oxlint.json'), 'utf8'));

test('npm source classification covers product, tooling, release surfaces, tests, and declarations', () => {
  assert.equal(classifySourcePath('npm/agentplugins/lib/platform.js'), 'production');
  assert.equal(classifySourcePath('npm/agentplugins/scripts/stage-release.js'), 'tooling');
  assert.equal(classifySourcePath('npm/quality/check-quality-scope.js'), 'tooling');
  assert.equal(classifySourcePath('npm/quality/check-quality-scope.test.js'), 'test');
  assert.equal(classifySourcePath('scripts/terminal-ui/stage_npm.js'), 'tooling');
  assert.equal(classifySourcePath('examples/local/codex-node-local/plugin/main.mjs'), 'release-surface');
  assert.equal(classifySourcePath('npm/agentplugins/test/verifier.test.js'), 'test');
  assert.equal(classifySourcePath('npm/plugin-kit-ai-runtime/index.d.ts'), 'declaration');
  assert.equal(classifySourcePath('npm/new-owner/source.tsx'), null);
});

test('actual npm quality route rejects comments, ignored errors, and removed commands', () => {
  assert.doesNotThrow(() => assertRequiredRoute(manifest, qualityManifest, makefile, declarationConfig));
  const testCommand = '\tcd npm/agentplugins && npm test && npm pack --dry-run --ignore-scripts';
  assert.throws(() => assertRequiredRoute(manifest, qualityManifest, makefile.replace(testCommand, `\t# ${testCommand.trim()}`), declarationConfig), /uncommented/u);
  assert.throws(() => assertRequiredRoute(manifest, qualityManifest, makefile.replace(testCommand, `\t-${testCommand.trim()}`), declarationConfig), /fail-closed/u);
  assert.throws(() => assertRequiredRoute(manifest, qualityManifest, makefile.replace(testCommand, '\ttrue'), declarationConfig), /complete/u);
  assert.throws(() => assertRequiredRoute(manifest, qualityManifest, makefile.replace('npm/plugin-kit-ai/lib', 'npm/plugin-kit-ai/omitted'), declarationConfig), /complete/u);
  assert.throws(() => assertRequiredRoute(manifest, qualityManifest, makefile.replace('test-agentplugins-js:', 'ifdef SKIP_QUALITY\ntest-agentplugins-js:').replace('\n\ntest-agentplugins-native-install:', '\nendif\n\ntest-agentplugins-native-install:'), declarationConfig), /complete/u);
  assert.throws(() => assertRequiredRoute(manifest, qualityManifest, makefile.replace('test-agentplugins-js:', ' ifdef SKIP_QUALITY\ntest-agentplugins-js:').replace('\n\ntest-agentplugins-native-install:', '\n endif\n\ntest-agentplugins-native-install:'), declarationConfig), /complete/u);
  assert.throws(() => assertRequiredRoute(manifest, qualityManifest, `${makefile}\ntest-agentplugins-js:\n\ttrue\n`, declarationConfig), /complete/u);
  assert.throws(() => assertRequiredRoute(manifest, qualityManifest, `${makefile}\ntest-agentplugins-js: # replacement\n\ttrue\n`, declarationConfig), /complete/u);
  assert.throws(() => assertRequiredRoute(manifest, qualityManifest, `${makefile}\ntest-agentplugins-js extra-target:\n\ttrue\n`, declarationConfig), /complete/u);
  assert.throws(() => assertRequiredRoute(manifest, qualityManifest, `${makefile}\nextra-target test-agentplugins-js:\n\ttrue\n`, declarationConfig), /complete/u);
  assert.throws(() => assertRequiredRoute(manifest, qualityManifest, makefile, { ...declarationConfig, compilerOptions: { ...declarationConfig.compilerOptions, skipLibCheck: true } }), /skipLibCheck/u);
  assert.throws(() => assertRequiredRoute({ ...manifest, devDependencies: {} }, qualityManifest, makefile, declarationConfig), /published package/u);
});

test('declaration gate rejects an invalid declaration consumed by TypeScript', () => {
  const fixtureRoot = mkdtempSync(path.join(tmpdir(), 'uap-declaration-gate-'));
  try {
    writeFileSync(path.join(fixtureRoot, 'invalid.d.ts'), 'export declare const invalidExport: MissingExportedType;\n');
    writeFileSync(path.join(fixtureRoot, 'consumer.ts'), "import { invalidExport } from './invalid.js';\nvoid invalidExport;\n");
    writeFileSync(path.join(fixtureRoot, 'tsconfig.json'), JSON.stringify({
      compilerOptions: declarationConfig.compilerOptions,
      include: ['invalid.d.ts', 'consumer.ts'],
    }));
    const result = spawnSync(process.execPath, [path.join(repositoryRoot, 'npm/quality/node_modules/typescript/bin/tsc'), '-p', path.join(fixtureRoot, 'tsconfig.json'), '--noEmit'], { encoding: 'utf8' });
    assert.notEqual(result.status, 0);
    assert.match(`${result.stdout}\n${result.stderr}`, /MissingExportedType/u);
    const skipped = spawnSync(process.execPath, [path.join(repositoryRoot, 'npm/quality/node_modules/typescript/bin/tsc'), '-p', path.join(fixtureRoot, 'tsconfig.json'), '--noEmit', '--skipLibCheck'], { encoding: 'utf8' });
    assert.equal(skipped.status, 0, `${skipped.stdout}\n${skipped.stderr}`);
  } finally {
    rmSync(fixtureRoot, { recursive: true, force: true });
  }
});

test('npm quality policy rejects disabled protected rules and ignored product roots', () => {
  assert.doesNotThrow(() => assertLintPolicy(config));
  assert.throws(() => assertLintPolicy({ ...config, rules: { ...config.rules, 'no-eval': 'off' } }), /protected rule/u);
  assert.throws(() => assertLintPolicy({ ...config, ignorePatterns: ['lib/**'] }), /cannot add overrides/u);
  assert.throws(() => assertLintPolicy({ ...config, overrides: [] }), /cannot add overrides/u);
});
