import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createObserver } from '../index.js';

const fixture = JSON.parse(readFileSync(new URL('../fixtures/wire-v1.json', import.meta.url)));
const native = (type, properties) => ({ type, properties });
const user = (id = 'u1', sessionID = 's1') => ({ id, sessionID, role: 'user' });
const answer = (parentID = 'u1', sessionID = 's1') => ({ id: 'a1', sessionID, role: 'assistant', parentID, finish: 'stop', time: { completed: 1 } });
function harness() {
  const out = [], diagnostics = [];
  let messages = [user(), answer()];
  let lookups = 0;
  const observer = createObserver({ emit: (x) => out.push(x), onDiagnostic: (x) => diagnostics.push(x), client: { session: { get: async ({path}) => ({data:{id:path.id}}), messages: async ({path, query}) => {
    assert.equal(path.id, 's1'); assert.ok(query.limit <= 100); lookups++; return {data: messages};
  } } } });
  return {out, diagnostics, observer, setMessages: (x) => { messages = x; }, lookups: () => lookups};
}

test('contract fixtures are produced by native event sequence', async () => {
  const h = harness(), o = h.observer.observe;
  await o(native('session.idle', {sessionID:'s1'})); // initial idle
  await o(native('message.updated', {info:user()}));
  await o(native('question.asked', {sessionID:'s1', id:'q1', messageID:'a1', questions:[{header:'x', question:'secret'}]}));
  await o(native('question.replied', {sessionID:'s1', requestID:'q1'}));
  await o(native('permission.asked', {sessionID:'s1', id:'p1', messageID:'a1', permission:'read', patterns:['secret']}));
  await o(native('permission.replied', {sessionID:'s1', requestID:'p1', reply:'once'}));
  await o(native('message.updated', {info:answer()}));
  await o(native('session.idle', {sessionID:'s1'}));
  await o(native('session.idle', {sessionID:'s1'}));
  await o(native('session.error', {sessionID:'s1', error:{name:'Error'}})); // completed turn is not an error
  await o(native('message.updated', {info:user('u2')}));
  h.setMessages([user('u2')]);
  await o(native('session.error', {sessionID:'s1', error:{name:'Error', data:{message:'secret'}}}));
  await o(native('future.event', {sessionID:'s1', text:'secret'}));
  assert.deepEqual(h.out, [fixture[1], fixture[2], fixture[0], fixture[3], fixture[4]]);
  assert.equal(h.lookups(), 4);
  assert.ok(!JSON.stringify(h.out).includes('secret'));
});

test('stale output, retry, pending request and mismatched IDs suppress completion', async () => {
  for (const [events, messages] of [
    [[native('session.idle',{sessionID:'s1'})], [user(), answer('older')]],
    [[native('session.status',{sessionID:'s1',status:{type:'retry'}}), native('session.idle',{sessionID:'s1'})], [user(), answer()]],
    [[native('question.asked',{sessionID:'s1',id:'q1',questions:[{}]}), native('session.idle',{sessionID:'s1'})], [user(), answer()]],
    [[native('session.idle',{sessionID:'s1'})], [user(), answer('u1','other')]],
    [[native('session.idle',{sessionID:'s1'})], [user('u2'), answer()]],
  ]) {
    const h = harness(); h.setMessages(messages);
    await h.observer.observe(native('message.updated',{info:user()}));
    await h.observer.observe(native('message.updated',{info:answer()}));
    for (const e of events) await h.observer.observe(e);
    assert.ok(!h.out.some((x) => x.kind === 'turn_idle_verified'));
  }
});

test('malformed requests do not invent IDs and different turns are distinct', async () => {
  const h = harness(), o = h.observer.observe;
  await o(native('message.updated',{info:user()}));
  await o(native('message.updated',{info:answer()}));
  await o(native('permission.asked',{sessionID:'other',id:'p1',messageID:'u1'}));
  await o(native('question.asked',{sessionID:'s1',id:23}));
  await o(native('session.idle',{sessionID:'s1'}));
  await o(native('message.updated',{info:user('u2')}));
  h.setMessages([user('u2'), {...answer('u2'), id:'a2'}]);
  await o(native('message.updated',{info:{...answer('u2'), id:'a2'}}));
  await o(native('session.idle',{sessionID:'s1'}));
  assert.deepEqual(h.out.map((x) => x.turnID), ['u1','u2']);
  assert.ok(h.diagnostics.length >= 2);
});

test('IDs with JSON control characters are rejected before emission', async () => {
  const h = harness(), o = h.observer.observe;
  await o(native('message.updated', { info: user() }));
  await o(native('question.asked', { sessionID: 's1', id: '\0'.repeat(256), questions: [{}] }));
  await o(native('message.updated', { info: { ...answer(), id: 'a\n1' } }));
  await o(native('session.idle', { sessionID: 's1' }));
  assert.deepEqual(h.out, []);
  assert.ok(h.diagnostics.includes('invalid request'));
  assert.ok(h.diagnostics.includes('invalid message.updated'));
});

