import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createObserver } from '../v1.js';
const native = (type, properties) => ({ type, properties });
const user = { id: 'user', sessionID: 'session', role: 'user' };
const assistant = { id: 'assistant', sessionID: 'session', parentID: 'user', role: 'assistant',
  path: { cwd: 'TEST-location' }, time: { created: 1790867856742 } };
const final = { ...assistant, finish: 'stop', time: { ...assistant.time, completed: 1790867857073 } };
function setup(extra = {}) {
  const out = [], diagnostics = [];
  let messages = [user, assistant];
  const observer = createObserver({ runtimeEligibility: () => 'supported', callbackAuthority: 'qualified_native_sync', location: 'TEST-location',
    client: { session: { get: async () => ({ data: { id: 'session', directory: 'TEST-location' } }),
      messages: async () => ({ data: messages }) } },
    emit: (fact) => out.push(fact), onDiagnostic: (reason) => diagnostics.push(reason), ...extra });
  return { observer, out, diagnostics, setMessages: (rows) => { messages = rows; } };
}
const ask = { sessionID: 'session', id: 'question', questions: [{}], tool: { messageID: 'assistant', callID: 'call' } };
async function heldAttention(extra = {}) {
  let enter, release, captured;
  const entered = new Promise((r) => { enter = r; }), pause = new Promise((r) => { release = r; });
  const spawned = [], h = setup({ ...extra, emit: async (fact, handoff) => {
    captured = handoff; enter(); await pause; if (handoff.isCurrent()) spawned.push(fact);
  } });
  await h.observer.observe(native('message.updated', { info: user }));
  await h.observer.observe(native('message.updated', { info: assistant }));
  const work = h.observer.observe(native('question.asked', ask)); await entered;
  return { ...h, spawned, work, release, get handoff() { return captured; } };
}

test('V1 requests carry native assistant lower bound, never fictional request birth or receive time', async () => {
  const a = setup(), b = setup();
  for (const h of [a, b]) {
    await h.observer.observe(native('message.updated', { info: user }));
  await h.observer.observe(native('message.updated', { info: assistant }));
    await h.observer.observe(native('question.asked', ask));
  }
  assert.equal(a.out.length, 1);
  assert.equal(a.out[0].provenance.nativeTime, assistant.time.created);
  assert.equal(a.out[0].provenance.timeBasis, 'assistant_created_lower_bound');
  assert.equal(a.out[0].provenance.observationID, b.out[0].provenance.observationID);
  assert.equal(a.out[0].provenance.nativeEventID, undefined);
  a.observer.dispose(); b.observer.dispose();
});

test('missing callback authority, native lower bound, scope or runtime cannot authorize strict IPC', async () => {
  for (const change of ['authority', 'time', 'scope', 'runtime']) {
    const h = setup(change === 'runtime' ? { runtimeEligibility: () => 'unverified' } : change === 'authority' ? { callbackAuthority: undefined } : {});
    h.setMessages([user, { ...assistant, ...(change === 'time' ? { time: {} } : {}),
      ...(change === 'scope' ? { path: { cwd: 'other-project' } } : {}) }]);
    await h.observer.observe(native('message.updated', { info: user }));
  await h.observer.observe(native('message.updated', { info: assistant }));
    await h.observer.observe(native('question.asked', ask));
    assert.deepEqual(h.out, [], change); h.observer.dispose();
  }
});

test('source error and idle alone cannot select a later assistant as terminal evidence', async () => {
  const h = setup(); await h.observer.observe(native('message.updated', { info: user }));
  await h.observer.observe(native('session.error', { sessionID: 'session', error: { name: 'APIError' } }));
  await h.observer.observe(native('session.idle', { sessionID: 'session' }));
  assert.deepEqual(h.out, []); h.observer.dispose();
});

