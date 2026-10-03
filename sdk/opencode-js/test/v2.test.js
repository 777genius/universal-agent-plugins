import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createV2Observer } from '../v2.js';

const tick = () => new Promise(setImmediate);
const deferred = () => { let resolve; const promise = new Promise((r) => { resolve = r; }); return { promise, resolve }; };
// Independent live source with real iterator settlement and abort handling.
// This is a unit contract model, not a native host or installed-host fixture.
class LiveSource {
  queue = []; waiting; ended = false; failure;
  constructor(signal) { this.signal = signal; signal.addEventListener('abort', () => this.end(), { once: true }); }
  push(value) { this.queue.push(value); this.wake(); }
  wake() { this.waiting?.(); this.waiting = undefined; }
  end(error) { this.ended = true; this.failure = error; this.wake(); }
  async *[Symbol.asyncIterator]() {
    while (!this.ended) {
      if (this.queue.length) yield this.queue.shift();
      else await new Promise((r) => { this.waiting = r; });
    }
    if (this.failure) throw this.failure;
  }
}
function service(overrides = {}) {
  const { native: nativeOverrides, checkpoints = false, holdMarkers = false, ...otherOverrides } = overrides;
  const markers = [], transport = { hold: holdMarkers, disposed: 0 };
  const sources = [], out = [], diagnostics = [], pending = new Map(), turns = new Map();
  const sequences = new Map(); let serial = 0;
  const context = { app: { version: '2.0.21' }, event: { subscribe({ signal }) {
    const source = new LiveSource(signal); sources.push(source); return source;
  } } };
  const scope = (run) => ({ sessionID: run.sessionID, turnID: turns.get(run.sessionID), location: 'TEST-location', rootSession: true });
  const checkpoint = checkpoints ? { async register(signal, namespace) {
    const type = `private.checkpoint-${namespace}`, source = sources.at(-1);
    return { type,
      async emit(nonce) {
        const event = { id: `marker-${markers.length}`, created: 1790867856742, type, data: { nonce } };
        markers.push({ event, source });
        if (!transport.hold) source.push(event);
      },
      read(e) { return Object.keys(e.data).length === 1 ? e.data.nonce : ''; },
      dispose() { transport.disposed++; },
    };
  } } : undefined;
  const options = { checkpoint, context, location: 'TEST-location', runtimeEligibility: () => 'supported',
    native: { correlate: (e) => e.data, session: async (run) => scope(run),
      assistant: async (run) => ({ ...scope(run), messageID: run.messageID, role: 'assistant', summary: false, final: true, outcome: 'success' }),
      currentPermission: async (run) => pending.get(run.requestID),
      questionSource: async (run) => ({ ...scope(run), requestID: run.requestID, messageID: run.messageID,
        callID: run.callID, role: 'assistant', summary: false, tool: 'question' }),
    }, emit: (fact) => out.push(fact), onDiagnostic: (reason) => diagnostics.push(reason), ...otherOverrides };
  options.native = { ...options.native, ...nativeOverrides };
  const observer = createV2Observer(options); observer.start();
  function publish(type, data = {}, nativeID = `event-${++serial}`) {
    const sid = data.sessionID ?? 'session';
    if (type === 'session.execution.started') {
      turns.set(sid, nativeID);
      publish('session.inbox.enqueued', { sessionID: sid, inboxID: `user-${nativeID}`, item: { type: 'user' } });
    }
    const seq = (sequences.get(sid) ?? 0) + 1;
    if (!type.startsWith('permission.') && !type.startsWith('form.')) sequences.set(sid, seq);
    const event = { id: nativeID, created: 1790867856742, type, data: {
      sessionID: sid, location: 'TEST-location', ...data,
      ...(!type.startsWith('permission.') && !type.startsWith('form.') ? { sequence: data.sequence ?? { aggregate: sid, seq } } : {}),
    } };
    if (type === 'permission.asked') pending.set(data.requestID, {
      sessionID: sid, turnID: turns.get(sid), location: 'TEST-location', rootSession: true, pending: true,
      requestID: data.requestID, messageID: data.messageID, callID: data.callID,
    });
    if (type === 'permission.replied') pending.delete(data.requestID);
    sources.at(-1).push(event);
    if (type === 'session.execution.started') {
      publish('session.inbox.delivered', { sessionID: sid, inboxID: `user-${nativeID}` });
      publish('session.step.started', { sessionID: sid, messageID: 'assistant' });
    }
    return event;
  }
  return { observer, sources, out, diagnostics, publish, pending, turns, markers, transport,
    consumeMarker(index = markers.length - 1) { const m = markers[index]; m.source.push(m.event); },
    async stop() { observer.dispose(); await observer.done(); } };
}
const permission = { requestID: 'permission', messageID: 'assistant', callID: 'call' };
async function start(s) { s.publish('session.execution.started', {}, 'native-start'); await tick(); }
async function terminal(s, type = 'session.execution.succeeded') {
  s.publish('session.step.ended', { messageID: 'assistant' });
  s.publish(type, { messageID: 'assistant' }); await tick();
}