test('cancel, unresolved permission, and resolution during lookup cannot yield stale facts', async () => {
  const h = harness(), o = h.observer.observe;
  await o(native('message.updated',{info:user()}));
  await o(native('message.updated',{info:answer()}));
  await o(native('permission.asked',{sessionID:'s1',id:'p1',permission:'read',tool:{messageID:'a1'}}));
  await o(native('session.idle',{sessionID:'s1'}));
  assert.deepEqual(h.out.map((x) => x.kind), ['permission_asked']);
  await o(native('permission.replied',{sessionID:'s1',requestID:'p1'}));
  await o(native('session.error',{sessionID:'s1',error:{name:'AbortError'}}));
  await o(native('session.idle',{sessionID:'s1'}));
  assert.deepEqual(h.out.map((x) => x.kind), ['permission_asked']);
});

test('bounded lookup rejects malformed responses and unknown events carry no body', async () => {
  const h = harness(), o = h.observer.observe;
  await o(native('message.updated',{info:user()}));
  await o(native('message.updated',{info:answer()}));
  h.setMessages([{...user(), sessionID:'other'},answer()]);
  await o(native('session.idle',{sessionID:'s1'}));
  await o(native('new.thing',{sessionID:'s1',prompt:'secret'}));
  await o(native('new.thing',{sessionID:'s1',prompt:'different secret'}));
  assert.deepEqual(h.out, [{...fixture[4], nativeType:'new.thing'}]);
  assert.ok(h.diagnostics.includes('invalid or mismatched messages'));
});

test('idle status and idle event collapse into one semantic completion', async () => {
  const h = harness(), o = h.observer.observe;
  await o(native('message.updated',{info:user()}));
  await o(native('message.updated',{info:answer()}));
  await o(native('session.status',{sessionID:'s1',status:{type:'idle'}}));
  await o(native('session.idle',{sessionID:'s1'}));
  assert.deepEqual(h.out, [fixture[0]]);
  assert.equal(h.lookups(), 1);
});

test('lookup alone cannot promote an unseen assistant completion', async () => {
  const h = harness(), o = h.observer.observe;
  await o(native('message.updated',{info:user()}));
  await o(native('session.idle',{sessionID:'s1'}));
  assert.deepEqual(h.out, []);
  assert.equal(h.lookups(), 0);
  await o(native('message.updated',{info:answer()}));
  await o(native('session.idle',{sessionID:'s1'}));
  assert.deepEqual(h.out, [fixture[0]]);
});

test('retry can complete after a new final assistant update', async () => {
  const h = harness(), o = h.observer.observe;
  await o(native('message.updated',{info:user()}));
  await o(native('message.updated',{info:answer()}));
  await o(native('session.status',{sessionID:'s1',status:{type:'retry',attempt:1}}));
  await o(native('message.updated',{info:answer()})); // replay of the old output
  await o(native('session.idle',{sessionID:'s1'}));
  assert.deepEqual(h.out, []);
  h.setMessages([user(), {...answer(),id:'a2'}]);
  await o(native('message.updated',{info:{...answer(),id:'a2'}}));
  await o(native('session.idle',{sessionID:'s1'}));
  assert.deepEqual(h.out, [{...fixture[0],messageID:'a2'}]);
});

test('a resolved request needs a later assistant update before completion', async () => {
  const h = harness(), o = h.observer.observe;
  await o(native('message.updated',{info:user()}));
  await o(native('message.updated',{info:answer()}));
  await o(native('question.asked',{sessionID:'s1',id:'q1',questions:[{}]}));
  await o(native('question.replied',{sessionID:'s1',requestID:'q1'}));
  await o(native('session.idle',{sessionID:'s1'}));
  assert.deepEqual(h.out, [fixture[1]]);
  h.setMessages([user(), {...answer(),id:'a2'}]);
  await o(native('message.updated',{info:{...answer(),id:'a2'}}));
  await o(native('session.idle',{sessionID:'s1'}));
  assert.deepEqual(h.out, [fixture[1], {...fixture[0],messageID:'a2'}]);
});

test('resolution during bounded lookup suppresses an obsolete request', async () => {
  const out = [];
  let resume;
  const waiting = new Promise((resolve) => { resume = resolve; });
  const observer = createObserver({ emit: (x) => out.push(x), client: { session: { get: async ({path}) => ({data:{id:path.id}}), messages: async () => {
    await waiting;
    return {data:[user(),answer()]};
  } } } });
  await observer.observe(native('message.updated',{info:user()}));
  const pending = observer.observe(native('question.asked',{sessionID:'s1',id:'q1',questions:[{}]}));
  await observer.observe(native('question.replied',{sessionID:'s1',requestID:'q1'}));
  resume();
  await pending;
  assert.deepEqual(out, []);
});