test('native error-before-idle-before-final preserves final matching error, with no early handoff', async () => {
  const h = setup(), failed = { ...final, error: { name: 'APIError' } };
  await h.observer.observe(native('message.updated', { info: user }));
  await h.observer.observe(native('message.updated', { info: assistant }));
  await h.observer.observe(native('message.updated', { info: assistant }));
  await h.observer.observe(native('session.error', { sessionID: 'session', error: { name: 'APIError' } }));
  await h.observer.observe(native('session.idle', { sessionID: 'session' })); assert.deepEqual(h.out, []);
  h.setMessages([user, failed]); await h.observer.observe(native('message.updated', { info: failed }));
  assert.deepEqual(h.out.map((f) => f.kind), ['terminal_error']);
  assert.equal(h.out[0].provenance.nativeTime, assistant.time.created);
  assert.equal(h.out[0].provenance.nativeMessageID, failed.id); h.observer.dispose();
});

test('ambiguous active assistants and compaction suppress source error correlation', async () => {
  for (const compaction of [false, true]) {
    const h = setup(), failed = { ...final, error: { name: 'APIError' } };
    await h.observer.observe(native('message.updated', { info: user }));
  await h.observer.observe(native('message.updated', { info: assistant }));
    await h.observer.observe(native('message.updated', { info: assistant }));
    if (compaction) await h.observer.observe(native('session.compacted', { sessionID: 'session' }));
    else await h.observer.observe(native('message.updated', { info: { ...assistant, id: 'competing' } }));
    await h.observer.observe(native('session.error', { sessionID: 'session', error: { name: 'APIError' } }));
    h.setMessages([user, failed]); await h.observer.observe(native('message.updated', { info: failed }));
    await h.observer.observe(native('session.idle', { sessionID: 'session' }));
    assert.deepEqual(h.out, []); h.observer.dispose();
  }
});

test('V1 settlement synchronously aborts an already paused IPC adapter', async () => {
  const h = await heldAttention();
  const settled = h.observer.observe(native('question.replied', { sessionID: 'session', requestID: 'question' }));
  assert.equal(h.handoff.signal.aborted, true); assert.equal(h.handoff.isCurrent(), false);
  h.release(); await h.work; await settled; assert.deepEqual(h.spawned, []); h.observer.dispose();
});

test('strict V1 session saturation protects a paused live fence instead of evicting it', async () => {
  const h = await heldAttention({ dedupLimit: 1 });
  await h.observer.observe(native('message.updated', { info: { ...user, id: 'new-user', sessionID: 'other' } }));
  assert.ok(h.diagnostics.includes('session_capacity'));
  h.release(); await h.work; assert.equal(h.spawned.length, 1); h.observer.dispose();
});

test('V1 cannot use an older assistant as provenance for attention from a replaced step', async () => {
  const h = setup();
  h.setMessages([user, assistant, { ...assistant, id: 'new-step' }]);
  await h.observer.observe(native('message.updated', { info: user }));
  await h.observer.observe(native('message.updated', { info: assistant }));
  await h.observer.observe(native('question.asked', ask)); assert.deepEqual(h.out, []); h.observer.dispose();
});

test('30s V1 ingress budget includes paused metadata and callback waits without restamping', async (t) => {
  let elapsed = 0; t.mock.method(performance, 'now', () => elapsed);
  t.mock.timers.enable({ apis: ['setTimeout'] });
  let enter, release;
  const entered = new Promise((r) => { enter = r; }), pause = new Promise((r) => { release = r; });
  const pending = heldAttention({ client: { session: {
    get: async () => ({ data: { id: 'session', directory: 'TEST-location' } }),
    messages: async () => { enter(); await pause; return { data: [user, assistant] }; },
  } } });
  await entered; elapsed = 1500; t.mock.timers.tick(1500); release();
  const h = await pending;
  elapsed = 30001; t.mock.timers.tick(28501);
  assert.equal(h.handoff.signal.aborted, true); assert.equal(h.handoff.isCurrent(), false);
  h.release(); await h.work; assert.deepEqual(h.spawned, []); h.observer.dispose();
});