test('settlement reaches ingress while metadata is paused and suppresses actual handoff', async () => {
  const paused = deferred(), entered = deferred();
  const s = service({ native: { session: async (run) => { entered.resolve(); await paused.promise;
    return { ...run, rootSession: true }; } } });
  await start(s); s.publish('permission.asked', permission); await entered.promise;
  s.publish('permission.replied', { requestID: permission.requestID }); await tick();
  paused.resolve(); await tick();
  assert.deepEqual(s.out, []); await s.stop();
});

test('settlement after pending read aborts a paused IPC adapter before spawn', async () => {
  const paused = deferred(), entered = deferred(), spawned = [];
  let captured;
  const s = service({ emit: async (fact, handoff) => { captured = handoff; entered.resolve(); await paused.promise;
    if (handoff.isCurrent()) spawned.push(fact); } });
  await start(s); s.publish('permission.asked', permission); await entered.promise;
  s.publish('permission.replied', { requestID: permission.requestID }); await tick();
  assert.equal(captured.signal.aborted, true); assert.equal(captured.isCurrent(), false);
  paused.resolve(); await tick(); assert.deepEqual(spawned, []); await s.stop();
});

test('terminal closes attention while retaining its immutable final native evidence', async () => {
  for (const failed of [false, true]) {
    const paused = deferred(), entered = deferred(); let calls = 0;
    const s = service({ native: { session: async (run) => {
      if (++calls === 1) { entered.resolve(); await paused.promise; }
      return { ...run, rootSession: true };
    }, assistant: async (run) => ({ ...run, rootSession: true, role: 'assistant', summary: false,
      final: true, outcome: failed ? 'error' : 'success' }) } });
    await start(s); s.publish('permission.asked', permission); await entered.promise;
    await terminal(s, failed ? 'session.execution.failed' : 'session.execution.succeeded'); paused.resolve(); await tick();
    assert.deepEqual(s.out.map((f) => [f.kind, f.turnID, f.messageID]), [[failed ? 'terminal_error' : 'turn_idle_verified', 'user-native-start', failed ? undefined : 'assistant']]);
    assert.equal(s.out[0].provenance.nativeTime, 1790867856742);
    assert.equal(s.out[0].provenance.timeBasis, 'envelope_created'); await s.stop();
  }
});

test('source error and natural end invalidate queued and active jobs before serialized work', async () => {
  for (const error of [undefined, new Error('private error body')]) {
    const paused = deferred(), entered = deferred();
    const s = service({ native: { session: async (run) => { entered.resolve(); await paused.promise; return { ...run, rootSession: true }; } } });
    await start(s); s.publish('permission.asked', permission); await entered.promise;
    s.publish('session.step.ended', { messageID: 'assistant' });
    s.publish('session.execution.succeeded', { messageID: 'assistant' });
    s.sources[0].end(error); await tick(); paused.resolve(); await tick();
    assert.deepEqual(s.out, []);
    assert.ok(s.diagnostics.includes(error ? 'subscription_error' : 'subscription_ended')); await s.stop();
  }
});

test('replacement budget is lifetime bounded even when every replacement briefly succeeds', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const s = service(); await start(s);
  for (const [i, ms] of [250, 1000, 2000].entries()) {
    s.sources.at(-1).end(); await tick();
    t.mock.timers.tick(ms - 1); await tick(); assert.equal(s.sources.length, i + 1);
    t.mock.timers.tick(1); await tick(); assert.equal(s.sources.length, i + 2);
    await start(s); // Successful live consumption must not replenish budget.
  }
  s.sources.at(-1).end(); await tick(); t.mock.timers.tick(10000); await tick();
  assert.equal(s.sources.length, 4); assert.ok(s.diagnostics.includes('subscription_replacements_exhausted'));
  await s.stop();
});

test('job overflow suppresses the affected execution but control settlement still invalidates it', async () => {
  const paused = deferred(), entered = deferred();
  const s = service({ maxJobs: 1, native: { session: async (run) => { entered.resolve(); await paused.promise; return { ...run, rootSession: true }; } } });
  await start(s); s.publish('permission.asked', permission); await entered.promise;
  s.publish('permission.asked', { ...permission, requestID: 'other' }); await tick();
  s.publish('permission.replied', { requestID: permission.requestID }); await tick(); paused.resolve(); await tick();
  assert.deepEqual(s.out, []); assert.ok(s.diagnostics.includes('job_capacity')); await s.stop();
});

test('independent instances compute identical canonical native IDs, including after reconnect', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const a = service(), b = service();
  b.sources[0].end(); await tick(); t.mock.timers.tick(250); await tick();
  await start(a); await start(b); await terminal(a); await terminal(b);
  assert.equal(a.out.length, 1); assert.equal(b.out.length, 1);
  assert.equal(a.out[0].provenance.observationID, b.out[0].provenance.observationID);
  assert.equal(a.out[0].turnID, 'user-native-start'); await a.stop(); await b.stop();
});

