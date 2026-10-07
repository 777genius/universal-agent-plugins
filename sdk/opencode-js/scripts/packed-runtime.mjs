// External consumer: resolve exports from an extracted/installed package, never source imports.
import assert from 'node:assert/strict';
import { createObserver, createV2Observer } from 'universal-agent-plugins-opencode-events';
import { createObserver as v1 } from 'universal-agent-plugins-opencode-events/v1';
import { createV2Observer as strict } from 'universal-agent-plugins-opencode-events/v2';
const tick = () => new Promise(setImmediate);
const facts = [];
const first = createObserver({ client: { session: { get: async ({ path }) => ({ id: path.id }),
  messages: async () => [] } }, emit() {} });
await first.observe({ type: 'message.updated', properties: { info: { id: 'user', sessionID: 's', role: 'user' } } });
first.dispose(); assert.equal(v1, createObserver);
const push = createV2Observer({ client: {
  get: async ({ sessionID }) => ({ id: sessionID, location: { directory: '/TEST-packed' } }),
  context: async () => [{ id: 'native-user', type: 'user' },
    { id: 'native-assistant', type: 'assistant', finish: 'stop', time: { completed: 1 } }],
}, location: { directory: '/TEST-packed' }, emit: (event) => facts.push(event) });
const observe = (type, data = {}) => push.observe({ type, data: { sessionID: 'session', ...data } });
observe('session.inbox.enqueued', { inboxID: 'native-user', item: { type: 'user' } });
observe('session.execution.started'); observe('session.inbox.delivered', { inboxID: 'native-user' });
observe('session.step.started', { assistantMessageID: 'native-assistant' });
observe('session.step.ended', { assistantMessageID: 'native-assistant', finish: 'stop' });
observe('session.execution.succeeded'); await tick(); push.dispose();
assert.deepEqual(facts, [{ version: 1, sessionID: 'session', turnID: 'native-user', rootSession: true,
  kind: 'turn_idle_verified', messageID: 'native-assistant' }]);
// Same semantic execution through owned reader. Strict native proofs are an independent
// unit port model, not native installed-host evidence. Unknown legacy provenance stayed absent.
let sequence = 0;
const envelopes = [
  ['session.inbox.enqueued', { inboxID: 'native-user', item: { type: 'user' } }],
  ['session.execution.started', {}], ['session.inbox.delivered', { inboxID: 'native-user' }],
  ['session.step.started', { assistantMessageID: 'native-assistant' }],
  ['session.step.ended', { assistantMessageID: 'native-assistant', finish: 'stop' }],
  ['session.execution.succeeded', {}],
].map(([type, data]) => ({ type, id: `evt-${++sequence}`, created: 1790867856742,
  data: { sessionID: 'session', ...data, sequence: { aggregate: 'session', seq: sequence } } }));
let subscribed = false, end, preparedFact, preparation;
const owned = strict({ location: '/TEST-packed', runtimeEligibility: () => 'supported',
  context: { app: { version: '2.0.21' }, event: { subscribe({ signal }) {
    subscribed = true;
    const stopped = new Promise((resolve) => { end = resolve; });
    signal.addEventListener('abort', end, { once: true });
    return { async *[Symbol.asyncIterator]() { for (const event of envelopes) yield event; await stopped; } };
  } } }, native: {
    correlate(event) { return { sessionID: event.data.sessionID, sequence: event.data.sequence,
      messageID: event.data.assistantMessageID }; },
    session: async (run) => ({ ...run, rootSession: true }),
    assistant: async (run) => ({ ...run, rootSession: true, role: 'assistant', summary: false, final: true, outcome: 'success' }),
  }, beforeEmit: async (event, handoff) => {
    assert.ok(Object.isFrozen(event)); assert.equal(handoff.revalidate, undefined);
    preparedFact = event; preparation = handoff; return true;
  }, emit: (event, handoff) => {
    assert.equal(event, preparedFact); assert.equal(handoff.ingressMonotonicMs, preparation.ingressMonotonicMs);
    assert.equal(handoff.metadataDeadline, preparation.metadataDeadline);
    assert.equal(handoff.isCurrent(), true); facts.push(event);
  } });
owned.start(); await tick(); assert.equal(subscribed, true);
assert.equal(facts.length, 2); assert.equal(facts[1].turnID, facts[0].turnID);
assert.equal(facts[1].provenance.nativeEventID, 'evt-6');
owned.dispose(); await owned.done();
console.log('external packed root factories, V1 subpath and strict V2 execution passed');
