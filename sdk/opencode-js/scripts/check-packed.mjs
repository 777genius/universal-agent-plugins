// Real installed tarball consumers. Run with npm; no host/model is launched.
import { spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, copyFileSync, existsSync, rmSync, readdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
const root = resolve(import.meta.dirname, '..');
const npm = process.env.npm_execpath;
if (!npm) throw new Error('Run via npm run check:packed (npm CLI required)');
const temp = mkdtempSync(join(tmpdir(), 'TEST-observer-packed-'));
function run(args, cwd) {
  const r = spawnSync(process.execPath, [npm, ...args], { cwd, encoding: 'utf8', env: process.env });
  if (r.status !== 0) throw new Error(`npm ${args[0]} failed: ${r.stderr || r.stdout}`);
  return r.stdout;
}
try {
  run(['pack', '--ignore-scripts', '--pack-destination', temp], root);
  const archive = readdirSync(temp).find((name) => name.endsWith('.tgz'));
  if (!archive) throw new Error('npm pack produced no tarball');
  for (const version of ['1.18.33', '1.18.34', 'v2-only']) {
    const cwd = join(temp, version); mkdirSync(cwd);
    writeFileSync(join(cwd, 'package.json'), JSON.stringify({ private: true, type: 'module' }));
    run(['install', '--ignore-scripts', '--no-audit', '--no-fund', join(temp, archive), 'typescript@7.0.2',
      ...(version === 'v2-only' ? [] : [`@opencode-ai/sdk@${version}`])], cwd);
    if (version === 'v2-only' && existsSync(join(cwd, 'node_modules/@opencode-ai/sdk'))) throw new Error('V2 consumer pulled V1 SDK');
    const source = version === 'v2-only'
      ? `import { createV2Observer, type V2ObserverOptions, type PreparationHandoff } from 'universal-agent-plugins-opencode-events/v2';
         declare const options: V2ObserverOptions; createV2Observer({ ...options, beforeEmit(event, handoff: PreparationHandoff) {
           void event; void handoff.metadataDeadline; void handoff.ingressMonotonicMs; return handoff.isCurrent(); } }).dispose();`
      : `import { createObserver, createV2Observer, type V2Client, type V2Location, type V2NativeEvent, type V2ObservedEvent, type V2ObserverOptions, type OpenCodeNativeEvent } from 'universal-agent-plugins-opencode-events';
         import { createObserver as v1, type ObserverOptions, type PreparationHandoff } from 'universal-agent-plugins-opencode-events/v1';
         declare const options: ObserverOptions; declare const native: OpenCodeNativeEvent;
         createObserver({ ...options, beforeEmit(event, handoff: PreparationHandoff) {
           void event; void handoff.clockID; return handoff.isCurrent(); } }).observe(native); v1(options).dispose();
         declare const v2Options: V2ObserverOptions; declare const v2Event: V2NativeEvent;
         declare const client: V2Client; declare const location: V2Location; declare const fact: V2ObservedEvent;
         const v2 = createV2Observer({ ...v2Options, client, location, emit: (e: V2ObservedEvent) => { void e; void fact; } });
         v2.observe(v2Event); v2.dispose();`;
    writeFileSync(join(cwd, 'consumer.ts'), source);
    const compile = spawnSync(process.execPath, [join(cwd, 'node_modules/typescript/bin/tsc'), '--strict', '--noEmit',
      '--module', 'NodeNext', '--target', 'ES2024', 'consumer.ts'], { cwd, encoding: 'utf8' });
    if (compile.status !== 0) throw new Error(`${version} installed typecheck failed: ${compile.stdout}${compile.stderr}`);
    const path = version === 'v2-only' ? '/v2' : '';
    const symbol = version === 'v2-only' ? 'createV2Observer' : 'createObserver';
    const runtime = spawnSync(process.execPath, ['--input-type=module', '-e',
      `import { ${symbol} } from 'universal-agent-plugins-opencode-events${path}'; if (typeof ${symbol} !== 'function') process.exit(1);`],
      { cwd, encoding: 'utf8' });
    if (runtime.status !== 0) throw new Error(`${version} installed import failed: ${runtime.stderr}`);
    copyFileSync(join(root, 'scripts/packed-runtime.mjs'), join(cwd, 'consumer.mjs'));
    const behavior = spawnSync(process.execPath, ['consumer.mjs'], { cwd, encoding: 'utf8' });
    if (behavior.status !== 0) throw new Error(`${version} installed behavior failed: ${behavior.stdout}${behavior.stderr}`);
    console.log(`${version}: installed import, public types and behavior passed`);
  }
} finally { rmSync(temp, { recursive: true, force: true }); }