test('root, child and observed location must agree independently of load location', async () => {
  for (const scope of [{ rootSession: false }, { location: 'other-project' }, { turnID: 'old-execution' }]) {
    const s = service({ native: { session: async (run) => ({ ...run, rootSession: true, ...scope }) } });
    await start(s); await terminal(s); assert.deepEqual(s.out, []); await s.stop();
  }
  const s = service(); await start(s);
  s.publish('permission.asked', { ...permission, location: 'unrelated-project' }); await tick();
  await terminal(s); assert.deepEqual(s.out, []); await s.stop();
});

test('missing public checkpoint binding stays closed despite observed live creation', async () => {
  const s = service(); await start(s);
  s.publish('form.created', { requestID: 'form', messageID: 'assistant', callID: 'question-call', fields: 'SECRET' }); await tick();
  assert.deepEqual(s.out, []); assert.ok(s.diagnostics.includes('form_checkpoint_unavailable'));
  s.publish('form.cancelled', { requestID: 'form' }); await tick(); await s.stop();
});

test('step failure/retry/interruption are silent and conflicting execution terminals cannot both hand off', async () => {
  const s = service({ native: { assistant: async (run) => ({ ...run, rootSession: true, role: 'assistant', summary: false, final: true, outcome: 'error' }) } });
  await start(s); s.publish('session.step.failed', { messageID: 'assistant' }); await tick(); assert.deepEqual(s.out, []);
  s.publish('session.execution.failed', { messageID: 'assistant' }); await tick();
  s.publish('session.execution.succeeded', { messageID: 'assistant' }); await tick();
  assert.deepEqual(s.out.map((f) => f.kind), ['terminal_error']); await s.stop();
  const quiet = service(); await start(quiet);
  quiet.publish('session.retry.scheduled'); quiet.publish('session.execution.interrupted');
  await terminal(quiet); assert.deepEqual(quiet.out, []); await quiet.stop();
});

test('a native lower sequence contradicts ordering and invalidates paused relevance', async () => {
  const paused = deferred(), entered = deferred();
  const s = service({ native: { session: async (run) => { entered.resolve(); await paused.promise; return { ...run, rootSession: true }; } } });
  await start(s); s.publish('permission.asked', permission); await entered.promise;
  s.publish('session.step.ended', { messageID: 'assistant', sequence: { aggregate: 'session', seq: 0 } });
  await tick(); paused.resolve(); await tick(); assert.deepEqual(s.out, []);
  assert.ok(s.diagnostics.includes('native_sequence_contradiction')); await s.stop();
});

test('unqualified runtime never even subscribes', async () => {
  const s = service({ runtimeEligibility: () => 'unverified' });
  assert.equal(s.sources.length, 0); assert.deepEqual(s.out, []);
  assert.deepEqual(s.diagnostics, ['runtime_unverified']); await s.stop();
});

test('16 active native lookups remain occupied until actual settlement even after 2s deadline', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  let active = 0, peak = 0; const paused = deferred();
  const s = service({ native: { session: async (run) => {
    active++; peak = Math.max(active, peak); await paused.promise; active--; return { ...run, rootSession: true };
  } } });
  for (let i = 0; i < 16; i++) {
    const sessionID = `session-${i}`;
    s.publish('session.execution.started', { sessionID });
    s.publish('permission.asked', { ...permission, sessionID, requestID: `permission-${i}` });
  }
  await tick(); assert.equal(peak, 16);
  t.mock.timers.tick(1999); await tick(); assert.equal(active, 16);
  t.mock.timers.tick(1); await tick(); assert.ok(s.diagnostics.includes('lookup_timeout'));
  // Abort-ignoring native reads must not permit unbounded replacement reads.
  t.mock.timers.tick(250); await tick();
  s.publish('session.execution.started', { sessionID: 'later' });
  s.publish('permission.asked', { ...permission, sessionID: 'later', requestID: 'later' }); await tick();
  assert.equal(peak, 16); assert.ok(s.diagnostics.includes('lookup_capacity')); assert.deepEqual(s.out, []); paused.resolve(); await tick(); await s.stop();
});

test('512 live session fences cannot be evicted by a 513th execution', async () => {
  const s = service();
  for (let i = 0; i < 513; i++) s.publish('session.execution.started', { sessionID: `session-${i}` });
  await tick(); assert.ok(s.diagnostics.includes('session_capacity'));
  s.publish('permission.asked', { ...permission, sessionID: 'session-512' }); await tick(); assert.deepEqual(s.out, []);
  s.publish('session.step.ended', { sessionID: 'session-0', messageID: 'assistant' });
  s.publish('session.execution.succeeded', { sessionID: 'session-0', messageID: 'assistant' }); await tick();
  assert.deepEqual(s.out, []); // Overflow invalidates every live authorization fence.
  await s.stop();
});

