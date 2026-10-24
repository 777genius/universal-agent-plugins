import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import type { IncomingMessage, ServerResponse } from 'node:http';
import { test } from 'node:test';
import { createClient, ManagementError, type Operation } from '../dist/index.js';

const token = 'a'.repeat(64);
function operation(state: Operation['state'] = 'running'): Operation {
  return { operationId: 'operation-one', planId: 'plan-one', requestId: 'request-123456789', contextId: 'context-one', state, phase: 'commit', completed: 0, total: 1, items: [], diagnostics: [], createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z' };
}
async function fixture(handler: (request: IncomingMessage, response: ServerResponse) => void) {
  const server = createServer((request, response) => {
    assert.equal(request.headers.authorization, `Bearer ${token}`);
    assert.equal(request.headers.origin, origin);
    response.setHeader('content-type', 'application/json');
    handler(request, response);
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  assert.ok(address && typeof address !== 'string');
  const origin = `http://127.0.0.1:${address.port}`;
  return { client: createClient({ baseUrl: origin, token }), close: () => new Promise<void>((resolve, reject) => { server.closeAllConnections(); server.close((error) => error ? reject(error) : resolve()); }) };
}

test('typed client preserves origin, capability, escaped IDs and search query', async (t) => {
  const seen: string[] = [];
  const f = await fixture((request, response) => { seen.push(request.url ?? ''); response.end(JSON.stringify({ items: [], freshness: 'current', diagnostics: [] })); });
  t.after(f.close);
  await f.client.catalog('search & filter');
  await f.client.catalogEntry('publisher/plugin');
  assert.equal(new URL(seen[0]!, 'http://localhost').searchParams.get('query'), 'search & filter');
  assert.equal(seen[1], '/api/v1/catalog/publisher%2Fplugin');
});

test('lost apply response is not automatically replayed; caller reuses exact identity', async (t) => {
  let requests = 0;
  let accepted = 0;
  const identities = new Set<string>();
  const f = await fixture((request, response) => {
    requests++;
    let body = '';
    request.on('data', (chunk: Buffer) => { body += chunk.toString(); });
    request.on('end', () => {
      assert.equal(request.url, '/api/v1/plans/plan-one/apply');
      const id = String(JSON.parse(body).requestId);
      if (!identities.has(id)) { identities.add(id); accepted++; }
      if (requests === 1) request.socket.destroy();
      else { response.statusCode = 202; response.end(JSON.stringify(operation())); }
    });
  });
  t.after(f.close);
  await assert.rejects(f.client.apply('plan-one', 'request-123456789'));
  assert.equal(requests, 1);
  const resumed = await f.client.apply('plan-one', 'request-123456789');
  assert.equal(resumed.operationId, 'operation-one');
  assert.equal(requests, 2);
  assert.equal(accepted, 1);
});

test('structured conflict retains machine code and does not imply mutation retry', async (t) => {
  const f = await fixture((_request, response) => { response.statusCode = 409; response.end(JSON.stringify({ code: 'REQUEST_CONFLICT', message: 'The accepted input differs.', retryable: false })); });
  t.after(f.close);
  await assert.rejects(f.client.apply('p', 'request-123456789'), (error: unknown) => error instanceof ManagementError && error.code === 'REQUEST_CONFLICT' && error.status === 409 && !error.retryable);
});

test('wait returns interrupted result and reports only changed observations', async (t) => {
  let reads = 0;
  const f = await fixture((_request, response) => response.end(JSON.stringify(operation(++reads < 3 ? 'running' : 'interrupted'))));
  t.after(f.close);
  const states: string[] = [];
  const result = await f.client.wait('operation-one', { pollIntervalMs: 1, timeoutMs: 2_000, onProgress: (value) => states.push(value.state) });
  assert.equal(result.state, 'interrupted');
  assert.equal(result.result, undefined);
  assert.deepEqual(states, ['running', 'interrupted']);
});

test('aborting observation leaves accepted work running; cancellation is explicit', async (t) => {
  let cancels = 0;
  const f = await fixture((request, response) => { if (request.method === 'POST') cancels++; response.end(JSON.stringify(operation())); });
  t.after(f.close);
  const controller = new AbortController();
  await assert.rejects(f.client.wait('operation-one', { signal: controller.signal, onProgress: () => controller.abort(), pollIntervalMs: 1 }), { name: 'AbortError' });
  assert.equal(cancels, 0);
  assert.equal((await f.client.operation('operation-one')).state, 'running');
  await f.client.cancel('operation-one');
  assert.equal(cancels, 1);
});

test('deadline only stops waiting and unexpected state is not success', async (t) => {
  let futureState = false;
  let cancels = 0;
  const f = await fixture((request, response) => { if (request.method === 'POST') cancels++; response.end(JSON.stringify({ ...operation(), ...(futureState ? { state: 'future_state' } : {}) })); });
  t.after(f.close);
  await assert.rejects(f.client.wait('operation-one', { timeoutMs: 20, pollIntervalMs: 1 }), (error: unknown) => error instanceof ManagementError && error.code === 'WAIT_TIMEOUT');
  futureState = true;
  await assert.rejects(f.client.wait('operation-one'), (error: unknown) => error instanceof ManagementError && error.code === 'UNKNOWN_OPERATION_STATE');
  assert.equal(cancels, 0);
});

test('an already aborted wait performs no request', async (t) => {
  let reads = 0;
  const f = await fixture((_request, response) => { reads++; response.end(JSON.stringify(operation())); });
  t.after(f.close);
  await assert.rejects(f.client.wait('operation-one', { signal: AbortSignal.abort() }), { name: 'AbortError' });
  assert.equal(reads, 0);
});


test('Skill preview uses the retained catalog and candidate identities', async (t) => {
  const f = await fixture((request, response) => {
    assert.equal(request.url, '/api/v1/skills/catalog/catalog-one/candidates/skill%2Fone/preview');
    response.end(JSON.stringify({ catalogId: 'catalog-one', candidateId: 'skill/one', markdown: '# A Skill', diagnostics: [] }));
  });
  t.after(f.close);
  assert.equal((await f.client.skills.preview('catalog-one', 'skill/one')).markdown, '# A Skill');
});
