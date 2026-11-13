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

test('actual trigger reader waits only for the owned two-link transition and rejects aliases/collisions', async () => {
  const fs = require('node:fs'), paths = require('node:path'), os = require('node:os');
  const directory = fs.mkdtempSync(paths.join(os.tmpdir(), 'uap-ready-')), ready = paths.join(directory, 'ready');
  const req = { stage: 'a', nonce: 'nonce' }, body = JSON.stringify(req) + '\n';
  const box = { ...fs, ...paths, Buffer, Error, TextDecoder, performance, process: { env: { UAP_VSCODE_TRUSTED_READY: ready } }, LIMIT: 1 << 20,
    delay: async () => { if (fs.existsSync(ready + '.pending')) fs.unlinkSync(ready + '.pending'); },
    readSync(fd, ...args) { assert.equal(fs.fstatSync(fd).nlink, 1); return fs.readSync(fd, ...args); } };
  vm.createContext(box);
  const helpers = source.slice(source.indexOf('function object('), source.indexOf('function same('));
  vm.runInContext(stripTypeScriptTypes(helpers + source.slice(source.indexOf('async function waitReady('), source.indexOf('type Reply =')), { mode: 'strip' }) + '\nglobalThis.wait = waitReady;', box);
  try {
    fs.writeFileSync(ready + '.pending', body); fs.linkSync(ready + '.pending', ready);
    await box.wait(req, performance.now() + 1000); assert.equal(fs.statSync(ready).nlink, 1);
    fs.linkSync(ready, ready + '.foreign'); await assert.rejects(box.wait(req, performance.now() + 1000), /pair differs/);
    fs.unlinkSync(ready + '.foreign'); fs.linkSync(ready, ready + '.pending');
    let raced = false;
    box.lstatSync = pathname => {
      if (!raced && pathname.endsWith('/ready.pending')) { raced = true; fs.unlinkSync(ready + '.pending'); }
      return fs.lstatSync(pathname);
    };
    await box.wait(req, performance.now() + 1000); assert.equal(raced, true); box.lstatSync = fs.lstatSync;
    fs.linkSync(ready, ready + '.pending');
    box.lstatSync = pathname => {
      if (pathname.endsWith('/ready.pending')) { fs.unlinkSync(ready + '.pending'); fs.unlinkSync(ready); }
      return fs.lstatSync(pathname);
    };
    await assert.rejects(box.wait(req, performance.now() + 100), /identity differs/);
    box.lstatSync = fs.lstatSync; fs.writeFileSync(ready, body);
    fs.writeFileSync(ready + '.pending', 'foreign');
    await assert.rejects(box.wait(req, performance.now() + 1000), /pending collision/);
    fs.unlinkSync(ready + '.pending'); fs.writeFileSync(ready, '{}'); await assert.rejects(box.wait(req, performance.now() + 1000));
    fs.writeFileSync(ready, body); fs.linkSync(ready, ready + '.pending'); fs.linkSync(ready, ready + '.third');
    await assert.rejects(box.wait(req, performance.now() + 1000), /identity differs/);
    fs.unlinkSync(ready + '.third'); fs.unlinkSync(ready + '.pending');
    const originalRead = box.readSync;
    let swapped = false;
    box.readSync = (fd, ...args) => { if (!swapped) { swapped = true; fs.renameSync(directory, directory + '.old'); fs.mkdirSync(directory); fs.writeFileSync(ready, 'foreign'); } return originalRead(fd, ...args); };
    await assert.rejects(box.wait(req, performance.now() + 1000), /parent changed/);
    assert.equal(fs.readFileSync(ready, 'utf8'), 'foreign'); fs.rmSync(directory + '.old', { recursive: true });
  } finally { fs.rmSync(directory, { recursive: true, force: true }); }
});