test('old permission and execution IDs cannot reopen relevance on reconnect', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] }); const s = service();
  const old = s.publish('session.execution.started', {}, 'native-old'); await tick();
  s.sources[0].end(); await tick(); t.mock.timers.tick(250); await tick();
  s.sources[1].push(old); s.publish('permission.asked', permission); await tick();
  assert.deepEqual(s.out, []);
  s.publish('session.execution.started', {}, 'native-new'); await tick();
  s.publish('permission.asked', { ...permission, requestID: 'fresh' }); await tick();
  assert.deepEqual(s.out.map((f) => f.turnID), ['user-native-new']); await s.stop();
});

test('actual pending authority closes an unconsumed settlement before callback admission', async () => {
  const entered = deferred(), paused = deferred(); const spawned = []; let checks = 0;
  const s = service({ native: { currentPermission: async (run) => {
    if (++checks === 1) { entered.resolve(); await paused.promise; }
    return s.pending.get(run.requestID);
  } }, emit: (fact) => spawned.push(fact) });
  await start(s); s.publish('permission.asked', permission); await entered.promise;
  // Native state settles before the event reader consumes a reply envelope.
  s.pending.delete(permission.requestID); paused.resolve(); await tick();
  assert.deepEqual(spawned, []); await s.stop();
});

test('the default 256 end-to-end slots include queued and active callback work, with four active callbacks', async () => {
  const pause = deferred(); let active = 0, peak = 0; const attempts = [];
  const s = service({ emit: async (fact, handoff) => {
    active++; peak = Math.max(peak, active); await pause.promise;
    if (handoff.isCurrent()) attempts.push(fact); active--;
  } });
  for (let i = 0; i < 257; i++) {
    const sessionID = `session-${i}`;
    s.publish('session.execution.started', { sessionID });
    s.publish('permission.asked', { ...permission, sessionID, requestID: `request-${i}` });
  }
  await tick(); assert.equal(peak, 4); assert.ok(s.diagnostics.includes('job_capacity'));
  s.observer.dispose(); pause.resolve(); await s.observer.done();
  assert.equal(active, 0); assert.deepEqual(attempts, []);
});

const form = { requestID: 'native-form', messageID: 'assistant', callID: 'question-call' };
test('readiness and job markers require same-reader consumption, emit return alone never authorizes', async () => {
  const s = service({ checkpoints: true, holdMarkers: true }); await start(s);
  assert.equal(s.markers.length, 1);
  s.publish('form.created', form); await tick(); assert.deepEqual(s.out, []);
  s.consumeMarker(0); await tick();
  s.publish('form.created', form); await tick();
  assert.equal(s.markers.length, 1); // A repeated pre-readiness form remains closed.
  s.publish('form.created', { ...form, requestID: 'future-form' }); await tick();
  assert.equal(s.markers.length, 2); assert.deepEqual(s.out, []);
  s.consumeMarker(1); await tick();
  assert.equal(s.markers.length, 2); // Exactly one final marker per job.
  assert.deepEqual(s.out.map((f) => [f.kind, f.requestID]), [['question_asked', 'future-form']]);
  assert.equal(s.out[0].provenance.nativeEventID.startsWith('event-'), true);
  await s.stop(); assert.equal(s.transport.disposed, 1);
});

test('native close queued before marker suppresses question while slow metadata and FIFO receipt settle', async () => {
  for (const type of ['form.replied', 'form.cancelled']) {
    const entered = deferred(), pause = deferred();
    const s = service({ checkpoints: true, native: { questionSource: async (run) => {
      entered.resolve(); await pause.promise;
      return { ...run, rootSession: true, role: 'assistant', summary: false, tool: 'question' };
    } } });
    await start(s); s.publish('form.created', form); await entered.promise;
    // Both packets are upstream FIFO entries; the reader consumes close first.
    s.publish(type, { requestID: form.requestID }); pause.resolve(); await tick();
    assert.deepEqual(s.out, []); await s.stop();
  }
});

test('settlement before checkpoint consumption invalidates paused attention without erasing terminal fact', async () => {
  const s = service({ checkpoints: true }); await start(s);
  s.transport.hold = true; s.publish('form.created', form); await tick();
  assert.equal(s.markers.length, 2);
  s.publish('form.cancelled', { requestID: form.requestID });
  s.consumeMarker(1); await terminal(s);
  assert.deepEqual(s.out.map((f) => f.kind), ['turn_idle_verified']); await s.stop();
});

test('after-snapshot close aborts an already paused adapter and forbids subsequent spawn', async () => {
  const entered = deferred(), pause = deferred(), spawned = []; let token;
  const s = service({ checkpoints: true, emit: async (f, handoff) => {
    token = handoff; entered.resolve(); await pause.promise;
    if (handoff.isCurrent() && await handoff.revalidate() && handoff.isCurrent()) spawned.push(f);
  } });
  await start(s); s.publish('form.created', form); await entered.promise;
  s.publish('form.replied', { requestID: form.requestID }); await tick();
  assert.equal(token.signal.aborted, true); pause.resolve(); await tick();
  assert.deepEqual(spawned, []); await s.stop();
});