test('native compaction summary is never ordinary V1 assistant completion', async () => {
  const summary = true, h = setup(); h.setMessages([user, { ...final, summary }]);
  await h.observer.observe(native('message.updated', { info: user }));
  await h.observer.observe(native('message.updated', { info: assistant }));
  await h.observer.observe(native('message.updated', { info: { ...final, summary } }));
  await h.observer.observe(native('session.idle', { sessionID: 'session' }));
  assert.deepEqual(h.out, []); h.observer.dispose();
});

test('uncorrelated native error and compaction summary synchronously invalidate paused V1 attention', async () => {
  for (const closing of [native('session.error', { sessionID: 'session', error: { name: 'APIError' } }),
    native('message.updated', { info: { ...assistant, id: 'summary', summary: true } })]) {
    const h = await heldAttention();
    const closed = h.observer.observe(closing);
    assert.equal(h.handoff.signal.aborted, true); assert.equal(h.handoff.isCurrent(), false);
    h.release(); await h.work; await closed; assert.deepEqual(h.spawned, []); h.observer.dispose();
  }
});

// Regression: the compacted callback arrives after the native continuation user and leaves a sticky silence flag.
test('auto summary then continuation user then compacted accepts only a fresh current ordinary assistant', async () => {
  const h = setup();
  await h.observer.observe(native('message.updated', { info: user }));
  const summary = { ...final, id: 'summary', summary: true };
  await h.observer.observe(native('message.updated', { info: summary }));
  const continuation = { ...user, id: 'continuation' };
  await h.observer.observe(native('message.updated', { info: continuation }));
  await h.observer.observe(native('session.compacted', { sessionID: 'session' }));
  // Old summary republication must not qualify resumed work or poison a later ordinary assistant.
  await h.observer.observe(native('message.updated', { info: summary }));
  const resumed = { ...final, id: 'resumed', parentID: 'continuation',
    time: { created: final.time.completed + 1, completed: final.time.completed + 2 } };
  h.setMessages([continuation, resumed]);
  await h.observer.observe(native('message.updated', { info: resumed }));
  await h.observer.observe(native('session.idle', { sessionID: 'session' }));
  assert.deepEqual(h.out.map((e) => [e.kind, e.turnID]), [['turn_idle_verified', 'continuation']]); h.observer.dispose();
});

// Regression: independently renewing 2s for session metadata accepts a 3s attention job.
test('V1 messages and ancestry share the entire 2s metadata deadline', async (t) => {
  let elapsed = 0, releaseMessages, enteredGet, captured;
  t.mock.method(performance, 'now', () => elapsed); t.mock.timers.enable({ apis: ['setTimeout'] });
  const messages = new Promise((r) => { releaseMessages = r; });
  const getStarted = new Promise((r) => { enteredGet = r; });
  const h = setup({ client: { session: {
    messages: () => messages,
    get: async ({ signal }) => { captured = signal; enteredGet(); return new Promise(() => {}); },
  } } });
  await h.observer.observe(native('message.updated', { info: user }));
  await h.observer.observe(native('message.updated', { info: assistant }));
  const job = h.observer.observe(native('question.asked', ask));
  elapsed = 1500; t.mock.timers.tick(1500); releaseMessages({ data: [user, assistant] }); await getStarted;
  elapsed = 2000; t.mock.timers.tick(500); await job;
  assert.equal(captured.aborted, true); assert.deepEqual(h.out, []); h.observer.dispose();
});

// Regression: hydration of an old assistant after Asked manufactures a lower bound never seen at ingress.
test('strict V1 attention requires its native assistant before Asked and preserves ingress stamp', async () => {
  const h = setup(); await h.observer.observe(native('message.updated', { info: user }));
  await h.observer.observe(native('question.asked', ask)); assert.deepEqual(h.out, []); h.observer.dispose();
  let localTime = 10, handoff;
  const current = setup({ clock: { id: 'TEST-V1-coordinate', now: () => localTime },
    emit: (fact, token) => { handoff = token; current.out.push(fact); }, client: { session: {
      get: async () => ({ id: 'session', directory: 'TEST-location' }),
      messages: async () => { localTime = 100; return [user, assistant]; },
    } } });
  await current.observer.observe(native('message.updated', { info: user }));
  await current.observer.observe(native('message.updated', { info: assistant }));
  await current.observer.observe(native('question.asked', ask));
  assert.equal(handoff.ingressMonotonicMs, 10); assert.equal(handoff.clockID, 'TEST-V1-coordinate'); current.observer.dispose();
});

