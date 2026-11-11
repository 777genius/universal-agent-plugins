import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, writeFile, appendFile, readFile, mkdir, rm } from 'node:fs/promises';
import { spawnSync } from 'node:child_process';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { liveEvidenceRows } from '../scripts/native-jsonl.ts';

test('live native evidence waits for a complete LF record across partial UTF-8 writes', async (t) => {
 const root = await mkdtemp(join(tmpdir(), 'TEST-opencode-jsonl-'));
 t.after(() => rm(root, { recursive: true, force: true }));
 const evidence = join(root, 'evidence.jsonl');
 const ready = { phase: 'server-ready', pid: 123 };
 const native = { native: { type: 'permission.asked', properties: { title: 'пример', sessionID: 'session' } } };
 const first = JSON.stringify(ready) + '\n';
 const next = Buffer.from(JSON.stringify(native));
 const split = next.indexOf(Buffer.from('п')) + 1;
 assert(split > 0 && split < next.length);
 await writeFile(evidence, Buffer.concat([Buffer.from(first), next.subarray(0, split)]));
 assert.deepEqual(liveEvidenceRows(await readFile(evidence, 'utf8')), [ready]);
 await appendFile(evidence, next.subarray(split));
 assert.deepEqual(liveEvidenceRows(await readFile(evidence, 'utf8')), [ready], 'valid EOF JSON is still uncommitted');
 await appendFile(evidence, '\n');
 assert.deepEqual(liveEvidenceRows(await readFile(evidence, 'utf8')), [ready, native]);
 await appendFile(evidence, '{"fact":');
 assert.deepEqual(liveEvidenceRows(await readFile(evidence, 'utf8')), [ready, native]);
});

test('live evidence without a committed record supplies no observations', () => {
 for (const snapshot of ['', '{"phase":', '{"phase":"plugin-loaded"}']) {
  assert.deepEqual(liveEvidenceRows(snapshot), []);
 }
 assert.deepEqual(liveEvidenceRows('\n{"phase":"plugin-loaded"}\r\n\n'), [{ phase: 'plugin-loaded' }]);
});

test('malformed terminated native evidence is never hidden by a later row or live tail', () => {
 const valid = '{"fact":{"kind":"turn_idle_verified"}}\n';
 for (const snapshot of [
  '{"broken":\n' + valid,
  valid + '{"broken":\n',
  valid + '{"broken":\n{"partial":',
 ]) {
  assert.throws(() => liveEvidenceRows(snapshot), SyntaxError);
 }
});

test('the actual closed native receipt rejects truncated EOF evidence', async (t) => {
 const root = await mkdtemp(join(tmpdir(), 'TEST-opencode-closed-jsonl-'));
 t.after(() => rm(root, { recursive: true, force: true }));
 const run = join(root, 'run-v1');
 await mkdir(run);
 await writeFile(join(run, 'evidence.jsonl'), '{"phase":"server-ready"}\n{"fact":');
 await writeFile(join(run, 'result.json'), '{"status":"passed"}');
 const pack = join(root, 'pack.json');
 await writeFile(pack, '[]');
 const receipt = fileURLToPath(new URL('../scripts/native-receipt.ts', import.meta.url));
 const result = spawnSync(process.execPath, [receipt, root, 'a'.repeat(40), run, run, pack], {
  encoding: 'utf8',
  timeout: 10000,
 });
 assert.ifError(result.error);
 assert.equal(result.signal, null);
 assert.equal(result.status, 1);
 assert.match(result.stderr, /SyntaxError/);
 assert.match(result.stderr, /at JSON\.parse/);
 assert.match(result.stderr, /native-receipt\.ts/);
 await assert.rejects(readFile(join(root, 'evidence/native-qualification.json')), { code: 'ENOENT' });
});
