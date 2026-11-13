'use strict';
// Only actual inventory wait/control predicates run; no extension, gateway or process.
const assert = require('node:assert/strict');
const { createHash } = require('node:crypto');
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
    for (const replace of [false, true]) {
      fs.linkSync(ready, ready + '.pending'); let observed = false;
      box.lstatSync = pathname => {
        if (!observed && pathname.endsWith('/ready.pending')) {
          observed = true; const held = fs.openSync(ready + '.pending', fs.constants.O_RDONLY);
          try {
            fs.unlinkSync(ready + '.pending'); const info = fs.fstatSync(held);
            assert.equal(info.nlink, 1);
            if (replace) { fs.renameSync(ready, ready + '.old'); fs.writeFileSync(ready, body); }
            return info;
          } finally { fs.closeSync(held); }
        }
        return fs.lstatSync(pathname);
      };
      if (replace) await assert.rejects(box.wait(req, performance.now() + 1000), /pair differs/);
      else await box.wait(req, performance.now() + 1000);
      assert.equal(observed, true); box.lstatSync = fs.lstatSync;
      if (replace) fs.unlinkSync(ready + '.old');
    }
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

// Test the actual gateway polling implementation; no native process/model is used.
const gatewayStart = source.indexOf('async function callGateway(');
const gatewayEnd = source.indexOf('async function observe(', gatewayStart);
assert(gatewayStart > 0 && gatewayEnd > gatewayStart);
function harness(catalogs, wrongSession = false, behavior = '') {
  let clock = 0, listed = 0;
  const events = [], session = 'owned-session';
  const tool = { name: 'inspect_runtime', description: 'Inspect only disposable fixture runtime facts.',
    inputSchema: { type: 'object', properties: { nonce: { type: 'string' }, operation: { type: 'string' },
      marker: { type: 'string' } }, required: ['nonce'], additionalProperties: false } };
  const facts = { nonce: 'n', revision: 'v1', cwd: '/project', argv: [], plugin_root: '/plugin',
    plugin_data: '/data', literal: 'literal', once: 'once', marker: 'marker', pid: 42, session: 'probe-session' };
  const expected = { label: 'owned', facts: { ...facts } };
  delete expected.facts.pid;
  delete expected.facts.session;
  const sandbox = { Error, Buffer, URL, createHash, observations: [], phase: 'start',
    performance: { now: () => clock }, delay: async ms => { clock += ms; },
    object: value => { if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('object required'); return value; },
    keys: (value, keys) => assert.equal(JSON.stringify(Object.keys(value).sort()), JSON.stringify(keys.slice().sort())),
    same: (a, b) => JSON.stringify(a) === JSON.stringify(b),
    json: value => JSON.parse(Buffer.isBuffer(value) ? value.toString('utf8') : value),
    rpc: (reply, id) => { assert.equal(reply.id, id); assert.equal(reply.status, 200); return reply.result; },
    gatewayHTTP: async (_url, method, selectedSession, payload) => {
      if (method === 'DELETE') {
        events.push(['DELETE']);
        return { status: 204, session: selectedSession, body: Buffer.alloc(0) };
      }
      events.push([payload.method, payload.id]);
      if (payload.method === 'initialize') return { status: 200, session, id: payload.id,
        result: { protocolVersion: '2025-06-18', serverInfo: { name: 'VS Code MCP Gateway', version: '1.0.0' } } };
      if (payload.method === 'notifications/initialized') return { status: 202, session, body: Buffer.alloc(0) };
      if (payload.method === 'tools/list') {
        if (behavior === 'late-catalog') clock = 1000;
        return { status: 200, session: wrongSession ? 'foreign' : session,
          id: payload.id, result: { tools: catalogs[Math.min(listed++, catalogs.length - 1)] } };
      }
      if (payload.method === 'tools/call') {
        if (behavior === 'call-fails') throw new Error('inert call failed');
        return { status: 200, session, id: payload.id,
          result: { isError: false, content: [{ type: 'text', text: JSON.stringify(facts) }] } };
      }
      throw new Error('unreviewed method');
    } };
  vm.createContext(sandbox);
  vm.runInContext(stripTypeScriptTypes(source.slice(gatewayStart, gatewayEnd), { mode: 'strip' }) +
    '\nglobalThis.probe = callGateway;', sandbox);
  const server = { address: { toString: () => 'http://127.0.0.1:4567/gateway/owned' } };
  return { run: () => sandbox.probe(server, expected, { nonce: 'n', operation: 'read', marker: 'marker' }, 1000),
    tool, events, observations: sandbox.observations };
}

test('gateway retries only empty catalog and invokes the owned tool exactly once', async () => {
  const selected = harness([[]]);
  const h = harness([[], [selected.tool]]);
  const result = await h.run();
  assert.equal(result.label, 'owned');
  assert.deepEqual(h.events, [['initialize', 1], ['notifications/initialized', undefined],
    ['tools/list', 2], ['tools/list', 3], ['tools/call', 4], ['DELETE']]);
  assert.equal(h.observations.filter(item => item.phase === 'gateway-tools-list').length, 1);
});

test('unexpected nonempty catalog and mismatched session fail closed before tools/call', async () => {
  const valid = harness([[]]).tool;
  for (const h of [harness([[{ name: 'foreign' }, { name: 'second' }]]),
    harness([[valid]], true)]) {
    await assert.rejects(h.run(), /exact gateway tool catalog differs/);
    assert.equal(h.events.some(entry => entry[0] === 'tools/call'), false);
    assert.equal(h.events.at(-1)[0], 'DELETE');
  }
});

test('persistent empty catalog expires the original deadline without tool invocation', async () => {
  const h = harness([[]]);
  await assert.rejects(h.run(), /exact gateway tool catalog differs/);
  assert(h.events.filter(entry => entry[0] === 'tools/list').length <= 10);
  assert.equal(h.events.some(entry => entry[0] === 'tools/call'), false);
  assert.equal(h.events.at(-1)[0], 'DELETE');
});

test('late catalog cannot invoke tools/call and a failed tool call is never retried', async () => {
  const tool = harness([[]]).tool;
  for (const behavior of ['late-catalog', 'call-fails']) {
    const h = harness([[tool]], false, behavior);
    await assert.rejects(h.run(), behavior === 'late-catalog' ? /exact gateway tool catalog differs/ : /inert call failed/);
    assert.equal(h.events.filter(entry => entry[0] === 'tools/call').length, behavior === 'call-fails' ? 1 : 0);
    assert.equal(h.events.filter(entry => entry[0] === 'DELETE').length, 1);
    assert.deepEqual(h.events.find(entry => entry[0] === 'tools/list'), ['tools/list', 2]);
  }
});