// Regression: a later callback during paused hydration backfills source provenance for an earlier Asked.
test('assistant first observed during paused request metadata cannot retroactively authorize that request', async () => {
  let entered, release;
  const started = new Promise((r) => { entered = r; }), pause = new Promise((r) => { release = r; });
  const h = setup({ client: { session: {
    messages: async () => { entered(); await pause; return [user, assistant]; },
    get: async () => ({ id: 'session', directory: 'TEST-location' }),
  } } });
  await h.observer.observe(native('message.updated', { info: user }));
  const job = h.observer.observe(native('question.asked', ask)); await started;
  await h.observer.observe(native('message.updated', { info: assistant })); release(); await job;
  assert.deepEqual(h.out, []); h.observer.dispose();
});

test('V1 preparation carries the original native-read deadline and close aborts it before business delivery', async () => {
  let elapsed = 0, enter, release, prep;
  const entered = new Promise((r) => { enter = r; }), pause = new Promise((r) => { release = r; });
  const h = setup({ clock: { id: 'TEST-clock', now: () => elapsed },
    client: { session: { get: async () => ({ id: 'session', directory: 'TEST-location' }),
      messages: async () => { elapsed = 1200; return [user, assistant]; } } },
    beforeEmit: async (_fact, handoff) => { prep = handoff; enter(); await pause; return true; } });
  await h.observer.observe(native('message.updated', { info: user }));
  await h.observer.observe(native('message.updated', { info: assistant }));
  const job = h.observer.observe(native('question.asked', ask)); await entered;
  assert.equal(prep.metadataDeadline, 2000); assert.equal(prep.ingressMonotonicMs, 0); assert.equal(prep.revalidate, undefined);
  await h.observer.observe(native('question.replied', { sessionID: 'session', requestID: 'question' }));
  assert.equal(prep.signal.aborted, true); release(); await job; assert.deepEqual(h.out, []); h.observer.dispose();
});

test('V1 native metadata and an abort-ignoring preparation jointly retain the sixteen actual-settlement slots', async () => {
  let enter, releasePrep, releaseReads, reads = 0, prep;
  const entered = new Promise((r) => { enter = r; }), preparation = new Promise((r) => { releasePrep = r; }),
    metadata = new Promise((r) => { releaseReads = r; });
  const h = setup({ beforeEmit: async (_fact, handoff) => { prep = handoff; enter(); await preparation; return true; },
    client: { session: { get: async ({ path }) => ({ id: path.id, directory: 'TEST-location' }),
      messages: async ({ path }) => {
        if (path.id === 'session') return [user, assistant];
        reads++; await metadata; return [];
      } } } });
  await h.observer.observe(native('message.updated', { info: user }));
  await h.observer.observe(native('message.updated', { info: assistant }));
  const first = h.observer.observe(native('question.asked', ask)); await entered;
  await h.observer.observe(native('question.replied', { sessionID: 'session', requestID: 'question' }));
  assert.equal(prep.signal.aborted, true);
  const jobs = [];
  for (let i = 0; i < 16; i++) {
    const sessionID = `other-${i}`;
    await h.observer.observe(native('message.updated', { info: { ...user, sessionID } }));
    await h.observer.observe(native('message.updated', { info: { ...assistant, sessionID } }));
    jobs.push(h.observer.observe(native('question.asked', { ...ask, sessionID })));
  }
  await new Promise(setImmediate); assert.equal(reads, 15);
  assert.ok(h.diagnostics.includes('messages lookup capacity exceeded'));
  releasePrep(); releaseReads(); await Promise.all([first, ...jobs]); assert.deepEqual(h.out, []); h.observer.dispose();
});

