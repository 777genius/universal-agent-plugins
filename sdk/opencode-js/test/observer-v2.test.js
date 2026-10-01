import test from 'node:test';
import assert from 'node:assert/strict';
import { createV2Observer } from '../index.js';

const own = { directory: '/TEST-observer', workspaceID: 'workspace-a' };
const tick = async () => { for (let i = 0; i < 12; i++) await new Promise(setImmediate); };
const deferred = () => { let resolve; const promise = new Promise((r) => { resolve = r; }); return { promise, resolve }; };
function harness(options = {}) {
  const facts = [], diagnostics = [], calls = { get: 0, context: 0 };
  let rows = [{ id: 'user', type: 'user' }, { id: 'assistant', type: 'assistant', finish: 'stop', time: { completed: 1 } }, { id: 'idle', type: 'idle' }];
  const observer = createV2Observer({
    location: own,
    client: {
      get: async (input) => { calls.get++; return options.get ? options.get(input) : { id: input.sessionID, location: own }; },
      context: async (input) => { calls.context++; return options.context ? options.context(input) : rows; },
    },
    emit: (fact) => { facts.push(fact); return options.emit?.(fact); },
    onDiagnostic: (code) => diagnostics.push(code), ...options.config,
  });
  const event = (type, data = {}, extra = {}) => observer.observe({ type, data: { sessionID: 'session', ...data }, ...extra });
  const begin = (uid = 'user') => {
    event('session.inbox.enqueued', { inboxID: uid, item: { type: 'user', delivery: 'steer' } });
    event('session.execution.started'); event('session.inbox.delivered', { inboxID: uid });
    event('session.step.started', { assistantMessageID: 'assistant' });
  };
  const finish = (assistantMessageID = 'assistant') => { event('session.step.ended', { assistantMessageID, finish: 'stop' }); event('session.execution.succeeded'); };
  const question = (rid = 'question') => event('form.created', { form: { id: rid, sessionID: 'session', fields: [{ type: 'string', key: 'answer' }], metadata: { kind: 'question', tool: { messageID: 'assistant', id: 'tool' } } } });
  return { observer, facts, diagnostics, calls, event, begin, finish, question, rows: (value) => { rows = value; } };
}

// Regression: awaiting initial ancestry loses earlier inbox association or replays a resolved form.
test('first-get coalesces and settles current ordered metadata only', async () => {
  const get = deferred(), h = harness({ get: () => get.promise });
  h.begin(); h.question(); h.event('form.replied', { id: 'question' }); h.finish();
  await tick(); assert.equal(h.calls.get, 1); assert.equal(h.facts.length, 0);
  get.resolve({ id: 'session', location: own }); await tick();
  assert.deepEqual(h.facts, [{ version: 1, sessionID: 'session', turnID: 'user', rootSession: true, kind: 'turn_idle_verified', messageID: 'assistant' }]);
  h.observer.dispose();
});

// Regression: step settlement is mistaken for whole busy-period settlement or queued user is ignored.
test('queued delivery selects final user and completion waits for terminal success', async () => {
  const h = harness(); h.begin();
  h.event('session.inbox.enqueued', { inboxID: 'user2', item: { type: 'user', delivery: 'queue' } });
  h.event('session.step.ended', { assistantMessageID: 'assistant', finish: 'stop' });
  await tick(); assert.equal(h.facts.length, 0);
  h.event('session.inbox.delivered', { inboxID: 'user2' });
  h.event('session.step.started', { assistantMessageID: 'assistant' });
  h.rows([{ id: 'user2', type: 'user' }, { id: 'assistant', type: 'assistant', finish: 'stop', time: { completed: 2 } }, { id: 'idle', type: 'idle' }]);
  h.finish(); await tick(); assert.equal(h.facts.length, 1); assert.equal(h.facts[0].turnID, 'user2');
  h.observer.dispose();
});

// Regression: stale context results emit after successor work, deletion, move or disposal.
for (const action of ['move', 'delete', 'dispose', 'successor', 'reply']) {
  test(`late request context is fenced by ${action}`, async () => {
    const context = deferred(), h = harness({ context: () => context.promise });
    h.begin(); await tick(); h.question(); await tick(); assert.equal(h.calls.context, 1);
    if (action === 'move') h.event('session.moved', {}, { location: { directory: '/TEST-foreign' } });
    if (action === 'delete') h.event('session.deleted');
    if (action === 'dispose') { h.observer.dispose(); h.observer.dispose(); }
    if (action === 'successor') h.begin('user2');
    if (action === 'reply') h.event('form.cancelled', { id: 'question' });
    context.resolve([{ id: 'user', type: 'user' }, { id: 'assistant', type: 'assistant' }]);
    await tick(); assert.equal(h.facts.length, 0); h.observer.dispose();
  });
}

