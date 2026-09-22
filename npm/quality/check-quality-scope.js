const assert = require('node:assert/strict');
const { execFileSync } = require('node:child_process');
const { readFileSync } = require('node:fs');
const path = require('node:path');

const repositoryRoot = path.resolve(__dirname, '../..');
const packageRoot = path.join(repositoryRoot, 'npm/agentplugins');
const SOURCE_PATTERN = /\.(?:[cm]?[jt]sx?|vue)$/u;
const PROTECTED_RULES = ['no-eval', 'no-implied-eval', 'no-new-func'];
const RELEASE_SURFACE_FILES = new Set([
  'examples/local/codex-node-local/plugin/main.mjs',
  'examples/local/codex-node-local/plugin/plugin-runtime.mjs',
  'examples/local/codex-node-typescript-local/plugin/main.ts',
  'examples/local/codex-node-typescript-local/plugin/plugin-runtime.ts',
  'examples/plugins/opencode-basic/.opencode/plugins/custom-tool.js',
  'examples/plugins/opencode-basic/.opencode/plugins/example.js',
  'examples/plugins/opencode-basic/.opencode/tools/echo.ts',
  'examples/plugins/opencode-basic/plugin/targets/opencode/plugins/custom-tool.js',
  'examples/plugins/opencode-basic/plugin/targets/opencode/plugins/example.js',
  'examples/plugins/opencode-basic/plugin/targets/opencode/tools/echo.ts',
  'examples/starters/claude-node-typescript-runtime-package-starter/plugin/main.ts',
  'examples/starters/claude-node-typescript-starter/plugin/main.ts',
  'examples/starters/claude-node-typescript-starter/plugin/plugin-runtime.ts',
  'examples/starters/codex-node-typescript-starter/plugin/main.ts',
  'examples/starters/codex-node-typescript-starter/plugin/plugin-runtime.ts',
]);
const TOOLING_SURFACE_FILES = new Set([
  'npm/quality/check-quality-scope.js',
  'scripts/terminal-ui/stage_npm.js',
]);
const LINT_ROOTS = [
  'npm/agentplugins/bin', 'npm/agentplugins/lib', 'npm/agentplugins/scripts',
  'npm/plugin-kit-ai/bin', 'npm/plugin-kit-ai/lib', 'npm/plugin-kit-ai-runtime/index.js',
];
const ALL_LINT_TARGETS = [...LINT_ROOTS, ...RELEASE_SURFACE_FILES, ...TOOLING_SURFACE_FILES];
const CORRECTNESS_TARGETS = [
  'npm/agentplugins/bin', 'npm/agentplugins/lib', 'npm/plugin-kit-ai/bin',
  'npm/plugin-kit-ai/lib', 'npm/plugin-kit-ai-runtime/index.js', ...RELEASE_SURFACE_FILES,
];
const QUALITY_INSTALL = 'cd npm/quality && npm ci --ignore-scripts --no-audit --no-fund';
const SCOPE_COMMAND = 'node npm/quality/check-quality-scope.js';
const SCOPE_TEST_COMMAND = 'node --test npm/quality/check-quality-scope.test.js';
const OXLINT_PREFIX = 'npm --prefix npm/quality exec -- oxlint --config npm/agentplugins/oxlint.json --deny-warnings --disable-nested-config';
const BROAD_LINT = `${OXLINT_PREFIX} ${ALL_LINT_TARGETS.join(' ')}`;
const CORRECTNESS_LINT = `${OXLINT_PREFIX} -D correctness ${CORRECTNESS_TARGETS.join(' ')}`;
const DECLARATION_COMMAND = 'npm --prefix npm/quality exec -- tsc -p npm/plugin-kit-ai-runtime/tsconfig.docs.json --noEmit';

function classifySourcePath(relativePath) {
  const normalized = relativePath.replaceAll('\\', '/');
  if (!SOURCE_PATTERN.test(normalized)) return 'non-source';
  if (normalized.startsWith('npm/agentplugins/test/') || normalized.endsWith('.test.js')) return 'test';
  if (normalized.startsWith('npm/agentplugins/scripts/') || TOOLING_SURFACE_FILES.has(normalized)) return 'tooling';
  if (RELEASE_SURFACE_FILES.has(normalized)) return 'release-surface';
  if (normalized.startsWith('npm/agentplugins/bin/') || normalized.startsWith('npm/agentplugins/lib/')
      || normalized.startsWith('npm/plugin-kit-ai/bin/') || normalized.startsWith('npm/plugin-kit-ai/lib/')
      || normalized === 'npm/plugin-kit-ai-runtime/index.js') return 'production';
  if (normalized === 'npm/plugin-kit-ai-runtime/index.d.ts') return 'declaration';
  return null;
}

