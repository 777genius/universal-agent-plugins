import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { basename, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const npmCLI = process.env.npm_execpath;
if (!npmCLI) throw new Error('Run through npm run test:consumer so the exact npm CLI is available.');
const workspace = await mkdtemp(join(tmpdir(), 'uap-sdk-consumer-'));
const npm = (args: string[], cwd: string) => execFileSync(process.execPath, [npmCLI, ...args], { cwd, env: process.env, encoding: 'utf8', maxBuffer: 4 * 1024 * 1024 });
try {
  const packed: Array<{ filename: string; files: Array<{ path: string }> }> = JSON.parse(npm(['pack', '--ignore-scripts', '--json', '--pack-destination', workspace], root));
  assert.equal(packed.length, 1);
  const packageInfo = packed[0]!;
  assert.equal(packageInfo.filename, basename(packageInfo.filename));
  for (const file of packageInfo.files) assert.ok(!file.path.includes('node_modules') && !file.path.startsWith('src/'), `Unexpected packed file: ${file.path}`);
  assert.ok(packageInfo.files.some((file) => file.path === 'dist/generated/schema.d.ts'));
  await writeFile(join(workspace, 'package.json'), JSON.stringify({ private: true, type: 'module' }));
  npm(['install', '--ignore-scripts', '--no-audit', '--no-fund', join(workspace, packageInfo.filename)], workspace);
  await writeFile(join(workspace, 'consumer.ts'), `
import { createClient, type PrepareRequest, type Operation } from '@uap/management-sdk';
import { startUap, type StartupConfiguration } from '@uap/management-sdk/node';
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
  await writeFile(join(workspace, 'consumer.js'), `import { createClient } from '@uap/management-sdk'; import { startUap } from '@uap/management-sdk/node'; if (typeof createClient !== 'function' || typeof startUap !== 'function') throw new Error('Missing SDK exports');`);
  execFileSync(process.execPath, [join(workspace, 'consumer.js')], { cwd: workspace, stdio: 'inherit' });
  const publicDeclaration = await readFile(join(root, 'dist/index.d.ts'), 'utf8');
  assert.ok(!publicDeclaration.includes('node:'));
  console.log('Clean packed SDK consumer: typecheck and runtime imports passed.');
} finally {
  await rm(workspace, { recursive: true, force: true });
}