test('strict V1 overflow takes typed identity from the verified final native answer even without its final callback', async () => {
  const observations = [];
  for (const messageID of ['overflow-a', 'overflow-b']) {
    const h = setup(), active = { ...assistant, id: messageID },
      failed = { ...final, id: messageID, error: { name: 'ContextOverflowError' } };
    await h.observer.observe(native('message.updated', { info: user }));
    await h.observer.observe(native('message.updated', { info: active }));
    h.setMessages([user, failed]);
    await h.observer.observe(native('session.error', { sessionID: 'session', error: { name: 'ContextOverflowError' } }));
    await h.observer.observe(native('session.idle', { sessionID: 'session' }));
    assert.equal(h.out.length, 1); assert.equal(h.out[0].kind, 'terminal_error');
    assert.equal(h.out[0].provenance.nativeMessageID, messageID);
    observations.push(h.out[0].provenance.observationID); h.observer.dispose();
  }
  assert.notEqual(observations[0], observations[1]);
});

// Regression: exhausted business-job capacity drops native scope/delete controls
// whose session identity is properties.info.id, leaving preparation authorized.
for (const type of ['session.deleted', 'session.updated', 'session.error']) {
  test(`strict V1 ${type} aborts preparation synchronously with all 256 ingress jobs held`, async () => {
    let enter, release, preparation;
    const entered = new Promise((r) => { enter = r; }), pause = new Promise((r) => { release = r; });
    const h = setup({ beforeEmit: async (_fact, handoff) => {
      preparation = handoff; enter(); await pause; return true;
    } });
    let job; const fillers = [];
    try {
      await h.observer.observe(native('message.updated', { info: user }));
      await h.observer.observe(native('message.updated', { info: assistant }));
      job = h.observer.observe(native('question.asked', ask)); await entered;
      // These admitted calls remain counted until their promises settle. No await
      // lets their finally callbacks release capacity before the native control.
      for (let i = 0; i < 255; i++) fillers.push(h.observer.observe(native('session.idle', { sessionID: `capacity-${i}` })));
      assert.equal(h.diagnostics.includes('job_capacity'), false);
      const closed = h.observer.observe(native(type, type === 'session.error'
        ? { sessionID: 'session', error: { name: 'APIError' } }
        : { info: { id: 'session', directory: 'other-location' } }));
      assert.equal(preparation.signal.aborted, true);
      assert.equal(preparation.isCurrent(), false);
      release(); await Promise.all([job, closed, ...fillers]);
      assert.deepEqual(h.out, []); // Final emit represents the business spawn/delivery boundary.
      assert.equal(h.diagnostics.includes('job_capacity'), type === 'session.error');
    } finally { release(); await Promise.all([job, ...fillers]); h.observer.dispose(); }
  });
}

test('strict V1 saturated scope ingress preserves unrelated roots and unknown/global isolation', async () => {
  let enter, release, preparation;
  const entered = new Promise((r) => { enter = r; }), pause = new Promise((r) => { release = r; });
  const h = setup({ beforeEmit: async (_fact, handoff) => {
    preparation = handoff; enter(); await pause; return true;
  } });
  let job; const work = [];
  try {
    await h.observer.observe(native('message.updated', { info: user }));
    await h.observer.observe(native('message.updated', { info: assistant }));
    job = h.observer.observe(native('question.asked', ask)); await entered;
    for (let i = 0; i < 255; i++) work.push(h.observer.observe(native('session.idle', { sessionID: `capacity-${i}` })));
    for (const event of [native('session.deleted', { info: { id: 'other-root' } }),
      native('session.updated', { info: { id: 42 } }),
      native('global.updated', { info: { id: 'session' } }), native('global.updated', {})]) {
      work.push(h.observer.observe(event));
      assert.equal(preparation.signal.aborted, false);
      assert.equal(preparation.isCurrent(), true);
    }
    release(); await Promise.all([job, ...work]);
    assert.equal(h.out.length, 1); assert.equal(h.out[0].sessionID, 'session');
  } finally { release(); await Promise.all([job, ...work]); h.observer.dispose(); }
});
