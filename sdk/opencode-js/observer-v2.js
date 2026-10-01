// Native V2 metadata reducer. Never retain prompt, tool, form or error bodies.
const object = (x) => x !== null && typeof x === 'object' && !Array.isArray(x);
const id = (x) => typeof x === 'string' && x.length > 0 && !/[\u0000-\u001f]/u.test(x) && new TextEncoder().encode(x).length <= 256;
const location = (x) => object(x) && typeof x.directory === 'string' && x.directory.length > 0 &&
  (x.workspaceID === undefined || id(x.workspaceID));
const sameLocation = (a, b) => location(a) && location(b) && a.directory === b.directory && a.workspaceID === b.workspaceID;
const bounded = (value, fallback, min, max) => Math.max(min, Math.min(max, Number.isInteger(value) ? value : fallback));
const retainedAdd = (set, value, limit) => { set.add(value); if (set.size > limit) set.delete(set.values().next().value); };
const supported = new Set([
  'session.created', 'session.moved', 'session.deleted', 'session.inbox.enqueued', 'session.inbox.delivered',
  'session.inbox.cancelled', 'session.inbox.delivery.changed', 'session.execution.started', 'session.execution.succeeded',
  'session.execution.failed', 'session.execution.interrupted', 'session.step.started', 'session.step.ended',
  'session.step.failed', 'session.retry.scheduled', 'session.compaction.started', 'session.compaction.ended',
  'session.compaction.failed', 'form.created', 'form.replied', 'form.cancelled', 'permission.asked', 'permission.replied',
]);

