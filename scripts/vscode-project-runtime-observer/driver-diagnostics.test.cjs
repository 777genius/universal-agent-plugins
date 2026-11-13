'use strict';
// Execute only the actual pure diagnostic functions, never driver main or Playwright.
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { stripTypeScriptTypes } = require('node:module');
const { join } = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const source = readFileSync(join(__dirname, 'driver.ts'), 'utf8');
const start = '// BEGIN CLOSED FAILURE DIAGNOSTICS:';
const end = '// END CLOSED FAILURE DIAGNOSTICS';
const begin = source.indexOf(start), finish = source.indexOf(end);
assert(begin > 0 && finish > begin && source.indexOf(start, begin + 1) === -1);
const pure = source.slice(begin, finish);
assert(!/require\(|spawn\(|process\.|chromium\.|writeFile/u.test(pure));
const sandbox = { Buffer, Error, LIMIT: 1 << 20 };
vm.createContext(sandbox);
vm.runInContext(stripTypeScriptTypes(pure, { mode: 'strip' }) + '\nglobalThis.facts = failureFacts; globalThis.append = appendDiagnostic; globalThis.launch = launchDiagnostic;', sandbox);
const facts = (...args) => JSON.parse(JSON.stringify(sandbox.facts(...args)));
const normal = (error, capture, body = '') => facts(error, capture, null, null, false, false, Buffer.from(body), Buffer.byteLength(body), 0);

test('actual diagnostic append stays capped through repeated native flood chunks', () => {
  const chunk = Buffer.alloc(256 << 10, 0x61);
  let retained = Buffer.alloc(0), total = 0;
  for (let index = 0; index < 256; index++) {
    retained = sandbox.append(retained, chunk);
    total += chunk.length;
    assert(retained.length <= (1 << 20) + 1);
  }
  assert.equal(retained.length, (1 << 20) + 1);
  assert.equal(retained[retained.length - 1], 0x61);
  const row = facts(new Error('native merged stream overflow'), undefined, null, null, false, false, retained, total, 0);
  assert.equal(row.reason, 'publisher-stream-overflow');
  assert.equal(row.publisherMergedBytes, (1 << 20) + 1);
  assert.equal(row.mergedBytesCapped, true);
  const giant = sandbox.append(Buffer.alloc(0), Buffer.alloc(4 << 20, 0x62));
  assert.equal(giant.length, (1 << 20) + 1);
  assert(source.includes('diagnostic = appendDiagnostic(diagnostic, chunk)'));
  assert(!source.includes('diagnostic = Buffer.concat([diagnostic, chunk])'));
});

test('known own reasons and capture failures retain only the closed vocabulary', () => {
  const row = normal(new Error('publisher startup failed'), new Error('publisher proof missing'));
  assert.equal(row.reason, 'publisher-startup-failed');
  assert.equal(row.captureReason, 'publisher-proof-missing');
  assert.equal(row.proofSeen, false);
  assert.equal(row.endpointSeen, false);
  assert.equal(normal(new Error('stage deadline exceeded')).reason, 'stage-deadline');
});

test('arbitrary exception, endpoint, cookie, profile and native buffer bytes never escape', () => {
  const privateValues = ['ws://127.0.0.1:52345/devtools/browser/private-endpoint', 'cookie-private-0123456789', '/private/user/profile/config-secret'];
  const privateText = privateValues.join(' ');
  for (const error of [new Error(privateText), { message: privateText }, privateText]) {
    const value = normal(error, new Error(privateText), privateText);
    assert.equal(value.reason, 'unclassified');
    assert.equal(value.captureReason, 'unclassified');
    assert(!JSON.stringify(value).includes('private'));
    assert(!JSON.stringify(value).includes('127.0.0.1'));
  }
});

test('publisher terminal facts and byte counts have fixed safe bounds', () => {
  const row = facts(new Error('publisher startup failed'), undefined, 127, 'SIGABRT', true, true, Buffer.alloc(0), 2 ** 40, 2 ** 40);
  assert.equal(row.publisherExitCode, 127);
  assert.equal(row.publisherSignal, 'SIGABRT');
  assert.equal(row.proofSeen, true);
  assert.equal(row.endpointSeen, true);
  assert.equal(row.publisherMergedBytes, (1 << 20) + 1);
  assert.equal(row.preVerifierStderrBytes, (1 << 20) + 1);
  assert.equal(row.mergedBytesCapped, true);
  const invalid = facts(undefined, undefined, 'private-code', 'private-signal', false, false, Buffer.alloc(0), NaN, -1);
  assert.equal(invalid.publisherExitCode, null);
  assert.equal(invalid.publisherSignal, 'other');
  assert.equal(invalid.publisherMergedBytes, 0);
  assert(!JSON.stringify(invalid).includes('private'));
  const opaque = facts(undefined, undefined, {}, { toString: () => 'SIGTERM', private: 'must-not-escape' }, false, false, Buffer.alloc(0), 0, 0);
  assert.equal(opaque.publisherSignal, 'other');
  assert(!JSON.stringify(opaque).includes('must-not-escape'));
});

test('native startup signatures are fixed booleans and do not expose matched text', () => {
  for (const [text, field] of [
    ['No usable sandbox!', 'sandboxUnavailable'],
    ['The SUID sandbox helper binary is missing: /private/publisher/chrome-sandbox', 'sandboxUnavailable'],
    ['The SUID sandbox helper binary was found, but is not configured correctly. /private/publisher', 'sandboxUnavailable'],
    ['Failed to move to new namespace', 'namespaceDenied'],
    ['error while loading shared libraries: private.so', 'sharedLibraryMissing'],
    ["version GLIBC_2.39 not found in /private/library", 'glibcVersionMissing'],
    ['Missing X server or $DISPLAY', 'displayUnavailable']
  ]) {
    const row = normal(new Error('publisher startup failed'), undefined, text);
    assert.equal(row.startupSignatures[field], true);
    assert(Object.values(row.startupSignatures).every(value => typeof value === 'boolean'));
    assert(!JSON.stringify(row).includes(text));
  }
});

test('terminal failure publication follows publisher cleanup and waits for supervised shutdown', () => {
  const reporting = source.slice(source.indexOf('  } catch (error) {\n    const metadata ='));
  assert(reporting.includes('const facts = failureFacts('));
  const ui = source.slice(source.indexOf('async function failureUI'), source.indexOf('async function main'));
  assert(!/textContent|getAttribute|\.slice\(/u.test(ui));
  assert(reporting.includes('visibleNativeUI: metadata'));
  assert(reporting.indexOf('failureUI(page)') < reporting.indexOf('browser.close()'));
  assert(!/String\(error\)|error\.message|diagnostic\.toString|endpoint\s*[,}]/u.test(reporting));
  const settled = reporting.indexOf('if (!await stopped(child, Math.max(1, failureShutdownDeadline');
  const publication = reporting.indexOf('renameSync(pending, req.driverResultFile)');
  const drained = reporting.indexOf('if (publisherClosed) await within(publisherClosed');
  assert(settled >= 0 && drained > settled && publication > drained);
  assert(reporting.indexOf('const facts = failureFacts(') > drained);
  assert(reporting.indexOf('await parentShutdown()') > publication);
  assert(reporting.includes('diagnostic.fill(0)'));
});

// Actual asynchronous picker and terminal handoff helpers.
function section(start, end) { return source.slice(source.indexOf(start), source.indexOf(end, source.indexOf(start))); }
function helpers(rowsByPoll, textRead) {
  let now = 0, polls = 0;
  const sandbox = { Error, performance: { now: () => now }, delay: async ms => { now += ms; },
    setTimeout, clearTimeout };
  vm.createContext(sandbox);
  const actual = section('function object(', 'function text(')
    + section('async function labelRow(', 'async function pick(')
    + section('function collectedResult(', 'async function focusWorkbench');
  vm.runInContext(stripTypeScriptTypes(actual, { mode: 'strip' })
    + '\nglobalThis.api = {labelRow, collectedResult, publisherTerminal, finalCapture, stopped};', sandbox);
  const page = { locator() {
    const labels = rowsByPoll[Math.min(polls++, rowsByPoll.length - 1)];
    return { count: async () => labels.length, nth: i => ({ label: labels[i], locator: () => ({
      count: async () => 1, innerText: async options => textRead ? textRead(labels[i], options) : labels[i] }) }) };
  } };
  return { ...sandbox.api, page, polls: () => polls };
}
test('actual picker waits for asynchronously populated exact label', async () => {
  const h = helpers([[], ['foreign'], ['owned']]);
  assert.equal((await h.labelRow(h.page, 'owned')).label, 'owned');
  assert.equal(h.polls(), 3);
});
test('actual picker retains uniqueness, cardinality and finite missing deadline', async () => {
  for (const rows of [['owned', 'owned'], Array(257).fill('owned'), ['foreign']]) {
    const h = helpers([rows]);
    await assert.rejects(h.labelRow(h.page, 'owned'), /ambiguous|bound exceeded|missing/);
    assert(h.polls() <= 61);
  }
});
const identity = { mode: 'trusted-project-v1', stage: 'empty-a', nonce: 'a'.repeat(32) };
const control = { label: 'uap-foreign-a', uri: `http://127.0.0.1:9/preserved-foreign/${identity.nonce}/a` };
const req = { ...identity, readinessControl: control };
const result = { schemaVersion: 1, ...identity, collected: true, version: '1', appRoot: '/app', appHost: 'desktop',
  folder: '/a', home: '/home', trusted: true, observerPID: 1, networkNamespace: 2, skills: [], definitions: [], calls: [],
  readiness: { ...control, definitionsReady: true, gatewayReady: true } };
test('actual observer handoff rejects failure, identity mismatch and unknown envelope', () => {
  const h = helpers([[]]);
  assert.equal(h.collectedResult(result, req), result);
  for (const invalid of [null, [], { ...result, collected: false }, { ...result, nonce: 'other' },
    { ...result, stage: 'other' }, { ...result, skills: 'malformed' }, { ...result, extra: 'private' }]) {
    assert.throws(() => h.collectedResult(invalid, req));
  }
});
test('actual handoff requires both readiness channels joined to the sealed control', () => {
  const h = helpers([[]]);
  for (const readiness of [undefined, {}, { ...result.readiness, definitionsReady: false },
    { ...result.readiness, gatewayReady: 1 }, { ...result.readiness, uri: control.uri + '-stale' },
    { ...result.readiness, label: 'uap-foreign-b' }, { ...result.readiness, endpoint: 'private' }]) {
    assert.throws(() => h.collectedResult({ ...result, readiness }, req));
  }
  for (const readinessControl of [{ ...control, uri: 'http://127.0.0.1:9/foreign' },
    { ...control, extra: 'private' }, { label: 'uap-foreign-b', uri: control.uri }]) {
    assert.throws(() => h.collectedResult(result, { ...req, readinessControl }));
  }
});
test('actual stop callback late capture is refused before successful handoff', async () => {
  const h = helpers([[]]);
  let capture;
  const child = { exitCode: null, signalCode: null, removeListener() {}, once(event, callback) {
    assert.equal(event, 'exit');
    capture = new Error('native merged stream overflow');
    callback();
  } };
  assert.equal(await h.stopped(child, 100), true);
  let closed;
  const drained = new Promise(resolve => { closed = resolve; });
  capture = undefined;
  const handoff = h.finalCapture(drained, 100, () => capture);
  capture = new Error('native merged stream overflow');
  closed();
  await assert.rejects(handoff, /native merged stream overflow/);
  await h.finalCapture(Promise.resolve(), 100, () => undefined);
  const main = source.slice(source.indexOf('const result = collectedResult('));
  assert(main.indexOf('await finalCapture(publisherClosed') > main.indexOf('await stopped(child'));
  assert(main.indexOf('await finalCapture(publisherClosed') < main.indexOf("type: 'driver-result'"));
});

test('actual final capture refuses pipes that never close within the stop budget', async () => {
  const h = helpers([[]]);
  await assert.rejects(h.finalCapture(new Promise(() => {}), 5, () => undefined), /bounded operation deadline/);
});

test('actual publisher terminal guard refuses nonzero exits and foreign signals', () => {
  const h = helpers([[]]);
  h.publisherTerminal(0, null);
  h.publisherTerminal(null, 'SIGTERM');
  for (const [code, signal] of [[1, null], [null, 'SIGTRAP'], [null, null], [0, 'SIGTERM']])
    assert.throws(() => h.publisherTerminal(code, signal), /publisher terminal result differs/);
});

test('actual picker bounds delayed and stale text reads by its own remaining budget', async () => {
  const h = helpers([['owned']], async (label, options) => {
    assert(options.timeout > 0 && options.timeout <= 3000);
    await new Promise(resolve => setTimeout(resolve, 5));
    return label;
  });
  assert.equal((await h.labelRow(h.page, 'owned')).label, 'owned');
  const stale = helpers([['owned']], async (_label, options) => {
    assert(options.timeout > 0 && options.timeout <= 3000);
    throw new Error('inert detached row');
  });
  await assert.rejects(stale.labelRow(stale.page, 'owned'), /inert detached row/);
  assert(source.includes('shutdownDeadline ??= performance.now() + 5000;'));
});

function inputHarness(states) {
  let now = 0, polls = 0, current, reads = 0;
  const context = { Error, performance: { now: () => now }, delay: async ms => { now += ms; }, clearTimeout,
    setTimeout: (fn, ms) => setTimeout(() => { now += ms; fn(); }, Math.min(ms, 5)) };
  vm.createContext(context);
  vm.runInContext(stripTypeScriptTypes(section('async function quickInput(', 'async function labelRow(')
    + section('async function within<', 'async function focusWorkbench'), { mode: 'strip' }) + '\nglobalThis.run = quickInput;', context);
  const input = { count: async () => { current = states[Math.min(polls++, states.length - 1)]; if (current.countHang) return new Promise(() => {}); return current.count ?? 1; },
    isVisible: async () => current.visible !== false, getAttribute: async (_key, options) => {
      reads++; assert(options.timeout > 0 && options.timeout <= 3000 - now);
      if (current.hang) return new Promise(() => {});
      if (current.slow) now += 4000;
      if (current.throw) throw new Error('private detached input text');
      return current.placeholder;
    } };
  return { run: () => context.run({ locator: () => input }, 'expected'), input, reads: () => reads, polls: () => polls };
}
test('actual input transition waits through visible old, absent and detached inputs', async () => {
  for (const initial of [{ placeholder: 'old' }, { count: 0 }, { throw: true }, { visible: false }]) {
    const h = inputHarness([initial, { placeholder: 'expected' }]);
    assert.equal(await h.run(), h.input); assert.equal(h.polls(), 2);
  }
});
test('actual input transition refuses duplicates and one shared expired read budget', async () => {
  for (const state of [{ placeholder: 'wrong' }, { count: 0 }, { count: 2 }, { countHang: true }, { hang: true }, { slow: true, placeholder: 'expected' }, { throw: true }]) {
    const h = inputHarness([state]);
    await assert.rejects(h.run(), /native picker differs|native picker input is ambiguous|bounded operation deadline/);
    assert(h.polls() <= 61);
    if (state.count === 2) assert.equal(h.reads(), 0);
  }
});
test('actual MCP substeps retain a fixed phase before an operation can fail', async () => {
  for (const failure of ['list', 'server', 'action', 'start', 'trust']) {
    let phase, picks = 0;
    const stop = () => { throw new Error('private injected operation'); };
    const context = { Error, performance: { now: () => 0 },
      listServers: async (_page, mark) => { mark('mcp-server-picker'); if (failure === 'list') stop(); },
      pick: async () => { picks++; if ((failure === 'server' && picks === 1) || (failure === 'start' && picks === 2)) stop(); },
      quickInput: async () => { if (failure === 'action') stop(); }, visibleDialogs: async () => stop() };
    vm.createContext(context);
    vm.runInContext(stripTypeScriptTypes(section('async function startServer(', 'async function input('), { mode: 'strip' }) + '\nglobalThis.run = startServer;', context);
    await assert.rejects(context.run({}, { label: 'owned' }, 60000, value => { phase = value; }), /private injected operation/);
    assert.equal(phase, { list: 'mcp-server-picker', server: 'mcp-server-selection', action: 'mcp-action-picker', start: 'mcp-start-action', trust: 'mcp-trust' }[failure]);
  }
});
test('owned picker errors map to finite reasons while arbitrary text stays opaque', () => {
  for (const [message, reason] of [['native picker differs', 'picker-input-deadline'], ['native picker input is ambiguous', 'picker-input-ambiguous'],
    ['native picker label missing', 'picker-label-missing'], ['native picker label is ambiguous', 'picker-label-ambiguous'],
    ['native picker row bound exceeded', 'picker-row-bound'], ['native MCP trust origin differs', 'mcp-trust-origin']])
    assert.equal(normal(new Error(message)).reason, reason);
  assert.equal(normal(new Error('private arbitrary picker text')).reason, 'unclassified');
});

test('actual server-list transition marks command and expected picker separately', async () => {
  for (const failAt of [1, 2]) {
    let phase, calls = 0;
    const context = { Error, noDialog: async () => {}, pick: async () => {}, quickInput: async () => {
      if (++calls === failAt) throw new Error('inert picker failure'); return { fill: async () => {} };
    } };
    vm.createContext(context);
    vm.runInContext(stripTypeScriptTypes(section('async function listServers(', 'async function workspaceTrust('), { mode: 'strip' }) + '\nglobalThis.run = listServers;', context);
    await assert.rejects(context.run({ keyboard: { press: async () => {} } }, value => { phase = value; }), /inert picker failure/);
    assert.equal(phase, failAt === 1 ? 'mcp-command-palette' : 'mcp-server-picker');
  }
});

test('actual trigger publisher is exclusive, complete and single-link; collisions preserve foreign bytes', () => {
  const fs = require('node:fs'), os = require('node:os'), paths = require('node:path');
  const directory = fs.mkdtempSync(paths.join(os.tmpdir(), 'uap-trigger-'));
  const triggerFile = paths.join(directory, 'ready'), req = { triggerFile, uid: process.getuid(), gid: process.getgid(), stage: 'a', nonce: 'nonce' };
  function publisher(overrides = {}) {
    const box = { ...fs, ...paths, Buffer, Error, outputPath: value => value, ...overrides };
    vm.createContext(box);
    vm.runInContext(stripTypeScriptTypes(source.slice(source.indexOf('function publishTrigger('), source.indexOf('async function failureUI(')), { mode: 'strip' }) + '\nglobalThis.publish = publishTrigger;', box);
    return box.publish;
  }
  try {
    publisher()(req); assert.equal(fs.readFileSync(triggerFile, 'utf8'), JSON.stringify({ stage: 'a', nonce: 'nonce' }) + '\n');
    assert.equal(fs.statSync(triggerFile).nlink, 1); assert.equal(fs.existsSync(triggerFile + '.pending'), false);
    assert.throws(() => publisher()(req)); assert.equal(fs.statSync(triggerFile).nlink, 1);
    fs.unlinkSync(triggerFile); fs.unlinkSync(triggerFile + '.pending');
    fs.writeFileSync(triggerFile + '.pending', 'foreign'); assert.throws(() => publisher()(req));
    assert.equal(fs.readFileSync(triggerFile + '.pending', 'utf8'), 'foreign'); fs.unlinkSync(triggerFile + '.pending');
    assert.throws(() => publisher({ linkSync(from, to) { fs.writeFileSync(to, 'foreign'); fs.linkSync(from, to); } })(req));
    assert.equal(fs.readFileSync(triggerFile, 'utf8'), 'foreign');
  } finally { fs.rmSync(directory, { recursive: true, force: true }); }
});

test('MCP telemetry is finite and never coerces or emits hostile state values', () => {
  const hostile = { toString() { throw new Error('private'); }, valueOf() { throw new Error('private'); } };
  const row = facts(new Error('trigger publication refused'), undefined, null, null, true, true, Buffer.alloc(0), 0, 0,
    { startSelected: true, trustAccepted: false, lastState: hostile, stopped: hostile, starting: 999, running: -1, other: NaN, private: 'secret' });
  assert.deepEqual(row.mcp, { startSelected: true, trustAccepted: false, lastState: 'other', stopped: 0, starting: 255, running: 0, other: 0, launchNotification: false, outputVisible: false, launchError: 'none' });
  assert.equal(row.reason, 'trigger-publication-refused'); assert.equal(JSON.stringify(row).includes('secret'), false);
});

test('actual launch projector exports only expected native message classes and output identity', () => {
  const label = 'owned-server', secret = '/private/path COOKIE=token';
  const node = text => ({ text, getClientRects: () => [1], getAttribute: () => text });
  const root = (messages, outputs = []) => ({ querySelectorAll: selector => selector.startsWith('.notification') ? messages.map(node) : outputs.map(node),
    createTreeWalker: row => { let done = false; return { nextNode: () => done ? null : (done = true, { nodeValue: row.text }) }; } });
  const inspect = r => JSON.parse(JSON.stringify(sandbox.launch(label, r)));
  assert.deepEqual(inspect(root(['Error starting owned-server: CodeExpectedError: Variable ${env:}: can not be resolved because no environment variable name is given.'], ['MCP: owned-server - Output'])),
    { launchNotification: true, outputVisible: true, launchError: 'variable-resolution' });
  for (const messages of [['Error starting foreign: '+secret], ['Error starting owned-server: '+secret], [], Array(17).fill(secret), [secret.repeat(4096)], ['Error starting owned-server: x','Error starting owned-server: y']]) {
    const value = inspect(root(messages, ['MCP: foreign - Output']));
    assert.equal(value.outputVisible, false); assert(!JSON.stringify(value).includes(secret));
    assert.deepEqual(Object.keys(value).sort(), ['launchError','launchNotification','outputVisible']);
  }
  assert.equal(inspect(root(['Error starting owned-server: '+secret])).launchError, 'other');
  assert.equal(inspect(root(['Error starting foreign: '+secret])).launchNotification, false);
  assert.equal(inspect({ querySelectorAll() { throw new Error(secret); } }).launchError, 'diagnostic-refused');
  assert.equal(inspect(root([], ['MCP: owned-server - Output','MCP: owned-server - Output'])).launchError, 'diagnostic-refused');
});

test('actual start rejects a late Running row after diagnostic exhausts original budget', async () => {
  for (const throws of [false, true]) {
    let clock = 0, rows = 0;
    const context = { Error, performance: { now: () => clock }, listServers: async () => {}, pick: async () => {}, quickInput: async () => {},
      visibleDialogs: async () => ({ count: async () => 0 }), delay: async () => {}, launchDiagnostic: () => {}, within: async work => work,
      labelRow: async () => { rows++; return { locator: () => ({ innerText: async () => 'Running' }) }; } };
    vm.createContext(context); vm.runInContext(stripTypeScriptTypes(section('async function startServer(', 'async function input('), { mode: 'strip' }) + '\nglobalThis.run = startServer;', context);
    const page = { evaluate: async () => { clock = 15000; if (throws) throw new Error('private'); return {}; } };
    await assert.rejects(context.run(page, { label: 'owned' }, 60000, () => {}), /native MCP startup deadline exceeded/);
    assert.equal(rows, 0);
  }
});

test('actual start rejects Running when final state read exhausts original budget', async () => {
  let clock = 0;
  const context = { Error, performance: { now: () => clock }, listServers: async () => {}, pick: async () => {}, quickInput: async () => {},
    visibleDialogs: async () => ({ count: async () => 0 }), delay: async () => {}, launchDiagnostic: () => {}, within: async work => work,
    labelRow: async () => ({ locator: () => ({ innerText: async () => { clock = 15000; return 'Running'; } }) }) };
  vm.createContext(context); vm.runInContext(stripTypeScriptTypes(section('async function startServer(', 'async function input('), { mode: 'strip' }) + '\nglobalThis.run = startServer;', context);
  await assert.rejects(context.run({ evaluate: async () => ({}) }, { label: 'owned' }, 60000, () => {}), /native MCP startup deadline exceeded/);
});
