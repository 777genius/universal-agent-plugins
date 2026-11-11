import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, writeFile, appendFile, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { parseNativeRows } from '../scripts/native-jsonl.ts';

test('live native evidence waits for a partially written row to finish', async (t) => {
 const root = await mkdtemp(join(tmpdir(), 'TEST-native-jsonl-'));
 t.after(() => rm(root, { recursive: true, force: true }));
 const evidence = join(root, 'evidence.jsonl');
 const first = { phase: 'server-ready' };
 const second = { native: { id: 'evt_123' } };
 await writeFile(evidence, JSON.stringify(first) + '\n' + '{"native":{"id":"evt_');
 const partial = await readFile(evidence, 'utf8');
 assert.deepEqual(parseNativeRows(partial), [first]);
 assert.throws(() => parseNativeRows(partial, true), /unterminated JSONL record/);
 await appendFile(evidence, '123"}}\n');
 const completed = await readFile(evidence, 'utf8');
 assert.deepEqual(parseNativeRows(completed), [first, second]);
 assert.deepEqual(parseNativeRows(completed, true), [first, second]);
});

test('even valid JSON is not a live record before its terminating LF', () => {
 const row = '{"phase":"completion-pass"}';
 assert.deepEqual(parseNativeRows(row), []);
 assert.deepEqual(parseNativeRows(row + '\n'), [{ phase: 'completion-pass' }]);
 assert.throws(() => parseNativeRows(row, true), /unterminated JSONL record/);
});

test('malformed complete native records fail both live and closed admission', () => {
 for (const text of ['{"native":]}\n', '{"phase":"server-ready"}\nnot-json\n']) {
  assert.throws(() => parseNativeRows(text), SyntaxError);
  assert.throws(() => parseNativeRows(text, true), SyntaxError);
 }
 assert.throws(() => parseNativeRows('not-json\n{"unfinished":'), SyntaxError);
 assert.throws(() => parseNativeRows('{"native":{}}{"phase":"owned-host-closed"}\n', true), SyntaxError);
});

test('closed native evidence rejects every unterminated tail', () => {
 const complete = '{"phase":"owned-host-closed"}\n';
 for (const tail of ['{"native":', '{"native":{}}', ' ', '\r']) {
  assert.deepEqual(parseNativeRows(complete + tail), [{ phase: 'owned-host-closed' }]);
  assert.throws(() => parseNativeRows(complete + tail, true), /unterminated JSONL record/);
 }
});

test('complete LF and CRLF rows retain their order', () => {
 const expected = [{ phase: 'server-ready' }, { native: { type: 'question.asked' } }];
 for (const separator of ['\n', '\r\n']) {
  const text = expected.map((row) => JSON.stringify(row)).join(separator) + separator;
  assert.deepEqual(parseNativeRows(text), expected);
  assert.deepEqual(parseNativeRows(text, true), expected);
 }
});

test('closed admission retains rejection of internal blank records and empty evidence', () => {
 for (const separator of ['\n', '\r\n']) {
  const text = '{"phase":"server-ready"}' + separator + separator + '{"phase":"owned-host-closed"}' + separator;
  assert.throws(() => parseNativeRows(text, true), SyntaxError);
 }
 for (const text of ['', '\n', ' \r\n\t\n']) {
  assert.deepEqual(parseNativeRows(text), []);
  assert.throws(() => parseNativeRows(text, true), SyntaxError);
 }
 // Preserve the old receipt's trim around actual rows, without skipping internal blanks.
 assert.deepEqual(parseNativeRows('\n{"phase":"server-ready"}\n\n', true), [{ phase: 'server-ready' }]);
});