function recipeFor(makefile, target) {
  const lines = makefile.split(/\r?\n/u);
  let conditionalDepth = 0;
  const starts = [];
  for (const [index, line] of lines.entries()) {
    if (/^ *(?:ifeq|ifneq|ifdef|ifndef)\b/u.test(line)) conditionalDepth += 1;
    if (!line.startsWith('\t')) {
      const colon = line.indexOf(':');
      const targets = colon === -1 ? [] : line.slice(0, colon).trim().split(/[ \t]+/u);
      if (targets.includes(target)) starts.push({ conditionalDepth, index, canonical: line === `${target}:` });
    }
    if (/^ *endif\b/u.test(line)) conditionalDepth -= 1;
    if (conditionalDepth < 0) throw new Error('Makefile contains an unmatched conditional');
  }
  if (conditionalDepth !== 0) throw new Error('Makefile contains an unmatched conditional');
  if (starts.length !== 1 || starts[0].conditionalDepth !== 0 || !starts[0].canonical) return [];
  const start = starts[0].index;
  const recipe = [];
  for (const line of lines.slice(start + 1)) {
    if (line.startsWith('\t')) recipe.push(line.slice(1));
    else if (line.trim() !== '') break;
  }
  return recipe;
}

function assertRequiredRoute(manifest, qualityManifest, makefile, declarationConfig) {
  if ('devDependencies' in manifest) throw new Error('published package manifest cannot own development tooling');
  if (qualityManifest.private !== true || qualityManifest.devDependencies?.oxlint !== '1.85.0' || qualityManifest.devDependencies?.typescript !== '7.0.2') {
    throw new Error('npm quality dependencies must stay in the private owner on reviewed exact versions');
  }
  if (manifest.scripts?.test !== 'node --test') throw new Error('the published package test contract must stay unchanged');
  if (declarationConfig.compilerOptions?.skipLibCheck !== false || declarationConfig.include?.length !== 1 || declarationConfig.include[0] !== 'index.d.ts') {
    throw new Error('declaration gate must semantically check index.d.ts with skipLibCheck disabled');
  }

  const expected = [QUALITY_INSTALL, SCOPE_COMMAND, SCOPE_TEST_COMMAND, BROAD_LINT, CORRECTNESS_LINT, DECLARATION_COMMAND, 'cd npm/agentplugins && npm test && npm pack --dry-run --ignore-scripts'];
  const recipe = recipeFor(makefile, 'test-agentplugins-js');
  const delegation = recipeFor(makefile, 'test-required');
  if (recipe.length !== expected.length || expected.some((command, index) => recipe[index] !== command)
      || !delegation.includes('$(MAKE) test-agentplugins-js')
      || recipe.some((command) => command.startsWith('-') || /(?:\|\|\s*true\b|;\s*true\b)/u.test(command))) {
    throw new Error('Makefile must retain the uncommented, complete, fail-closed npm quality route');
  }
}

function assertLintPolicy(config) {
  assert.deepEqual(Object.keys(config).sort(), ['$schema', 'categories', 'rules'], 'Oxlint config cannot add overrides or ignore surfaces');
  if (config.categories?.correctness !== 'off' || config.categories?.suspicious !== 'off') throw new Error('broad Oxlint scan must stay limited to protected rules before the production correctness pass');
  for (const rule of PROTECTED_RULES) if (config.rules?.[rule] !== 'error') throw new Error(`protected rule disabled: ${rule}`);
}

function trackedSourcePaths() {
  const roots = ['npm/agentplugins', 'npm/plugin-kit-ai', 'npm/plugin-kit-ai-runtime', 'npm/quality', ...RELEASE_SURFACE_FILES, ...TOOLING_SURFACE_FILES];
  const output = execFileSync('git', ['ls-files', '-z', '--', ...roots], { cwd: repositoryRoot, encoding: 'utf8' });
  return output.split('\0').filter(Boolean).filter((item) => SOURCE_PATTERN.test(item));
}

function verify() {
  const manifest = JSON.parse(readFileSync(path.join(packageRoot, 'package.json'), 'utf8'));
  const config = JSON.parse(readFileSync(path.join(packageRoot, 'oxlint.json'), 'utf8'));
  const qualityManifest = JSON.parse(readFileSync(path.join(repositoryRoot, 'npm/quality/package.json'), 'utf8'));
  const declarationConfig = JSON.parse(readFileSync(path.join(repositoryRoot, 'npm/plugin-kit-ai-runtime/tsconfig.docs.json'), 'utf8'));
  const makefile = readFileSync(path.join(repositoryRoot, 'Makefile'), 'utf8');
  assertRequiredRoute(manifest, qualityManifest, makefile, declarationConfig);
  assertLintPolicy(config);
  const sources = trackedSourcePaths();
  const unclassified = sources.filter((item) => classifySourcePath(item) === null);
  if (unclassified.length > 0) throw new Error(`unclassified npm-owned source:\n${unclassified.join('\n')}`);
  const counts = Object.create(null);
  for (const item of sources) counts[classifySourcePath(item)] = (counts[classifySourcePath(item)] ?? 0) + 1;
  process.stdout.write(`npm quality scope verified: ${JSON.stringify(counts)}\n`);
}

if (require.main === module) verify();
module.exports = { assertLintPolicy, assertRequiredRoute, classifySourcePath };
