import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createCore } from '../observer-core.js';
const tick = () => new Promise(setImmediate);
const deferred = () => { let resolve; const promise = new Promise((r) => { resolve = r; }); return { promise, resolve }; };
const fact = () => ({ version: 1, kind: 'terminal_error', sessionID: 'session', turnID: 'user', rootSession: true,
  provenance: { generation: 'v1', nativeMessageID: 'assistant', observationID: 'native', nativeTime: 1, timeBasis: 'assistant_created_lower_bound' } });
function token(core, controller = new AbortController(), deadline = core.now() + 2000) {
  return { signal: controller.signal, isCurrent: () => !controller.signal.aborted,
    ingressMonotonicMs: 0, clockID: core.clockID, metadataDeadline: deadline };
}

test('preparation is literal-true, frozen, isolated from checkpoint, then the same fact enters synchronous delivery', async () => {
  const order = [], prepared = new WeakMap(), candidate = fact();
  const core = createCore({ beforeEmit: async (event, handoff) => {
    order.push('prepare'); assert.ok(Object.isFrozen(event)); assert.ok(Object.isFrozen(event.provenance));
    assert.ok(Object.isFrozen(handoff)); assert.equal(handoff.revalidate, undefined);
    prepared.set(event, 'frame'); return true;
  }, emit: (event) => { assert.equal(event, candidate); assert.equal(prepared.get(event), 'frame'); order.push('spawn'); } });
  assert.equal(await core.emit(candidate, { ...token(core), revalidate: async () => { order.push('snapshot'); return true; } }), true);
  assert.deepEqual(order, ['prepare', 'snapshot', 'spawn']);
  for (const result of [false, undefined, 1, 'true', new Error('failed')]) {
    let snapshots = 0, spawns = 0;
    const rejected = createCore({ beforeEmit() { if (result instanceof Error) throw result; return result; }, emit() { spawns++; } });
    assert.equal(await rejected.emit(fact(), { ...token(rejected), revalidate: async () => { snapshots++; return true; } }), false);
    assert.equal(snapshots, 0); assert.equal(spawns, 0);
  }
});

test('abort-ignoring preparation retains lookup, callback and end-to-end job until actual settlement', async () => {
  const pauses = Array.from({ length: 4 }, deferred), controllers = Array.from({ length: 4 }, () => new AbortController());
  const diagnostics = [], signals = []; let entries = 0, lookups = 0, spawns = 0;
  const core = createCore({ maxJobs: 1, maxConcurrentLookups: 4, onDiagnostic: (r) => diagnostics.push(r),
    beforeEmit: (_event, handoff) => { signals.push(handoff.signal); return pauses[entries++].promise; }, emit() { spawns++; } });
  const jobs = controllers.map((c, i) => i === 0
    ? core.submit('held', () => true, (h) => core.emit(fact(), h))
    : core.emit(fact(), token(core, c)));
  await tick(); assert.equal(entries, 4);
  core.invalidate(); controllers.forEach((c) => c.abort()); await tick();
  assert.ok(signals.every((s) => s.aborted)); assert.ok(core.busy());
  assert.equal(core.submit('extra', () => true, () => { spawns++; }), undefined);
  await core.lookup(() => { lookups++; }, token(core)); assert.equal(lookups, 0);
  const fifthController = new AbortController(), fifth = core.emit(fact(), token(core, fifthController));
  await tick(); assert.equal(entries, 4); fifthController.abort(); await fifth;
  assert.ok(diagnostics.includes('job_capacity')); assert.ok(diagnostics.includes('lookup_capacity'));
  pauses.forEach((p) => p.resolve(true)); await Promise.all(jobs);
  assert.equal(core.busy(), false); assert.equal(spawns, 0);
  await core.lookup(() => { lookups++; return true; }, token(core)); assert.equal(lookups, 1);
});

test('callback capacity wakes on the original deadline while occupied callbacks remain owned', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  let elapsed = 0, calls = 0;
  const pauses = Array.from({ length: 4 }, deferred);
  const core = createCore({ clock: { id: 'TEST-clock', now: () => elapsed }, emit: () => pauses[calls++].promise });
  const jobs = pauses.map(() => core.emit(fact(), token(core)));
  await tick(); assert.equal(calls, 4);
  let settled = false;
  const waiting = core.emit(fact(), token(core)).then((r) => { settled = true; return r; });
  elapsed = 2000; t.mock.timers.tick(2000); await tick();
  assert.equal(settled, true); assert.equal(await waiting, false); assert.equal(calls, 4);
  pauses.forEach((p) => p.resolve()); await Promise.all(jobs);
});

test('metadata and abort-ignoring preparation share one deadline without releasing early on timeout', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  let elapsed = 0, captured, snapshots = 0, spawns = 0;
  const pause = deferred();
  const core = createCore({ clock: { id: 'TEST-clock', now: () => elapsed },
    beforeEmit: (_event, handoff) => { captured = handoff; return pause.promise; }, emit() { spawns++; } });
  const h = token(core);
  await core.lookup(async () => { elapsed = 1700; return {}; }, h);
  let ended = false;
  const delivery = core.emit(fact(), { ...h, revalidate: async () => { snapshots++; return true; } }).then((r) => { ended = true; return r; });
  await tick(); assert.equal(captured.metadataDeadline, 2000); assert.equal(captured.ingressMonotonicMs, 0);
  elapsed = 2000; t.mock.timers.tick(300); await tick();
  assert.equal(captured.signal.aborted, true); assert.equal(captured.isCurrent(), false); assert.equal(ended, false);
  pause.resolve(true); assert.equal(await delivery, false); assert.equal(snapshots, 0); assert.equal(spawns, 0);
});
