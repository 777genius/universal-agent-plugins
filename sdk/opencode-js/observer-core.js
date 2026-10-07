// Host-neutral bounds and cancellation only. Native reducers stay separate.
export const object = (x) => x !== null && typeof x === 'object' && !Array.isArray(x);
export const id = (x) => typeof x === 'string' && x.length > 0 && !/[\u0000-\u001f]/u.test(x) && new TextEncoder().encode(x).length <= 256;
export const timestamp = (x) => Number.isSafeInteger(x) && x > 0;
export const canonicalID = (generation, session, turn, kind, nativeID) =>
  JSON.stringify([generation, session, turn, kind, nativeID]);

export function createCore(options) {
  const now = options.clock ? () => options.clock.now() : () => performance.now();
  const clockID = options.clock?.id ?? 'local-performance';
  if (!id(clockID) || !Number.isFinite(now())) throw new TypeError('valid monotonic clock required');
  const diag = (reason) => { try { options.onDiagnostic?.(reason); } catch {} };
  const jobs = new Set(), lookups = new Set(), tails = new Map(), emitWaiters = new Set();
  let generation = 0, emitting = 0;
  const limit = (x, max) => Number.isInteger(x) ? Math.max(1, Math.min(max, x)) : max;
  const jobLimit = limit(options.maxJobs, 256), lookupLimit = limit(options.maxConcurrentLookups, 16);
  const timeout = limit(options.lookupTimeoutMs, 2000);
  function invalidate() {
    generation++;
    for (const job of jobs) job.controller.abort();
    for (const controller of lookups) controller.abort();
    for (const wake of emitWaiters) wake();
  }
  function submit(sid, relevant, work, born = now()) {
    if (jobs.size >= jobLimit) { diag('job_capacity'); return; }
    const controller = new AbortController(), connection = generation;
    const valid = () => !controller.signal.aborted && connection === generation &&
      now() - born <= 30000 && relevant();
    const job = { controller, valid };
    jobs.add(job);
    const timer = setTimeout(() => { diag('job_expired'); controller.abort(); }, Math.max(0, 30000 - (now() - born)));
    const before = tails.get(sid) ?? Promise.resolve();
    const after = before.then(async () => {
      if (valid()) await work({ signal: controller.signal, isCurrent: valid, metadataDeadline: now() + timeout, ingressMonotonicMs: born, clockID });
    }).catch(() => diag('job_failed')).finally(() => {
      clearTimeout(timer); jobs.delete(job);
      if (tails.get(sid) === after) tails.delete(sid);
    });
    tails.set(sid, after);
    return after;
  }
  async function lookup(call, handoff) {
    if (!handoff.isCurrent()) return;
    if (lookups.size >= lookupLimit) { diag('lookup_capacity'); return; }
    const remaining = Math.min(timeout, (handoff.metadataDeadline ?? now() + timeout) - now());
    if (remaining <= 0) { diag('lookup_timeout'); return; }
    const deadline = now() + remaining;
    const controller = new AbortController();
    lookups.add(controller);
    const abort = () => controller.abort();
    handoff.signal.addEventListener('abort', abort, { once: true });
    let timer, cancel;
    const request = Promise.resolve().then(() => {
      if (controller.signal.aborted) return;
      if (now() > deadline) { diag('lookup_timeout'); controller.abort(); return; }
      return call(controller.signal);
    });
    // Hold capacity until actual settlement, including abort-ignoring ports.
    request.then(() => lookups.delete(controller), () => lookups.delete(controller));
    try {
      const result = await Promise.race([request, new Promise((resolve) => {
        cancel = () => resolve(undefined);
        controller.signal.addEventListener('abort', cancel, { once: true });
        timer = setTimeout(() => { diag('lookup_timeout'); controller.abort(); }, remaining);
        if (controller.signal.aborted) cancel();
      })]);
      if (now() > deadline && !controller.signal.aborted) { diag('lookup_timeout'); controller.abort(); }
      return handoff.isCurrent() && !controller.signal.aborted ? result : undefined;
    } catch { diag('lookup_failed'); }
    finally {
      clearTimeout(timer);
      controller.signal.removeEventListener('abort', cancel);
      handoff.signal.removeEventListener('abort', abort);
    }
  }
  async function emit(fact, handoff) {
    const deadline = handoff.metadataDeadline ?? now() + timeout;
    const current = () => handoff.isCurrent() && now() < deadline;
    while (emitting >= 4 && current()) {
      await new Promise((resolve) => {
        let timer;
        const wake = () => {
          clearTimeout(timer); emitWaiters.delete(wake);
          handoff.signal.removeEventListener('abort', wake); resolve();
        };
        emitWaiters.add(wake); handoff.signal.addEventListener('abort', wake, { once: true });
        timer = setTimeout(wake, Math.max(0, deadline - now()));
        if (handoff.signal.aborted) wake();
      });
    }
    if (!current()) return false;
    if (fact.provenance) Object.freeze(fact.provenance);
    Object.freeze(fact);
    if (new TextEncoder().encode(JSON.stringify(fact)).length > 4096) { diag('frame_capacity'); return false; }
    emitting++;
    let preparation, preparationController;
    const abortPreparation = () => preparationController?.abort();
    handoff.signal.addEventListener('abort', abortPreparation, { once: true });
    try {
      if (options.beforeEmit) {
        const ready = await lookup((signal) => {
          preparationController = new AbortController();
          const abort = () => preparationController.abort();
          signal.addEventListener('abort', abort, { once: true });
          if (signal.aborted || handoff.signal.aborted) abort();
          const projected = Object.freeze({ signal: preparationController.signal,
            ingressMonotonicMs: handoff.ingressMonotonicMs, clockID: handoff.clockID,
            metadataDeadline: deadline,
            isCurrent: () => current() && !preparationController.signal.aborted });
          preparation = Promise.resolve(projected.isCurrent() ? options.beforeEmit(fact, projected) : false);
          preparation.then(() => signal.removeEventListener('abort', abort),
            () => signal.removeEventListener('abort', abort));
          return preparation;
        }, { ...handoff, metadataDeadline: deadline });
        if (ready !== true || !current()) return false;
      }
      if (handoff.revalidate && !await handoff.revalidate()) return false;
      if (!current()) return false;
      // Final callback entry is synchronous. Its first await follows owned spawn.
      await options.emit(fact, handoff);
      return true;
    }
    catch { diag('observed event callback failed'); return false; }
    finally {
      abortPreparation();
      // A soft timeout/abort is not actual hook settlement: retain callback/job
      // ownership as well as lookup capacity until an ignoring hook really ends.
      try { await preparation; } catch {}
      handoff.signal.removeEventListener('abort', abortPreparation);
      emitting--; for (const wake of emitWaiters) wake();
    }
  }
  return { now, clockID, diag, submit, lookup, emit, invalidate,
    recheck() { for (const job of jobs) if (!job.valid()) job.controller.abort(); },
    drain: () => Promise.all([...tails.values()]),
    busy: () => jobs.size > 0 };
}