test('failed callback is diagnostic and cannot replay an ambiguous effect', async () => {
  let attempts = 0;
  const diagnostics = [];
  const observer = createObserver({ emit: async () => { attempts++; throw Error('secret'); }, onDiagnostic: (x) => diagnostics.push(x), client: { session: { get: async ({path}) => ({data:{id:path.id}}), messages: async () => ({data:[user(),answer()]}) } } });
  await observer.observe(native('message.updated',{info:user()}));
  await observer.observe(native('message.updated',{info:answer()}));
  await observer.observe(native('session.idle',{sessionID:'s1'}));
  await observer.observe(native('session.idle',{sessionID:'s1'}));
  assert.equal(attempts, 1);
  assert.deepEqual(diagnostics, ['observed event callback failed']);
});

test('terminal error rejects mismatched message and stale session state', async () => {
  const h = harness(), o = h.observer.observe;
  await o(native('message.updated',{info:user()}));
  h.setMessages([user('u2')]);
  await o(native('session.error',{sessionID:'s1',error:{name:'Error'}}));
  assert.deepEqual(h.out, []);
  await o(native('message.updated',{info:user('u2')}));
  await o(native('session.error',{sessionID:'s1',messageID:'a1',error:{name:'Error'}}));
  assert.deepEqual(h.out, []);
  assert.ok(h.diagnostics.includes('unmatched error message'));
});

test('assistant error update during lookup cannot erase terminal error', async () => {
  const out = [];
  let release;
  const lookup = new Promise((resolve) => { release = resolve; });
  const observer = createObserver({ emit: (fact) => out.push(fact), client: { session: { get: async ({path}) => ({data:{id:path.id}}),
    messages: async () => { await lookup; return { data: [user(), {...answer(), error: { name: 'Error' }}] }; },
  } } });
  await observer.observe(native('message.updated', { info: user() }));
  const error = observer.observe(native('session.error', { sessionID: 's1', error: { name: 'Error' } }));
  await observer.observe(native('message.updated', { info: { ...answer(), error: { name: 'Error' } } }));
  release();
  await error;
  assert.deepEqual(out.map((x) => x.kind), ['terminal_error']);
});

test('independent requests survive overlapping lookups', async () => {
  const out = [];
  const observer = createObserver({ emit: (fact) => out.push(fact), client: { session: { get: async ({path}) => ({data:{id:path.id}}),
    messages: async () => { await new Promise((resolve) => setTimeout(resolve, 5)); return { data: [user(), answer()] }; },
  } } });
  await observer.observe(native('message.updated', { info: user() }));
  await Promise.all([
    observer.observe(native('permission.asked', { sessionID: 's1', id: 'p1', permission: 'bash' })),
    observer.observe(native('permission.asked', { sessionID: 's1', id: 'p2', permission: 'bash' })),
  ]);
  assert.deepEqual(out.map((x) => x.requestID).sort(), ['p1', 'p2']);
});

test('resolution requires a later final update and tombstones a reordered ask', async () => {
  const h = harness(), o = h.observer.observe;
  await o(native('message.updated', { info: user() }));
  await o(native('question.asked', { sessionID: 's1', id: 'q1', questions: [{}] }));
  await o(native('message.updated', { info: answer() }));
  await o(native('question.replied', { sessionID: 's1', requestID: 'q1' }));
  await o(native('question.asked', { sessionID: 's1', id: 'q1', questions: [{}] }));
  await o(native('session.idle', { sessionID: 's1' }));
  assert.deepEqual(h.out.map((x) => x.kind), ['question_asked']);
  await o(native('message.updated', { info: answer() }));
  await o(native('session.idle', { sessionID: 's1' }));
  assert.deepEqual(h.out.map((x) => x.kind), ['question_asked', 'turn_idle_verified']);
});

test('an older assistant error does not cancel the current turn', async () => {
  const h = harness(), o = h.observer.observe;
  await o(native('message.updated', { info: user() }));
  await o(native('message.updated', { info: { ...answer('older'), error: { name: 'Error' } } }));
  await o(native('message.updated', { info: answer() }));
  await o(native('session.idle', { sessionID: 's1' }));
  assert.deepEqual(h.out, [fixture[0]]);
});

test('unknown-event cache eviction cannot replay an admitted completion', async () => {
  const out = [];
  const observer = createObserver({ dedupLimit: 1, emit: (fact) => out.push(fact), client: { session: { get: async ({path}) => ({data:{id:path.id}}),
    messages: async () => ({ data: [user(), answer()] }),
  } } });
  await observer.observe(native('message.updated', { info: user() }));
  await observer.observe(native('message.updated', { info: answer() }));
  await observer.observe(native('session.idle', { sessionID: 's1' }));
  await observer.observe(native('future.event', { sessionID: 's1' }));
  await observer.observe(native('session.idle', { sessionID: 's1' }));
  assert.equal(out.filter((x) => x.kind === 'turn_idle_verified').length, 1);
});

