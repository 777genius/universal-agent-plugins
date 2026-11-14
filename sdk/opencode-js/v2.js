import { id } from './observer-core.js';
import { createV2Reducer } from './v2-reducer.js';
import { createCheckpoint } from './checkpoint.js';

/** @param {import('./v2.d.ts').V2ObserverOptions} options */
export function createV2Observer(options) {
  if (typeof options?.context?.event?.subscribe !== 'function' || typeof options.emit !== 'function' ||
      typeof options.runtimeEligibility !== 'function' || typeof options.native?.correlate !== 'function' ||
      typeof options.native?.session !== 'function' || typeof options.native?.assistant !== 'function' || !id(options.location)) {
    throw new TypeError('context, native ports, location, runtimeEligibility and emit required');
  }
  const lifetime = new AbortController();
  let connection, started = false, task = Promise.resolve(), replacements = 0;
  function invalidate(reason) {
    reducer.invalidate(); connection?.abort(); reducer.diag(reason);
  }
  const reducer = createV2Reducer(options, true, invalidate), core = reducer.core;
  const checkpoint = createCheckpoint(options.checkpoint, core, invalidate);
  reducer.setCheckpoint(checkpoint);
  async function delay(ms) {
    await new Promise((resolve) => {
      const done = () => { clearTimeout(timer); lifetime.signal.removeEventListener('abort', done); resolve(); };
      const timer = setTimeout(done, ms);
      lifetime.signal.addEventListener('abort', done, { once: true });
      if (lifetime.signal.aborted) done();
    });
  }
  async function read() {
    while (!lifetime.signal.aborted) {
      connection = new AbortController();
      try {
        const stream = options.context.event.subscribe({ signal: connection.signal });
        // for-await requests the first item before this microtask emits readiness.
        const opening = Promise.resolve().then(() => checkpoint.open(connection.signal));
        for await (const event of stream) {
          if (lifetime.signal.aborted || connection.signal.aborted) break;
          reducer.observe(event);
          if (connection.signal.aborted) break;
        }
        if (!lifetime.signal.aborted) invalidate('subscription_ended');
        await opening;
      } catch { if (!lifetime.signal.aborted) invalidate('subscription_error'); }
      connection.abort();
      await checkpoint.close();
      if (lifetime.signal.aborted || replacements >= 3) break;
      await delay([250, 1000, 2000][replacements++]);
    }
    if (!lifetime.signal.aborted) core.diag('subscription_replacements_exhausted');
  }
  return {
    start() {
      if (started || lifetime.signal.aborted) return;
      started = true;
      let eligibility;
      try { eligibility = options.runtimeEligibility(options.context.app?.version); } catch {}
      if (eligibility !== 'supported') { core.diag(eligibility === 'unsupported' ? 'runtime_unsupported' : 'runtime_unverified'); return; }
      task = read();
    },
    dispose() { invalidate('observer_disposed'); lifetime.abort(); reducer.dispose(); },
    async done() { await task; await core.drain(); },
  };
}
