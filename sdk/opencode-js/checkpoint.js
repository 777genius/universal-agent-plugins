import { randomBytes } from 'node:crypto';
import { id } from './observer-core.js';

// Private public-RPC binding only. No pending read, transport, history or lease.
// The binding projects its registered portable event to a closed 128-bit nonce.
export function createCheckpoint(port, core, fail) {
  let current;
  function invalidate() {
    const state = current;
    if (!state) return;
    state.ready = false;
    for (const resolve of state.waiters.values()) resolve(false);
    state.waiters.clear(); state.retired.clear();
  }
  function consume(envelope) {
    const state = current;
    if (!state?.registration || state.signal.aborted || envelope.type !== state.registration.type) return false;
    let nonce;
    try { nonce = state.registration.read(envelope); }
    catch { fail('checkpoint_schema_unverified'); return true; }
    if (nonce === undefined) { fail('checkpoint_schema_unverified'); return true; }
    if (state.retired.delete(nonce)) return true;
    if (typeof nonce !== 'string' || !/^[a-f0-9]{32}$/.test(nonce) || !state.waiters.has(nonce)) {
      fail('checkpoint_continuity_unverified'); return true;
    }
    const resolve = state.waiters.get(nonce);
    state.waiters.delete(nonce);
    resolve(true); // Only this independent reader can satisfy the snapshot.
    return true;
  }
  async function marker(state, handoff) {
    if (state !== current || !handoff.isCurrent() || !state.registration) return false;
    const result = await core.lookup(async (signal) => {
      const nonce = randomBytes(16).toString('hex');
      let finish, wasConsumed = false;
      const consumed = new Promise((resolve) => {
        finish = (value) => { if (value) wasConsumed = true; resolve(value); };
        state.waiters.set(nonce, finish);
      });
      const abort = () => finish(false);
      signal.addEventListener('abort', abort, { once: true });
      try {
        if (signal.aborted) return false;
        // Emit returning does NOT prove consumption; await both under one budget.
        await state.registration.emit(nonce);
        return await consumed;
      } finally {
        state.waiters.delete(nonce);
        if (!wasConsumed && !state.signal.aborted && state === current) {
          // Consume a cancelled job's published packet once without reopening it.
          if (state.retired.size >= 256) fail('checkpoint_capacity');
          else state.retired.add(nonce);
        }
        signal.removeEventListener('abort', abort);
      }
    }, handoff);
    if (!handoff.isCurrent()) return false;
    if (!result || state !== current) {
      if (state === current && !state.signal.aborted) fail('checkpoint_failed');
      return false;
    }
    return true;
  }
  async function open(signal) {
    invalidate();
    const state = { signal, ready: false, registration: undefined, waiters: new Map(), retired: new Set() };
    current = state;
    if (!port) { core.diag('form_checkpoint_unavailable'); return; }
    const handoff = { signal, isCurrent: () => current === state && !signal.aborted,
      metadataDeadline: core.now() + 2000 };
    const registration = await core.lookup(async () => {
      const owned = await port.register(signal, randomBytes(16).toString('hex'));
      if (!handoff.isCurrent()) { await owned?.dispose?.(); return; }
      state.registration = owned;
      return owned;
    }, handoff);
    if (!handoff.isCurrent()) return;
    if (!registration || typeof registration.emit !== 'function' || typeof registration.read !== 'function' ||
        typeof registration.dispose !== 'function' || !id(registration.type)) {
      fail('checkpoint_registration_unverified'); return;
    }
    if (await marker(state, handoff)) state.ready = true;
  }
  async function close() {
    invalidate();
    const state = current;
    current = undefined;
    // Called only after the owned reader has stopped; registration outlives ingress.
    if (state?.registration) {
      try { await state.registration.dispose(); } catch { core.diag('checkpoint_dispose_failed'); }
    }
  }
  return { open, close, consume, invalidate,
    ready: () => Boolean(current?.ready && !current.signal.aborted),
    async verify(handoff) {
      const state = current;
      return Boolean(state?.ready && await marker(state, handoff) && state.ready);
    } };
}