// Regression: abort releases capacity while native host promises remain alive.
test('soft timeout retains actual concurrency slots and dispose returns immediately', async () => {
  const calls = [], h = harness({ get: () => { const d = deferred(); calls.push(d); return d.promise; }, config: { maxConcurrentLookups: 2, lookupTimeoutMs: 100 } });
  for (let i = 0; i < 4; i++) h.event('session.created', { sessionID: `session${i}` });
  await tick(); assert.equal(calls.length, 2);
  await new Promise((r) => setTimeout(r, 130));
  h.event('session.created', { sessionID: 'session4' }); await tick(); assert.equal(calls.length, 2);
  h.observer.dispose(); calls.forEach((d) => d.resolve({})); await tick(); assert.equal(h.facts.length, 0);
});

// Regression: child/foreign ownership or mismatching context becomes actionable.
for (const [name, config] of [
  ['child', { get: async () => ({ id: 'session', parentID: 'parent', location: own }) }],
  ['foreign workspace', { get: async () => ({ id: 'session', location: { ...own, workspaceID: 'workspace-b' } }) }],
  ['malformed ancestry', { get: async () => ({ id: 'session', parentID: null, location: own }) }],
  ['missing ownership', { get: async () => ({ id: 'session' }) }],
  ['context mismatch', { context: async () => [{ id: 'other-user', type: 'user' }, { id: 'assistant', type: 'assistant', finish: 'stop', time: { completed: 1 } }] }],
  ['later continuation', { context: async () => [{ id: 'user', type: 'user' }, { id: 'assistant', type: 'assistant', finish: 'stop', time: { completed: 1 } }, { id: 'next', type: 'assistant' }] }],
]) test(`completion fails closed: ${name}`, async () => {
  const h = harness(config); h.begin(); h.finish(); await tick(); assert.equal(h.facts.length, 0); h.observer.dispose();
});

// Regression: native step failure/retry/interruption are reported as terminal outcome.
test('retry requires a later final step; interrupted work never emits failure', async () => {
  const h = harness(); h.begin(); h.event('session.retry.scheduled', { assistantMessageID: 'assistant' });
  h.event('session.execution.succeeded'); await tick(); assert.equal(h.facts.length, 0);
  h.event('session.step.started', { assistantMessageID: 'assistant' }); h.finish(); await tick();
  assert.equal(h.facts[0].kind, 'turn_idle_verified');
  h.begin('user2'); h.event('session.step.failed', { assistantMessageID: 'assistant' });
  h.event('session.execution.interrupted', { reason: 'user' }); h.event('session.execution.failed');
  await tick(); assert.equal(h.facts.length, 1); h.observer.dispose();
});

// Regression: terminal failure before assistant creation is dropped or re-emitted after success.
test('native terminal failure is one mutually exclusive terminal fact', async () => {
  const h = harness(); h.begin(); h.event('session.execution.failed', { error: { name: 'ProviderError', message: 'PRIVATE' } });
  await tick(); h.finish(); await tick(); assert.equal(h.facts.length, 1); assert.equal(h.facts[0].kind, 'terminal_error');
  assert.equal(JSON.stringify(h.facts).includes('PRIVATE'), false); h.observer.dispose();
});

// Regression: retained compaction continuity is confused with historical context hydration.
test('missing user requires observed compaction continuity and fresh final step', async () => {
  const h = harness(); h.begin(); h.rows([{ id: 'assistant', type: 'assistant', finish: 'stop', time: { completed: 1 } }]);
  h.finish(); await tick(); assert.equal(h.facts.length, 0);
  h.event('session.compaction.started'); h.event('session.compaction.ended');
  h.rows([{ id: 'assistant2', type: 'assistant', finish: 'stop', time: { completed: 2 } }]);
  h.event('session.step.started', { assistantMessageID: 'assistant2' }); h.finish('assistant2'); await tick();
  assert.equal(h.facts.length, 1); h.observer.dispose();
});

// Regression: generic/auth form becomes a question or callback failure triggers delivery replay.
test('four content-free facts and no request retry after callback rejection', async () => {
  const h = harness({ emit: async () => { throw new Error('PRIVATE_CALLBACK'); } }); h.begin(); await tick();
  h.event('form.created', { form: { id: 'auth', sessionID: 'session', metadata: { kind: 'auth' } } });
  h.question(); await tick(); h.question(); await tick();
  assert.equal(h.facts.length, 1); assert.equal(h.facts[0].kind, 'question_asked');
  h.event('form.replied', { id: 'question', answer: 'PRIVATE_ANSWER' });
  h.event('permission.asked', { id: 'permission', action: 'PRIVATE_ACTION', resources: ['PRIVATE_RESOURCE'], source: { type: 'tool', messageID: 'assistant', id: 'tool' } });
  await tick(); assert.equal(h.facts.length, 2); assert.equal(h.facts[1].kind, 'permission_asked');
  h.event('permission.replied', { requestID: 'permission' });
  h.event('session.step.started', { assistantMessageID: 'assistant' }); h.finish(); await tick();
  assert.equal(h.facts.length, 3); assert.equal(h.facts[2].kind, 'turn_idle_verified');
  assert.equal(JSON.stringify([h.facts, h.diagnostics]).includes('PRIVATE'), false); h.observer.dispose();
});