test('marker timeout, duplicate consumption and stream closure revoke readiness and every job', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  for (const failure of ['timeout', 'duplicate', 'schema', 'end', 'error']) {
    const s = service({ checkpoints: true }); await start(s);
    s.transport.hold = true; s.publish('form.created', form); await tick();
    assert.equal(s.markers.length, 2);
    if (failure === 'timeout') t.mock.timers.tick(2000);
    if (failure === 'duplicate') s.consumeMarker(0);
    if (failure === 'schema') {
      const marker = s.markers[1].event;
      s.sources[0].push({ ...marker, data: { ...marker.data, extra: 'PRIVATE' } });
    }
    if (failure === 'end' || failure === 'error') s.sources[0].end(failure === 'error' ? new Error('private') : undefined);
    await tick();
    assert.equal(s.sources[0].signal.aborted, true);
    s.consumeMarker(1); await tick(); assert.deepEqual(s.out, []);
    await s.stop(); assert.equal(s.transport.disposed, 1);
  }
});

test('question metadata and markers share a single 2s deadline rather than resetting each read', async (t) => {
  let elapsed = 0; t.mock.method(performance, 'now', () => elapsed);
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const pause = deferred(), entered = deferred();
  const s = service({ checkpoints: true, native: { session: async (run) => {
    entered.resolve(); await pause.promise; return { ...run, rootSession: true };
  } } });
  await start(s); s.transport.hold = true;
  s.publish('form.created', form); await entered.promise;
  elapsed = 1500; t.mock.timers.tick(1500); pause.resolve(); await tick();
  assert.equal(s.markers.length, 2);
  t.mock.timers.tick(501); await tick();
  assert.deepEqual(s.out, []); assert.equal(s.sources[0].signal.aborted, true); await s.stop();
});

test('question source must be actual question assistant and duplicate instances share only canonical native identity', async () => {
  for (const change of [{ tool: 'shell' }, { summary: true }, { callID: 'other' }, { rootSession: false }, { turnID: 'other' }]) {
    const s = service({ checkpoints: true, native: { questionSource: async (run) => ({ ...run,
      rootSession: true, role: 'assistant', tool: 'question', summary: false, ...change }) } });
    await start(s); s.publish('form.created', form); await tick(); assert.deepEqual(s.out, []); await s.stop();
  }
  const a = service({ checkpoints: true }), b = service({ checkpoints: true });
  await start(a); await start(b);
  a.publish('form.created', form, 'native-created'); b.publish('form.created', form, 'native-created'); await tick();
  assert.equal(a.out.length, 1); assert.equal(b.out.length, 1);
  assert.equal(a.out[0].provenance.observationID, b.out[0].provenance.observationID);
  assert.notEqual(a.markers[0].event.data.nonce, b.markers[0].event.data.nonce);
  await a.stop(); await b.stop();
});

test('summary assistant cannot authorize normal completion on either automatic or manual compaction', async () => {
  const s = service({ native: { assistant: async (run) => ({ ...run, rootSession: true, role: 'assistant',
    summary: true, final: true, outcome: 'success' }) } });
  await start(s); await terminal(s); assert.deepEqual(s.out, []); await s.stop();
});

test('projected fork lineage does not disqualify a root but an actual subagent parent does', async () => {
  for (const [parentID, expected] of [[null, 1], ['actual-parent', 0]]) {
    const s = service({ native: { session: async (run) => ({ ...run, rootSession: parentID === null,
      parentID, fork_session_id: 'fork-source' }) } });
    await start(s); await terminal(s); assert.equal(s.out.length, expected); await s.stop();
  }
});

test('V2 original ingress age includes metadata and callback waits, with native provenance unchanged', async (t) => {
  let elapsed = 0; t.mock.method(performance, 'now', () => elapsed);
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const entered = deferred(), pause = deferred(), metadata = deferred(), reading = deferred(), spawned = []; let token, nativeTime;
  const s = service({ native: { session: async (run) => {
    reading.resolve(); await metadata.promise; return { ...run, rootSession: true };
  } }, emit: async (fact, handoff) => {
    token = handoff; nativeTime = fact.provenance.nativeTime; entered.resolve(); await pause.promise;
    if (handoff.isCurrent()) spawned.push(fact);
  } });
  await start(s); s.publish('permission.asked', permission); await reading.promise;
  elapsed = 1500; t.mock.timers.tick(1500); metadata.resolve(); await entered.promise;
  elapsed = 30001; t.mock.timers.tick(28501); await tick();
  assert.equal(nativeTime, 1790867856742); assert.equal(token.signal.aborted, true); assert.equal(token.isCurrent(), false);
  pause.resolve(); await tick(); assert.deepEqual(spawned, []); await s.stop();
});