/** @param {import('./index.d.ts').V2ObserverOptions} options */
export function createV2Observer(options) {
  if (typeof options?.emit !== 'function' || typeof options.client?.get !== 'function' ||
      typeof options.client?.context !== 'function' || !location(options.location)) {
    throw new TypeError('emit, native get/context and location required');
  }
  const own = { directory: options.location.directory, workspaceID: options.location.workspaceID };
  const maxSessions = bounded(options.maxSessions, 512, 1, 512);
  const maxLookups = bounded(options.maxConcurrentLookups, 16, 1, 16);
  const timeout = bounded(options.lookupTimeoutMs, 2000, 100, 10000);
  const sessions = new Map(), eventIDs = new Set(), requests = new Set();
  const diagnosticCodes = new Set();
  let disposed = false, lifecycle = 0;
  const diag = (code) => {
    if (diagnosticCodes.has(code)) return;
    diagnosticCodes.add(code);
    try { options.onDiagnostic?.(code); } catch { /* advisory only */ }
  };
  function invalidate(s) { s.generation++; s.revision++; sessions.delete(s.sid); }
  function state(sid) {
    if (sessions.has(sid)) return sessions.get(sid);
    if (sessions.size === maxSessions) invalidate(sessions.values().next().value);
    const s = { sid, generation: 0, ownership: 'new', epoch: 0, revision: 0, started: false,
      user: '', assistant: '', final: '', retry: false, interrupted: false, compacting: false,
      compactedUser: '', blocked: false, terminal: '', result: '', admissions: new Map(),
      pending: new Map(), resolved: new Set(), admitted: new Set(), liveAssistants: new Set(), attempted: new Set(), verifying: new Set() };
    sessions.set(sid, s);
    return s;
  }
  const ownershipToken = (s) => ({ s, generation: s.generation, lifecycle });
  const ownedToken = (token) => !disposed && lifecycle === token.lifecycle &&
    sessions.get(token.s.sid) === token.s && token.s.generation === token.generation;
  const semanticToken = (s) => ({ ...ownershipToken(s), epoch: s.epoch, revision: s.revision });
  const current = (token) => ownedToken(token) && token.s.epoch === token.epoch && token.s.revision === token.revision;
  const liveWork = (s) => s.started && Boolean(s.user) && !s.result && !s.interrupted && !s.blocked;
  function lookup(call) {
    if (disposed || requests.size >= maxLookups) { diag('lookup_capacity'); return Promise.resolve(undefined); }
    const controller = new AbortController();
    let releaseWait, timer;
    const stopped = new Promise((resolve) => { releaseWait = resolve; });
    const record = { controller, stop: () => releaseWait(undefined), timer: undefined };
    requests.add(record);
    // Actual slot stays reserved until the host settles, even if it ignores signal.
    const underlying = Promise.resolve().then(() => disposed ? undefined : call(controller.signal));
    const result = underlying.then((value) => value, () => { diag('lookup_failed'); return undefined; });
    result.then(() => { requests.delete(record); clearTimeout(timer); });
    timer = setTimeout(() => { controller.abort(); diag('lookup_timeout'); record.stop(); }, timeout);
    record.timer = timer;
    return Promise.race([result, stopped]).finally(() => clearTimeout(timer));
  }
  function ensureOwnership(s) {
    if (s.ownership !== 'new') return;
    s.ownership = 'pending';
    const token = ownershipToken(s);
    void lookup((signal) => options.client.get({ sessionID: s.sid }, { signal })).then((info) => {
      if (!ownedToken(token)) return;
      if (!object(info) || info.id !== s.sid || (info.parentID !== undefined && !id(info.parentID)) ||
          !sameLocation(info.location, own)) { s.ownership = 'rejected'; diag('ownership_unverified'); return; }
      s.ownership = info.parentID === undefined ? 'root' : 'child';
      schedule(s); // Current metadata only; no event replay and no semantic revision on this token.
    });
  }
  async function context(s) {
    const rows = await lookup((signal) => options.client.context({ sessionID: s.sid }, { signal }));
    if (!Array.isArray(rows) || rows.length > 4096) { diag('context_unverified'); return; }
    const seen = new Set(), metadata = [];
    for (const row of rows) {
      if (!object(row) || !id(row.id) || seen.has(row.id) || typeof row.type !== 'string') {
        diag('context_unverified'); return;
      }
      seen.add(row.id);
      // Copy metadata fields explicitly, never clone native content.
      metadata.push({ id: row.id, type: row.type, finish: row.finish,
        completed: Number.isFinite(row.time?.completed), failed: row.error != null });
    }
    return metadata;
  }
  function associated(s, rows) {
    const lastUser = rows.findLast((row) => row.type === 'user');
    return lastUser ? lastUser.id === s.user : s.compactedUser === s.user && Boolean(s.user);
  }
  function emit(s, key, fact) {
    if (disposed || s.ownership !== 'root' || s.admitted.has(key)) return;
    if (s.admitted.size >= 2048) { s.blocked = true; diag('admission_capacity'); return; }
    s.admitted.add(key); // Callback failures/unknown delivery never make an item retryable.
    try { Promise.resolve(options.emit({ version: 1, sessionID: s.sid, turnID: s.user, rootSession: true, ...fact }))
      .catch(() => diag('callback_failed')); } catch { diag('callback_failed'); }
  }
  function schedule(s) {
    if (disposed || s.ownership !== 'root' || !liveWork(s)) return;
    const candidates = [...s.pending.values()].filter((candidate) => !s.admitted.has(`request:${candidate.kind}:${candidate.id}`));
    if (s.terminal && !s.result) candidates.push({ kind: s.terminal, id: '', messageID: s.final });
    for (const candidate of candidates) {
      if (candidate.kind === 'success' && (!s.final || s.retry || s.compacting || s.pending.size || s.admissions.size)) continue;
      const flight = `${s.epoch}:${candidate.kind}:${candidate.id}`;
      if (s.verifying.has(flight)) continue;
      const attempt = `${s.epoch}:${s.user}:${candidate.kind}:${candidate.id}:${candidate.messageID ?? ''}`;
      if (s.attempted.has(attempt)) continue;
      if (s.attempted.size >= 2048) { s.blocked = true; diag('verification_capacity'); return; }
      s.attempted.add(attempt);
      const token = semanticToken(s);
      s.verifying.add(flight);
      void verify(s, candidate, token).catch(() => diag('verification_failed')).finally(() => {
        s.verifying.delete(flight);
        if (ownedToken(token) && !current(token)) { s.attempted.delete(attempt); schedule(s); }
      });
    }
  }
  async function verify(s, candidate, token) {
    const rows = await context(s);
    if (!current(token)) return;
    if (!rows || !liveWork(s) || !associated(s, rows)) return;
    if (candidate.kind === 'success') {
      if (s.terminal !== 'success' || s.result || s.retry || s.compacting || s.pending.size || s.admissions.size || !s.final) return;
      const index = rows.findIndex((row) => row.id === s.final && row.type === 'assistant');
      const answer = rows[index];
      if (!answer || answer.finish !== 'stop' || !answer.completed || answer.failed ||
          rows.slice(index + 1).some((row) => row.type !== 'idle')) return;
      s.result = 'success';
      emit(s, `terminal:${s.epoch}`, { kind: 'turn_idle_verified', messageID: s.final });
    } else if (candidate.kind === 'failure') {
      if (s.terminal !== 'failure' || s.result) return;
      s.result = 'failure';
      emit(s, `terminal:${s.epoch}`, { kind: 'terminal_error' });
    } else {
      if (s.pending.get(candidate.id) !== candidate || s.resolved.has(candidate.id)) return;
      if (candidate.messageID && (!s.liveAssistants.has(candidate.messageID) ||
          !rows.some((row) => row.type === 'assistant' && row.id === candidate.messageID))) return;
      emit(s, `request:${candidate.kind}:${candidate.id}`, { kind: candidate.kind, requestID: candidate.id });
    }
  }
  function resetFinal(s) { s.final = ''; s.terminal = ''; }
  function block(s) { s.blocked = true; resetFinal(s); s.revision++; diag('metadata_capacity'); }
  function observe(event) {
    if (disposed) return;
    if (!object(event) || typeof event.type !== 'string' || !object(event.data)) { diag('invalid_event'); return; }
    const type = event.type, p = event.data;
    if (type === 'location.shutdown') { if (sameLocation(event.location, own)) dispose(); return; }
    if (!supported.has(type)) return;
    const sid = type === 'form.created' ? p.form?.sessionID : p.sessionID;
    if (!id(sid) || sid === 'global') { diag('invalid_session'); return; }
    // Invalidation must precede foreign-location filtering and envelope deduplication.
    if (type === 'session.moved' || type === 'session.deleted') {
      const tracked = sessions.get(sid);
      if (tracked) invalidate(tracked);
      return;
    }
    if (event.location !== undefined && !sameLocation(event.location, own)) return;
    if (event.id !== undefined) {
      if (!id(event.id)) { diag('invalid_event'); return; }
      if (eventIDs.has(event.id)) return;
      retainedAdd(eventIDs, event.id, 2048);
    }
    const s = state(sid);
    if (s.ownership === 'rejected' || s.ownership === 'child') return;
    let changed = true;
    if (type === 'session.execution.started') {
      s.epoch++; s.started = true; s.user = ''; s.assistant = ''; resetFinal(s);
      s.retry = false; s.interrupted = false; s.compacting = false; s.compactedUser = ''; s.result = '';
      s.pending.clear(); s.liveAssistants.clear(); s.attempted.clear();
      // Undelivered inbox admissions intentionally survive execution start.
    } else if (type === 'session.inbox.enqueued') {
      if (!id(p.inboxID) || !object(p.item) || typeof p.item.type !== 'string') { diag('invalid_inbox'); return; }
      if (s.admissions.size >= 64 && !s.admissions.has(p.inboxID)) block(s);
      else s.admissions.set(p.inboxID, { type: p.item.type, delivery: p.item.delivery === 'queue' ? 'queue' : 'steer' });
      resetFinal(s);
    } else if (type === 'session.inbox.delivered') {
      if (!id(p.inboxID)) { diag('invalid_inbox'); return; }
      const item = s.admissions.get(p.inboxID); s.admissions.delete(p.inboxID);
      if (!item) { s.blocked = true; diag('unmatched_delivery'); }
      else if (item.type === 'user' && s.started) {
        s.user = p.inboxID; s.assistant = ''; s.liveAssistants.clear(); s.compactedUser = ''; s.retry = false;
      }
      resetFinal(s);
    } else if (type === 'session.inbox.cancelled') {
      if (id(p.inboxID)) s.admissions.delete(p.inboxID);
    } else if (type === 'session.inbox.delivery.changed') {
      const admission = s.admissions.get(p.inboxID);
      if (admission && ['steer', 'queue'].includes(p.delivery)) admission.delivery = p.delivery;
    } else if (type === 'session.step.started') {
      if (!id(p.assistantMessageID) || !liveWork(s)) { diag('unmatched_step'); return; }
      s.assistant = p.assistantMessageID; retainedAdd(s.liveAssistants, p.assistantMessageID, 64);
      s.retry = false; resetFinal(s);
    } else if (type === 'session.step.ended') {
      if (!id(p.assistantMessageID) || p.assistantMessageID !== s.assistant || !liveWork(s)) return;
      s.final = p.finish === 'stop' && !s.retry && !s.compacting ? p.assistantMessageID : '';
    } else if (type === 'session.step.failed' || type === 'session.retry.scheduled') {
      resetFinal(s); s.retry = type === 'session.retry.scheduled';
    } else if (type === 'session.compaction.started') {
      s.compacting = true; resetFinal(s);
    } else if (type === 'session.compaction.ended') {
      if (s.compacting && liveWork(s)) s.compactedUser = s.user;
      s.compacting = false; resetFinal(s);
    } else if (type === 'session.compaction.failed') {
      s.compacting = false; resetFinal(s);
    } else if (type === 'session.execution.interrupted') {
      s.interrupted = true; resetFinal(s); s.pending.clear();
    } else if (type === 'session.execution.succeeded') {
      s.terminal = 'success';
    } else if (type === 'session.execution.failed') {
      if (!object(p.error)) { diag('invalid_failure'); return; }
      // Native interruption is distinct; still reject abort/cancel error discriminants.
      const tag = p.error?._tag ?? p.error?.name ?? p.error?.type;
      if (typeof tag === 'string' && /abort|cancel|interrupt/i.test(tag)) { s.interrupted = true; resetFinal(s); }
      else s.terminal = 'failure';
    } else if (type === 'form.replied' || type === 'form.cancelled' || type === 'permission.replied') {
      const rid = type === 'permission.replied' ? p.requestID : p.id;
      if (!id(rid)) { diag('invalid_request'); return; }
      retainedAdd(s.resolved, rid, 2048); s.pending.delete(rid); resetFinal(s);
    } else if (type === 'form.created' || type === 'permission.asked') {
      let rid, messageID, kind;
      if (type === 'form.created') {
        const form = p.form;
        if (!object(form) || form.metadata?.kind !== 'question' || !id(form.id) ||
            !Array.isArray(form.fields) || form.fields.length === 0 ||
            !id(form.metadata?.tool?.messageID) || !id(form.metadata?.tool?.id)) return;
        rid = form.id; messageID = form.metadata.tool.messageID; kind = 'question_asked';
      } else {
        if (!id(p.id) || typeof p.action !== 'string' || !p.action || !Array.isArray(p.resources) ||
            (p.source !== undefined && (!object(p.source) || p.source.type !== 'tool' || !id(p.source.messageID) || !id(p.source.id)))) return;
        rid = p.id; messageID = p.source?.messageID; kind = 'permission_asked';
      }
      if (!liveWork(s) || s.resolved.has(rid) || s.pending.has(rid)) return;
      if (s.pending.size >= 64) block(s);
      else s.pending.set(rid, { id: rid, messageID, kind });
      resetFinal(s);
    } else changed = false;
    if (changed) s.revision++;
    ensureOwnership(s);
    schedule(s);
  }
  function dispose() {
    if (disposed) return;
    disposed = true; lifecycle++;
    for (const record of requests) { clearTimeout(record.timer); record.controller.abort(); record.stop(); }
    sessions.clear(); eventIDs.clear();
  }
  return { observe, dispose };
}