// Regression: the initial ownership token accidentally includes semantic revision.
test('foreign move invalidates the unresolved first-get identity token', async () => {
  const get = deferred(), h = harness({ get: () => get.promise }); h.begin(); h.finish();
  h.event('session.moved', {}, { location: { directory: '/TEST-foreign' } });
  get.resolve({ id: 'session', location: own }); await tick(); assert.equal(h.facts.length, 0); h.observer.dispose();
});

// Regression: a synthetic shape passes while actual native metadata uses different fields/control rows.
for (const version of ['2.0.0', '2.0.21']) test(`retained sanitized native ${version} success contract`, async () => {
  const { readFile } = await import('node:fs/promises');
  const fixture = JSON.parse(await readFile(new URL(`./fixtures/v2-success-${version}.json`, import.meta.url), 'utf8'));
  const h = harness({ context: async () => fixture.context });
  for (const event of fixture.events) h.observer.observe(event);
  await tick();
  assert.deepEqual(h.facts, [{ version: 1, sessionID: 'session', turnID: 'user', rootSession: true, kind: 'turn_idle_verified', messageID: 'assistant' }]);
  h.observer.dispose();
});

// Regression: rejecting a saturated candidate silently schedules a later retry on duplicate terminal input.
test('failed semantic snapshot is not retried by repeated terminal input', async () => {
  const h = harness({ context: async () => undefined }); h.begin(); h.finish(); await tick();
  h.event('session.execution.succeeded'); h.event('session.execution.succeeded'); await tick();
  assert.equal(h.calls.context, 1); assert.equal(h.facts.length, 0); h.observer.dispose();
});

// Regression: per-candidate context calls fan out when ingress continues during verification.
test('one outstanding snapshot per candidate coalesces semantic revisions', async () => {
  const snapshot = deferred(), h = harness({ context: () => snapshot.promise });
  h.begin(); await tick(); h.question(); await tick();
  for (let i = 0; i < 20; i++) h.event('session.inbox.delivery.changed', { inboxID: 'missing', delivery: 'queue' });
  await tick(); assert.equal(h.calls.context, 1);
  h.event('form.replied', { id: 'question' }); snapshot.resolve([]); await tick();
  assert.equal(h.facts.length, 0); h.observer.dispose();
});

// Regression: disposal before the scheduled Promise callback still starts host calls.
test('immediate disposal does not start deferred lookup work', async () => {
  const h = harness(); h.begin(); h.observer.dispose(); await tick();
  assert.equal(h.calls.get, 0); assert.equal(h.calls.context, 0); assert.equal(h.facts.length, 0);
});

// Regression: metadata overflow discards unresolved requests and falsely allows a final stop.
test('pending request overflow suppresses completion without discarding unresolved state', async () => {
  const h = harness(); h.begin();
  for (let i = 0; i < 65; i++) h.question(`question${i}`);
  for (let i = 0; i < 65; i++) h.event('form.replied', { id: `question${i}` });
  h.finish(); await tick(); assert.equal(h.facts.length, 0); assert.ok(h.diagnostics.includes('metadata_capacity'));
  h.observer.dispose();
});

// Regression: a full context is projected or its mismatched IDs accepted outside the bounded contract.
for (const reason of ['excess rows', 'duplicate ID']) test(`invalid context fails closed: ${reason}`, async () => {
  const rows = reason === 'excess rows' ? Array.from({ length: 4097 }, (_, i) => ({ id: `row${i}`, type: 'idle' })) : [{ id: 'user', type: 'user' }, { id: 'user', type: 'assistant' }];
  const h = harness({ context: async () => rows }); h.begin(); h.finish(); await tick();
  assert.equal(h.facts.length, 0); assert.ok(h.diagnostics.includes('context_unverified')); h.observer.dispose();
});

// Regression: two global readers use directory alone and both notify the same workspace session.
test('global readers route to exactly one directory and workspace owner', async () => {
  const facts = [], client = { get: async () => ({ id: 'session', location: own }),
    context: async () => [{ id: 'user', type: 'user' }, { id: 'assistant', type: 'assistant', finish: 'stop', time: { completed: 1 } }] };
  const a = createV2Observer({ client, location: own, emit: (fact) => facts.push(fact) });
  const b = createV2Observer({ client, location: { ...own, workspaceID: 'workspace-b' }, emit: (fact) => facts.push(fact) });
  const events = [
    { type: 'session.inbox.enqueued', data: { sessionID: 'session', inboxID: 'user', item: { type: 'user' } } },
    { type: 'session.execution.started', data: { sessionID: 'session' } },
    { type: 'session.inbox.delivered', data: { sessionID: 'session', inboxID: 'user' } },
    { type: 'session.step.started', data: { sessionID: 'session', assistantMessageID: 'assistant' } },
    { type: 'session.step.ended', data: { sessionID: 'session', assistantMessageID: 'assistant', finish: 'stop' } },
    { type: 'session.execution.succeeded', data: { sessionID: 'session' } },
  ];
  for (const event of events) { a.observe(event); b.observe(event); }
  await tick(); assert.equal(facts.length, 1); a.dispose(); b.dispose();
});