test('a contradictory duplicate request invalidates all relevance before a paused read returns', async () => {
  const entered = deferred(), pause = deferred();
  const s = service({ checkpoints: true, native: { session: async (run) => {
    entered.resolve(); await pause.promise; return { ...run, rootSession: true };
  } } });
  await start(s); s.publish('form.created', form, 'native-form-event'); await entered.promise;
  s.publish('form.created', { ...form, callID: 'contradictory' }, 'native-form-event'); await tick();
  pause.resolve(); await tick(); assert.deepEqual(s.out, []);
  assert.ok(s.diagnostics.includes('native_identity_contradiction')); await s.stop();
});

test('native manual compaction step rejects an older ordinary assistant proof and permits the next human execution', async () => {
  for (const terminalType of ['session.execution.succeeded', 'session.execution.failed']) {
    const s = service(); await start(s);
    s.publish('session.step.ended', { messageID: 'assistant' });
    s.publish('session.step.started', { compaction: true, inputID: 'internal-control' });
    s.publish('permission.asked', permission);
    s.publish('session.step.ended', { compaction: true, messageID: 'assistant', inputID: 'internal-control' });
    s.publish(terminalType, { messageID: 'assistant' }); await tick();
    // Deliberately positive proof port for the older real assistant cannot override native control type.
    assert.deepEqual(s.out, []); assert.ok(s.diagnostics.includes('compaction_terminal_suppressed'));
    s.publish('session.execution.started', {}, 'next-human'); await terminal(s);
    assert.deepEqual(s.out.map((f) => [f.kind, f.turnID]), [['turn_idle_verified', 'user-next-human']]); await s.stop();
  }
});

test('a local tool success with providerExecuted false cannot become a permanent execution failure', async () => {
  const s = service(); await start(s);
  s.publish('session.step.ended', { messageID: 'assistant', executed: false, tool: 'subagent' });
  await terminal(s); assert.deepEqual(s.out.map((f) => f.kind), ['turn_idle_verified']); await s.stop();
});

// Regression: enforcing seq+1 disconnects a genuine sparse public compaction trace.
test('sparse public 14 to 16 compaction retains relative ordering authority', async () => {
  const s = service(); await start(s);
  s.publish('session.compaction.started', { sequence: { aggregate: 'session', seq: 14 } });
  s.publish('session.compaction.ended', { sequence: { aggregate: 'session', seq: 16 } });
  s.publish('session.step.started', { messageID: 'fresh-assistant', sequence: { aggregate: 'session', seq: 18 } });
  s.publish('session.step.ended', { messageID: 'fresh-assistant', sequence: { aggregate: 'session', seq: 20 } });
  s.publish('session.execution.succeeded', { sequence: { aggregate: 'session', seq: 22 } }); await tick();
  assert.equal(s.sources[0].signal.aborted, false);
  assert.deepEqual(s.out.map((e) => [e.kind, e.messageID]), [['turn_idle_verified', 'fresh-assistant']]);
  await s.stop();
});

// Regression: using load location for omitted event scope bypasses a native foreign/child proof.
test('omitted execution scope defers to bounded native proof; unrelated global traffic cannot kill it', async () => {
  for (const [change, expected] of [[{}, 1], [{ location: 'foreign' }, 0], [{ rootSession: false }, 0]]) {
    const s = service({ native: { correlate: (e) => ({ ...e.data, location: undefined }),
      session: async (run) => ({ ...run, rootSession: true, ...change }) } });
    await start(s);
    s.sources[0].push({ id: 'native-unrelated', created: 1790867856742, type: 'form.created',
      data: { sessionID: 'global', metadata: { kind: 'mcp' } } });
    await terminal(s); assert.equal(s.out.length, expected); assert.equal(s.sources[0].signal.aborted, false);
    await s.stop();
  }
});

// Regression: ignoring contradictory same-seq identity authorizes a paused callback.
test('exact duplicates are idempotent but a different event at the same native sequence closes that session', async () => {
  const entered = deferred(), pause = deferred();
  const s = service({ native: { session: async (run) => {
    entered.resolve(); await pause.promise; return { ...run, rootSession: true };
  } } });
  await start(s); s.publish('permission.asked', permission); await entered.promise;
  const event = s.publish('session.step.ended', { messageID: 'assistant' });
  s.sources[0].push(event); await tick(); assert.equal(s.sources[0].signal.aborted, false);
  s.sources[0].push({ ...event, id: 'conflicting-native-id' }); await tick();
  pause.resolve(); await tick(); assert.deepEqual(s.out, []);
  assert.ok(s.diagnostics.includes('native_sequence_contradiction')); await s.stop();
});