test('a lookup cannot emit facts from an evicted session state', async () => {
  for (const event of [
    native('session.idle', { sessionID: 's1' }),
    native('question.asked', { sessionID: 's1', id: 'q1', questions: [{}] }),
    native('session.error', { sessionID: 's1', error: { name: 'Error' } }),
  ]) {
    const out = [];
    let startLookup, resumeLookup;
    const started = new Promise((resolve) => { startLookup = resolve; });
    const waiting = new Promise((resolve) => { resumeLookup = resolve; });
    const observer = createObserver({ dedupLimit: 1, emit: (fact) => out.push(fact), client: { session: {
      get: async ({ path }) => ({ data: { id: path.id } }),
      messages: async () => { startLookup(); await waiting; return { data: [user(), answer()] }; },
    } } });
    await observer.observe(native('message.updated', { info: user() }));
    if (event.type === 'session.idle') await observer.observe(native('message.updated', { info: answer() }));
    const pending = observer.observe(event);
    await started;
    await observer.observe(native('message.updated', { info: user('other-user', 's2') }));
    await observer.observe(native('message.updated', { info: user('new-turn') }));
    resumeLookup();
    await pending;
    assert.deepEqual(out, [], event.type);
  }
});

test('bounded latest window still verifies an observed long turn', async () => {
  const out = [];
  const tail = Array.from({ length: 29 }, (_, i) => ({ ...answer(), id: `a${i}` }));
  tail.push(answer());
  const observer = createObserver({ emit: (fact) => out.push(fact), client: { session: { get: async ({path}) => ({data:{id:path.id}}),
    messages: async () => ({ data: tail.map((info) => ({ info, parts: [] })) }),
  } } });
  await observer.observe(native('message.updated', { info: user() }));
  await observer.observe(native('message.updated', { info: answer() }));
  await observer.observe(native('session.idle', { sessionID: 's1' }));
  assert.deepEqual(out, [fixture[0]]);
});

test('stalled lookup has a deadline and bounded concurrency', async () => {
  const diagnostics = [];
  const observer = createObserver({ lookupTimeoutMs: 100, maxConcurrentLookups: 1,
    emit: () => { throw new Error('should not emit'); }, onDiagnostic: (reason) => diagnostics.push(reason),
    client: { session: { get: async ({path}) => ({data:{id:path.id}}), messages: async () => new Promise(() => {}) } },
  });
  await observer.observe(native('message.updated', { info: user() }));
  await observer.observe(native('message.updated', { info: answer() }));
  const first = observer.observe(native('session.idle', { sessionID: 's1' }));
  await new Promise((resolve) => setTimeout(resolve, 5));
  await observer.observe(native('session.idle', { sessionID: 's1' }));
  await first;
  assert.ok(diagnostics.includes('messages lookup capacity exceeded'));
  assert.ok(diagnostics.includes('lookup failed or timed out'));
});

test('child facts carry verified ancestry for the main-session consumer', async () => {
  const out = [];
  const observer = createObserver({ emit: (fact) => out.push(fact), client: { session: {
    get: async ({ path }) => ({ data: { id: path.id, parentID: 'parent-session' } }),
    messages: async () => ({ data: [user(), answer()] }),
  } } });
  await observer.observe(native('message.updated', { info: user() }));
  await observer.observe(native('message.updated', { info: answer() }));
  await observer.observe(native('session.idle', { sessionID: 's1' }));
  assert.deepEqual(out, [{ ...fixture[0], rootSession: false }]);
});

test('rejected old request and old error do not poison current completion', async () => {
  const h = harness(), o = h.observer.observe;
  await o(native('message.updated', { info: user() }));
  await o(native('message.updated', { info: answer() }));
  await o(native('question.asked', { sessionID: 's1', id: 'q-old', messageID: 'old-assistant', questions: [{}] }));
  await o(native('session.error', { sessionID: 's1', messageID: 'old-assistant', error: { name: 'Error' } }));
  await o(native('session.idle', { sessionID: 's1' }));
  assert.deepEqual(h.out, [fixture[0]]);
});

test('unrelated resolution does not clear current final assistant', async () => {
  const h = harness(), o = h.observer.observe;
  await o(native('message.updated', { info: user() }));
  await o(native('message.updated', { info: answer() }));
  await o(native('question.replied', { sessionID: 's1', requestID: 'old-question' }));
  await o(native('session.idle', { sessionID: 's1' }));
  assert.deepEqual(h.out, [fixture[0]]);
});
