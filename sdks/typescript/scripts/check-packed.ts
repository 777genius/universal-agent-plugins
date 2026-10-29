// Installs the SDK in a clean consumer exactly as a user would and checks the
// public package contract: shipped files, browser-safe types and both entry
// points. Run through npm so the exact npm CLI is available:
//   npm run test:consumer                                   pack this checkout
//   npm run test:consumer -- --tarball=/abs/name-X.Y.Z.tgz  an already packed tarball
//   npm run test:consumer -- --registry-version=X.Y.Z       the public npm release
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtemp, readdir, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { basename, isAbsolute, join, relative, sep } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const npmCLI = process.env.npm_execpath;
if (!npmCLI) throw new Error('Run through npm run test:consumer so the exact npm CLI is available.');
const manifest: { name: string; version: string } = JSON.parse(await readFile(join(root, 'package.json'), 'utf8'));
const { name } = manifest;

const options = process.argv.slice(2);
if (options.length > 1) throw new Error('Pass at most one of --tarball=<absolute path> or --registry-version=<package version>.');
const option = options[0];
let source: { kind: 'checkout' } | { kind: 'tarball'; path: string } | { kind: 'registry'; version: string } = { kind: 'checkout' };
if (option?.startsWith('--tarball=')) {
  const path = option.slice('--tarball='.length);
  if (!isAbsolute(path) || basename(path) !== `${name}-${manifest.version}.tgz`) throw new Error(`--tarball must be an absolute path to ${name}-${manifest.version}.tgz`);
  source = { kind: 'tarball', path };
} else if (option !== undefined) {
  // Only the version of this checkout may be checked against the registry.
  if (option !== `--registry-version=${manifest.version}`) throw new Error(`Only --registry-version=${manifest.version} is accepted for this checkout.`);
  source = { kind: 'registry', version: manifest.version };
}

// Exactly what a consumer receives: npm metadata, documentation and compiled output.
const required = ['package.json', 'README.md', 'LICENSE', 'dist/index.js', 'dist/index.d.ts', 'dist/node.js', 'dist/node.d.ts', 'dist/generated/schema.d.ts'];
const allowed = (path: string) => ['package.json', 'README.md', 'LICENSE'].includes(path) || /^dist\/[A-Za-z0-9_/-]+\.(?:js|d\.ts)$/.test(path);

const workspace = await mkdtemp(join(tmpdir(), 'uap-sdk-consumer-'));
const npm = (args: string[], cwd: string) => execFileSync(process.execPath, [npmCLI, ...args], { cwd, env: process.env, encoding: 'utf8', maxBuffer: 4 * 1024 * 1024 });
const install = ['install', '--ignore-scripts', '--no-audit', '--no-fund'];

async function listFiles(directory: string): Promise<string[]> {
  const files: string[] = [];
  for (const entry of await readdir(directory, { withFileTypes: true, recursive: true })) {
    const path = relative(directory, join(entry.parentPath, entry.name)).split(sep).join('/');
    // Dependencies npm may nest under the package are not part of its tarball.
    if (path === 'node_modules' || path.startsWith('node_modules/')) continue;
    assert.ok(entry.isFile() || entry.isDirectory(), `Unexpected packed entry type: ${path}`);
    if (entry.isFile()) files.push(path);
  }
  return files.sort();
}

try {
  await writeFile(join(workspace, 'package.json'), JSON.stringify({ private: true, type: 'module' }));
  if (source.kind === 'registry') {
    // npm can acknowledge publication before every registry replica serves the
    // version. Bound propagation at about five and a half minutes.
    for (let attempt = 1; ; attempt += 1) {
      try {
        npm([...install, '--save-exact', `${name}@${source.version}`], workspace);
        break;
      } catch (error) {
        if (attempt === 12) throw error;
        await delay(attempt * 5_000);
      }
    }
  } else {
    let tarball: string;
    if (source.kind === 'tarball') tarball = source.path;
    else {
      // npm 10/11 print an array of records; npm 12 prints an object keyed by package name.
      const packed: unknown = JSON.parse(npm(['pack', '--ignore-scripts', '--json', '--pack-destination', workspace], root));
      const records = Array.isArray(packed) ? packed : Object.entries(packed as Record<string, unknown>).filter(([key]) => key === name).map(([, record]) => record);
      assert.ok(records.length === 1 && (Array.isArray(packed) || Object.keys(packed as object).length === 1), 'npm pack must report exactly one package');
      const { filename } = records[0] as { filename: string };
      assert.equal(filename, `${name}-${manifest.version}.tgz`);
      tarball = join(workspace, filename);
    }
    npm([...install, tarball], workspace);
  }

  const installed = join(workspace, 'node_modules', ...name.split('/'));
  const installedManifest: { name: string; version: string } = JSON.parse(await readFile(join(installed, 'package.json'), 'utf8'));
  assert.equal(installedManifest.name, name);
  assert.equal(installedManifest.version, manifest.version);
  const files = await listFiles(installed);
  for (const path of files) assert.ok(allowed(path), `Unexpected packed file: ${path}`);
  for (const path of required) assert.ok(files.includes(path), `Missing packed file: ${path}`);
  assert.ok(!(await readFile(join(installed, 'dist/index.d.ts'), 'utf8')).includes('node:'), 'The pure entry point must not expose Node types.');

  await writeFile(join(workspace, 'consumer.ts'), `
import { createClient, type PrepareRequest, type Operation } from '${name}';
import { startUap, type StartupConfiguration } from '${name}/node';
const request: PrepareRequest = { action: 'install', source: 'example/plugin', targetIds: ['claude'] };
const client = createClient({ baseUrl: 'http://127.0.0.1:12345', token: 'not-a-real-token' });
const config: StartupConfiguration = { stateRoot: '/state', dependenciesRoot: '/deps', scope: 'user', targets: [{ id: 'claude', client: 'claude', configRoot: '/profile' }] };
const observe = (operation: Operation) => operation.contextId;
void request; void client; void config; void observe; void startUap;
// @ts-expect-error Unknown actions must fail at the package boundary.
const invalid: PrepareRequest = { action: 'arbitrary-command' };
void invalid;
`);
  await writeFile(join(workspace, 'tsconfig.json'), JSON.stringify({ compilerOptions: { target: 'ES2022', module: 'NodeNext', moduleResolution: 'NodeNext', strict: true, noEmit: true, types: [], lib: ['ES2022', 'DOM'], skipLibCheck: false }, files: ['consumer.ts'] }));
  execFileSync(process.execPath, [join(root, 'node_modules/typescript/bin/tsc'), '-p', join(workspace, 'tsconfig.json')], { cwd: workspace, stdio: 'inherit' });
  await writeFile(join(workspace, 'consumer.js'), `import { createClient } from '${name}'; import { startUap } from '${name}/node'; if (typeof createClient !== 'function' || typeof startUap !== 'function') throw new Error('Missing SDK exports');`);
  execFileSync(process.execPath, [join(workspace, 'consumer.js')], { cwd: workspace, stdio: 'inherit' });
  console.log(`Clean ${source.kind} SDK consumer (${name}@${manifest.version}): files, typecheck and runtime imports passed.`);
} finally {
  await rm(workspace, { recursive: true, force: true });
}