// Regression: dropping unmapped durable events fails to detect later reorder in their aggregate.
test('durable events without business mapping still advance exact native ordering', async () => {
  const s = service(); await start(s);
  s.publish('session.title.updated', { sequence: { aggregate: 'session', seq: 100 } });
  s.publish('session.step.ended', { messageID: 'assistant', sequence: { aggregate: 'session', seq: 99 } });
  s.publish('session.execution.succeeded', { sequence: { aggregate: 'session', seq: 101 } }); await tick();
  assert.deepEqual(s.out, []); assert.ok(s.diagnostics.includes('native_sequence_contradiction')); await s.stop();
});

// Regression: creating a fresh stamp after metadata hides the original ingress stall from AN.
test('explicit monotonic port preserves the original ingress coordinate through metadata and handoff', async () => {
  let localTime = 100, captured;
  const entered = deferred(), pause = deferred();
  const s = service({ clock: { id: 'TEST-qualified-coordinate', now: () => localTime },
    native: { session: async (run) => { entered.resolve(); await pause.promise; return { ...run, rootSession: true }; } },
    emit: (fact, handoff) => { captured = handoff; s.out.push(fact); } });
  await start(s); s.publish('permission.asked', permission); await entered.promise;
  localTime = 700; pause.resolve(); await tick();
  assert.equal(captured.ingressMonotonicMs, 100); assert.equal(captured.clockID, 'TEST-qualified-coordinate');
  assert.equal(s.out[0].provenance.nativeTime, 1790867856742); await s.stop();
});

// Regression: both SDK and IPC revalidation emit markers and reset the same job's budget.
test('IPC rechecks reuse the one final native snapshot without minting a second marker', async () => {
  const s = service({ checkpoints: true, emit: async (fact, handoff) => {
    assert.equal(await handoff.revalidate(), true); assert.equal(await handoff.revalidate(), true); s.out.push(fact);
  } });
  await start(s); s.publish('form.created', form); await tick();
  assert.equal(s.out.length, 1); assert.equal(s.markers.length, 2); await s.stop();
});

// Regression: ignoring a contradictory terminal while proof is paused still spawns the older candidate.
test('a contradictory execution terminal synchronously fences its paused immutable candidate', async () => {
  const entered = deferred(), pause = deferred();
  const s = service({ native: { assistant: async (run) => {
    entered.resolve(); await pause.promise;
    return { ...run, rootSession: true, role: 'assistant', summary: false, final: true, outcome: 'success' };
  } } });
  await start(s); await terminal(s); await entered.promise;
  s.publish('session.execution.failed', { messageID: 'assistant' }); await tick(); pause.resolve(); await tick();
  assert.deepEqual(s.out, []); assert.ok(s.diagnostics.includes('native_terminal_contradiction')); await s.stop();
});

// Regression: validation of irrelevant/global business fields kills a different root's subscription.
test('irrelevant global form fields and session-local malformed correlation preserve unrelated live work', async () => {
  const s = service(); await start(s);
  s.sources[0].push({ type: 'form.created', data: { sessionID: 'global' } });
  s.sources[0].push({ type: 'session.step.started', id: 'malformed-other', created: 1790867856742,
    data: { sessionID: 'unrelated', location: 'TEST-location', sequence: { aggregate: 'unrelated', seq: 1 } } });
  await terminal(s); assert.equal(s.sources[0].signal.aborted, false); assert.equal(s.out.length, 1); await s.stop();
});

// Regression: treating copied history's public seq0→11 prefix as transport loss silences a new root fork.
test('a fresh root fork can start after the sparse copied-history prefix without receiving hidden events', async () => {
  const s = service(); s.turns.set('session', 'fork-start');
  for (const [seq, type, data, eventID] of [
    [0, 'session.created', {}, 'fork-created'],
    [11, 'session.inbox.enqueued', { inboxID: 'fork-user', item: { type: 'user' } }, 'fork-input'],
    [12, 'session.execution.started', {}, 'fork-start'],
    [13, 'session.inbox.delivered', { inboxID: 'fork-user' }, 'fork-delivered'],
    [14, 'session.step.started', { messageID: 'assistant' }, 'fork-step'],
    [16, 'session.step.ended', { messageID: 'assistant' }, 'fork-end'],
    [18, 'session.execution.succeeded', {}, 'fork-terminal'],
  ]) s.sources[0].push({ id: eventID, created: 1790867856742, type,
    data: { sessionID: 'session', ...data, sequence: { aggregate: 'session', seq } } });
  await tick(); assert.equal(s.sources[0].signal.aborted, false);
  assert.deepEqual(s.out.map((e) => e.turnID), ['fork-user']); await s.stop();
});

