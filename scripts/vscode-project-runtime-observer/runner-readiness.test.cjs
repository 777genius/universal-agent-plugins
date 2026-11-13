'use strict';
// Only actual inventory wait/control predicates run; no extension, gateway or process.
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { stripTypeScriptTypes } = require('node:module');
const { join } = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const source = readFileSync(join(__dirname, 'runner.ts'), 'utf8');
const begin = source.indexOf('// BEGIN NATIVE INVENTORY READINESS:');
const end = source.indexOf('// END NATIVE INVENTORY READINESS');
assert(begin > 0 && end > begin && source.indexOf('// BEGIN NATIVE INVENTORY READINESS:', begin + 1) === -1);
class Http { constructor(label, uri, headers = {}) { Object.assign(this, { label, uri: { toString: () => uri }, headers }); } }
const sandbox = { Error, performance, setTimeout, clearTimeout, vscode: { McpHttpServerDefinition: Http } };
vm.createContext(sandbox);
vm.runInContext(stripTypeScriptTypes(source.slice(begin, end), { mode: 'strip' }) + '\nglobalThis.wait = waitInventory; globalThis.definition = definitionControlReady; globalThis.gateway = gatewayControlReady; globalThis.rejectAbsent = rejectAbsentServers;', sandbox);
const control = { label: 'uap-foreign-a', uri: 'http://127.0.0.1:9/preserved-foreign/0123456789abcdef0123456789abcdef/a' };
const http = () => new Http(control.label, control.uri);
function channel(initial = []) {
  let rows = initial, listener, disposed = 0;
  return { read: () => rows, subscribe: callback => { listener = callback; return { dispose() { disposed++; listener = undefined; } }; },
    emit(value) { rows = value; listener?.(); }, get disposed() { return disposed; } };
}
const wait = (channel, ready = rows => sandbox.definition(rows, control), ms = 1000) => sandbox.wait(channel.read, channel.subscribe, ready, performance.now() + ms);

test('initial empty and first empty event cannot prove absence; positive current control returns one immutable snapshot', async () => {
  const ch = channel(), pending = wait(ch);
  let settled = false; pending.then(() => { settled = true; });
  ch.emit([]); await Promise.resolve(); assert.equal(settled, false);
  const inventory = [http()]; ch.emit(inventory);
  const snapshot = await pending; inventory.push({ label: 'removed-owned-server' });
  assert.equal(snapshot.length, 1); assert.equal(snapshot.some(row => row.label === 'removed-owned-server'), false);
  assert.equal(ch.disposed, 1);
});

test('subscribe precedes read, and current control preserves unexpected owned server for absence rejection', async () => {
  const ch = channel([http(), { label: 'removed-owned-server' }]);
  const snapshot = await wait(ch);
  assert.throws(() => sandbox.rejectAbsent(snapshot, ['removed-owned-server']), /removed server visible/);
  assert.equal(snapshot.some(row => row.label === 'installed-owned-server'), false);
  assert.equal(ch.disposed, 1);
  let subscribed = false;
  await sandbox.wait(() => { assert(subscribed); return [http()]; }, () => { subscribed = true; return { dispose() {} }; }, rows => sandbox.definition(rows, control), performance.now() + 1000);
});

test('wrong URI, wrong type, headers and duplicated control fail closed and dispose', async () => {
  for (const rows of [[new Http(control.label, control.uri + '-stale')], [{ label: control.label }],
    [new Http(control.label, control.uri, { Authorization: 'private' })], [http(), http()]]) {
    const ch = channel(rows); await assert.rejects(wait(ch), /control differs/); assert.equal(ch.disposed, 1);
  }
});

test('missing control and expired original deadline refuse; count/read failures dispose', async () => {
  const ch = channel(); await assert.rejects(wait(ch, undefined, 10), /deadline/); assert.equal(ch.disposed, 1);
  let subscribed = false;
  await assert.rejects(sandbox.wait(() => [], () => { subscribed = true; }, () => true, performance.now() - 1), /deadline/);
  assert.equal(subscribed, false);
  const huge = channel(Array.from({ length: 257 }, () => http()));
  await assert.rejects(wait(huge), /count bound/); assert.equal(huge.disposed, 1);
  let disposed = false;
  await assert.rejects(sandbox.wait(() => { throw new Error('read failed'); }, () => ({ dispose() { disposed = true; } }), () => true, performance.now() + 1000), /read failed/);
  assert(disposed);
});

test('gateway lags definitions independently; label barrier never reads endpoint and rejects ambiguity', async () => {
  await wait(channel([http()]));
  const ch = channel(), pending = wait(ch, rows => sandbox.gateway(rows, control));
  let settled = false; pending.then(() => { settled = true; });
  ch.emit([]); await Promise.resolve(); assert.equal(settled, false);
  ch.emit([{ label: control.label, get address() { throw new Error('endpoint must not be read'); } }]);
  assert.equal((await pending).length, 1); assert.equal(ch.disposed, 1);
  const duplicate = channel([{ label: control.label }, { label: control.label }]);
  await assert.rejects(wait(duplicate, rows => sandbox.gateway(rows, control)), /ambiguous/); assert.equal(duplicate.disposed, 1);
  await assert.rejects(wait(channel(), rows => sandbox.gateway(rows, control), 10), /deadline/);
  assert(source.includes('gatewayServers.filter(row => row.label === expected.label)'));
  assert(!source.includes('callGateway(req.readinessControl'));
});


test('disposal failure rejects without masking read failure, including synchronous registration event', async () => {
  const subscribe = changed => { changed(); return { dispose() { throw new Error('dispose failed'); } }; };
  await assert.rejects(sandbox.wait(() => [http()], subscribe, rows => sandbox.definition(rows, control), performance.now() + 1000), /dispose failed/);
  await assert.rejects(sandbox.wait(() => { throw new Error('original read failed'); }, subscribe, () => true, performance.now() + 1000), /original read failed/);
});
