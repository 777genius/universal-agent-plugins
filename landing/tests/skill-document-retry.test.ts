import assert from 'node:assert/strict';
import test, { type TestContext } from 'node:test';
import type { Ref } from 'vue';
import { useSkillDocument } from '../composables/useSkillDocument.ts';

// Nuxt supplies these lifecycle and ref functions as auto-imports. The test
// controls their lifecycle so a pending retry can be exercised without a browser.
function setup(t: TestContext, responses: (url: string, options?: RequestInit) => Promise<Response>) {
  let mount = () => {};
  let changed = () => {};
  let dispose = () => {};
  const global = globalThis as unknown as {
    ref: (value: unknown) => { value: unknown };
    shallowRef: (value?: unknown) => { value: unknown };
    useRuntimeConfig: () => { public: { skillsCatalogEndpoint: string } };
    onMounted: (fn: () => void) => void;
    onScopeDispose: (fn: () => void) => void;
    watch: (refs: unknown, fn: () => void) => void;
  };
  global.ref = (value: unknown) => ({ value });
  global.shallowRef = (value?: unknown) => ({ value });
  global.useRuntimeConfig = () => ({ public: { skillsCatalogEndpoint: 'https://example.invalid/v1/skills' } });
  global.onMounted = (fn: () => void) => { mount = fn; };
  global.onScopeDispose = (fn: () => void) => { dispose = fn; };
  global.watch = (_refs: unknown, fn: () => void) => { changed = fn; };
  const fetchMock: typeof fetch = (url, options) => responses(String(url), options);
  t.mock.method(globalThis, 'fetch', fetchMock);
  const id = { value: 'skills-sh:example/first:first' } as Ref<string>;
  const name = { value: 'first' } as Ref<string>;
  const state = useSkillDocument(id, name);
  return { id, name, state, mount: () => mount(), changed: () => changed(), dispose: () => dispose() };
}

const busy = (retryAfter: unknown = 1) => new Response(JSON.stringify({ error: { code: 'preview_busy' }, retryAfter }), { status: 429 });
const resolved = (markdown: string) => new Response(JSON.stringify({ status: 'resolved', source: 'example', markdown }), { status: 200 });
const settle = () => new Promise<void>((resolve) => setImmediate(resolve));

test('busy preview waits for the server and retries once', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  let calls = 0;
  const view = setup(t, async () => ++calls === 1 ? busy(2) : resolved('# First'));
  view.mount();
  await settle();
  assert.equal(view.state.waitingForRetry.value, true);
  assert.equal(view.state.loading.value, true);
  t.mock.timers.tick(1999);
  await settle();
  assert.equal(calls, 1);
  t.mock.timers.tick(1);
  await settle();
  assert.equal(calls, 2);
  assert.equal(view.state.document.value?.markdown, '# First');
  assert.equal(view.state.waitingForRetry.value, false);
  assert.equal(view.state.failed.value, false);
  view.dispose();
});

test('a second busy response fails and leaves manual retry available', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  let calls = 0;
  const view = setup(t, async () => ++calls <= 2 ? busy() : resolved('# Recovered'));
  view.mount();
  await settle();
  t.mock.timers.tick(1000);
  await settle();
  assert.equal(calls, 2);
  assert.equal(view.state.failed.value, true);
  assert.equal(view.state.loading.value, false);
  await view.state.retry();
  assert.equal(calls, 3);
  assert.equal(view.state.document.value?.markdown, '# Recovered');
  assert.equal(view.state.failed.value, false);
  view.dispose();
});

test('missing retry hint uses the API default rather than retrying early', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  let calls = 0;
  const view = setup(t, async () => ++calls === 1 ? new Response('{}', { status: 429 }) : resolved('# First'));
  view.mount();
  await settle();
  t.mock.timers.tick(19999);
  await settle();
  assert.equal(calls, 1);
  t.mock.timers.tick(1);
  await settle();
  assert.equal(calls, 2);
  assert.equal(view.state.document.value?.markdown, '# First');
  view.dispose();
});

test('changing Skill cancels the pending retry and preserves the new result', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const calls: string[] = [];
  const view = setup(t, async (url) => { calls.push(url); return calls.length === 1 ? busy() : resolved('# Second'); });
  view.mount();
  await settle();
  assert.equal(view.state.waitingForRetry.value, true);
  view.id.value = 'skills-sh:example/second:second';
  view.name.value = 'second';
  view.changed();
  await settle();
  t.mock.timers.tick(5000);
  await settle();
  assert.equal(calls.length, 2);
  assert.match(calls[1], /second/);
  assert.equal(view.state.document.value?.markdown, '# Second');
  assert.equal(view.state.failed.value, false);
  view.dispose();
});

test('a late response for the previous Skill cannot overwrite the current one', async (t) => {
  let finishFirst!: (response: Response) => void;
  let calls = 0;
  const view = setup(t, async () => {
    if (++calls === 1) return new Promise<Response>((resolve) => { finishFirst = resolve; });
    return resolved('# Second');
  });
  view.mount();
  view.id.value = 'skills-sh:example/second:second';
  view.name.value = 'second';
  view.changed();
  await settle();
  finishFirst(resolved('# First'));
  await settle();
  assert.equal(view.state.document.value?.markdown, '# Second');
  assert.equal(view.state.failed.value, false);
  view.dispose();
});

test('unmount cancels a waiting retry and ordinary failures fail immediately', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  let calls = 0;
  const view = setup(t, async () => { calls++; return busy(); });
  view.mount();
  await settle();
  view.dispose();
  t.mock.timers.tick(5000);
  await settle();
  assert.equal(calls, 1);

  const errorView = setup(t, async () => new Response('', { status: 503 }));
  errorView.mount();
  await settle();
  assert.equal(errorView.state.failed.value, true);
  assert.equal(errorView.state.waitingForRetry.value, false);
  errorView.dispose();
});

test('invalid or excessive retry hints do not trigger an early retry', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  for (const delay of [0, -1, 31, '1', 1.5]) {
    let calls = 0;
    const view = setup(t, async () => { calls++; return busy(delay); });
    view.mount();
    await settle();
    t.mock.timers.tick(60000);
    await settle();
    assert.equal(calls, 1);
    assert.equal(view.state.failed.value, true);
    view.dispose();
  }
});