test('consumer preparation precedes one final same-reader question checkpoint and preserves the original fact/stamps', async () => {
  const entered = deferred(), pause = deferred(); let prepared, preparation, emitted, final;
  const order = [];
  const s = service({ checkpoints: true, beforeEmit: async (fact, handoff) => {
    prepared = fact; preparation = handoff; order.push('prepare'); entered.resolve(); await pause.promise; return true;
  }, emit: async (fact, handoff) => {
    emitted = fact; final = handoff; order.push('spawn');
    assert.equal(s.markers.length, 2);
    assert.equal(await handoff.revalidate(), true); assert.equal(await handoff.revalidate(), true);
  } });
  await start(s); s.publish('form.created', form); await entered.promise;
  assert.equal(s.markers.length, 1); assert.equal(preparation.revalidate, undefined);
  pause.resolve(); await tick();
  assert.deepEqual(order, ['prepare', 'spawn']); assert.equal(prepared, emitted);
  assert.equal(preparation.ingressMonotonicMs, final.ingressMonotonicMs);
  assert.equal(preparation.clockID, final.clockID); assert.equal(preparation.metadataDeadline, final.metadataDeadline);
  assert.equal(s.markers.length, 2); await s.stop();
});

test('question close or source loss during preparation aborts its hook before any marker or spawn', async () => {
  for (const close of ['form.replied', 'end']) {
    const entered = deferred(), pause = deferred(); let preparation;
    const s = service({ checkpoints: true, beforeEmit: async (_fact, handoff) => {
      preparation = handoff; entered.resolve(); await pause.promise; return true;
    } });
    await start(s); s.publish('form.created', form); await entered.promise;
    if (close === 'end') s.sources[0].end(); else s.publish(close, { requestID: form.requestID });
    await tick(); assert.equal(preparation.signal.aborted, true); assert.equal(preparation.isCurrent(), false);
    pause.resolve(); await tick(); assert.deepEqual(s.out, []); assert.equal(s.markers.length, 1); await s.stop();
  }
});

test('permission preparation precedes exactly one pending read; settlement during preparation prevents the read', async () => {
  for (const settled of [false, true]) {
    const entered = deferred(), pause = deferred(), order = []; let captured;
    const s = service({ beforeEmit: async (_fact, handoff) => {
      captured = handoff; order.push('prepare'); entered.resolve(); await pause.promise; return true;
    }, native: { currentPermission: async (run) => { order.push('pending'); return s.pending.get(run.requestID); } },
      emit: async (_fact, handoff) => { order.push('spawn'); assert.ok(await handoff.revalidate()); } });
    await start(s); s.publish('permission.asked', permission); await entered.promise;
    if (settled) { s.publish('permission.replied', { requestID: permission.requestID }); await tick(); assert.equal(captured.signal.aborted, true); }
    pause.resolve(); await tick(); assert.deepEqual(order, settled ? ['prepare'] : ['prepare', 'pending', 'spawn']); await s.stop();
  }
});

test('question metadata, consumer preparation and same-reader marker consume the same two-second budget', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  let elapsed = 0, preparation;
  const reading = deferred(), metadata = deferred(), preparing = deferred(), pause = deferred();
  const s = service({ checkpoints: true, clock: { id: 'TEST-clock', now: () => elapsed },
    native: { session: async (run) => { reading.resolve(); await metadata.promise; return { ...run, rootSession: true }; } },
    beforeEmit: async (_fact, handoff) => { preparation = handoff; preparing.resolve(); await pause.promise; return true; } });
  await start(s); s.publish('form.created', form); await reading.promise;
  elapsed = 1700; t.mock.timers.tick(1700); metadata.resolve(); await preparing.promise;
  assert.equal(preparation.metadataDeadline, 2000); assert.equal(preparation.ingressMonotonicMs, 0);
  elapsed = 1900; t.mock.timers.tick(200); s.transport.hold = true; pause.resolve(); await tick();
  assert.equal(s.markers.length, 2);
  elapsed = 2001; t.mock.timers.tick(101); await tick();
  assert.deepEqual(s.out, []); assert.equal(s.sources[0].signal.aborted, true);
  s.consumeMarker(1); await tick(); assert.deepEqual(s.out, []); await s.stop();
});

test('a contradictory request identity fences only its native session while unrelated preparation remains live', async () => {
  const pause = deferred(), entered = [deferred(), deferred()], tokens = new Map();
  const s = service({ beforeEmit: async (fact, handoff) => {
    tokens.set(fact.sessionID, handoff); entered[fact.sessionID === 'session' ? 0 : 1].resolve();
    await pause.promise; return true;
  } });
  try {
    await start(s); s.publish('permission.asked', permission); await entered[0].promise;
    s.publish('session.execution.started', { sessionID: 'other' }, 'other-start');
    s.publish('permission.asked', { ...permission, sessionID: 'other', requestID: 'other-permission' });
    await entered[1].promise;
    s.publish('permission.asked', { ...permission, callID: 'contradictory-call' }, 'new-envelope-same-request'); await tick();
    assert.equal(tokens.get('session').signal.aborted, true);
    assert.equal(tokens.get('other').signal.aborted, false);
    assert.equal(s.sources[0].signal.aborted, false);
    pause.resolve(); await tick();
    assert.deepEqual(s.out.map((fact) => [fact.sessionID, fact.requestID]), [['other', 'other-permission']]);
    assert.ok(s.diagnostics.includes('request_identity_ambiguous'));
  } finally { pause.resolve(); await s.stop(); }
});
