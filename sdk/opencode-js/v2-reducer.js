// One native V2 reducer for published push and strict owned-reader adapters.
import { createCore, canonicalID, timestamp } from './observer-core.js';
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
export function createV2Reducer(options, strict = false, fence = () => {}) {
  if (!strict && (typeof options?.emit !== 'function' || typeof options.client?.get !== 'function' ||
      typeof options.client?.context !== 'function' || !location(options.location))) {
    throw new TypeError('emit, native get/context and location required');
  }
  const own = strict ? options.location : { directory: options.location.directory, workspaceID: options.location.workspaceID };
  const maxSessions = bounded(options.maxSessions, 512, 1, 512);
  const timeout = bounded(options.lookupTimeoutMs, 2000, 100, 10000);
  const sessions = new Map(), eventIDs = new Map();
  const core = createCore({ ...options, onDiagnostic(code) {
    diag(code); if (strict && ['job_capacity', 'lookup_capacity', 'lookup_timeout'].includes(code)) fence(code);
  } });
  let checkpoint;
  const now = core.now;
  const diagnosticCodes = new Set();
  let disposed = false, lifecycle = 0;
  const diag = (code) => {
    if (diagnosticCodes.has(code)) return;
    diagnosticCodes.add(code);
    try { options.onDiagnostic?.(code); } catch { /* advisory only */ }
  };
  function invalidate(s) { s.generation++; s.revision++; sessions.delete(s.sid); core.recheck(); }
  const activeOwned = (s) => ['root', 'pending', 'new'].includes(s.ownership) &&
    (s.admissions.size > 0 || (s.started && !s.result && !s.interrupted && (!s.terminal || s.verifying.size > 0)));
  function state(sid) {
    if (sessions.has(sid)) {
      const existing = sessions.get(sid);
      sessions.delete(sid); sessions.set(sid, existing);
      return existing;
    }
    if (sessions.size === maxSessions) {
      if (strict) {
        const ended = [...sessions.values()].find((s) => s.result || s.interrupted || s.blocked);
        if (ended && !core.busy()) invalidate(ended);
        else { fence('session_capacity'); return; }
      }
      const records = strict ? [] : [...sessions.values()];
      const victim = records.find((s) => s.ownership === 'rejected' || s.ownership === 'child') ??
        records.find((s) => !activeOwned(s) && !s.verifying.size) ?? records[0];
      if (victim && activeOwned(victim)) diag('session_capacity');
      if (victim) invalidate(victim);
    }
    const s = { sid, generation: 0, ownership: 'new', epoch: 0, revision: 0, started: false,
      user: '', assistant: '', final: '', retry: false, interrupted: false, compacting: false,
      compactedUser: '', blocked: false, admissionsOverflow: false, terminal: '', result: '', admissions: new Map(),
      pending: new Map(), resolved: new Set(), runID: '', nativeTerminal: undefined, sequence: undefined, nativeScope: undefined, admitted: new Set(), liveAssistants: new Set(), attempted: new Set(), verifying: new Set() };
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
    const controller = new AbortController(), epoch = lifecycle;
    return core.lookup(call, { signal: controller.signal, isCurrent: () => !disposed && epoch === lifecycle,
      metadataDeadline: now() + Math.min(timeout, 2000) });
  }
  function ensureOwnership(s) {
    if (strict || s.ownership !== 'new') return;
    s.ownership = 'pending';
    const token = ownershipToken(s);
    void lookup((signal) => options.client.get({ sessionID: s.sid }, { signal })).then((info) => {
      if (!ownedToken(token)) return;
      if (info === undefined) { s.ownership = 'unverified'; diag('ownership_unverified'); return; }
      if (!object(info) || info.id !== s.sid || (info.parentID !== undefined && !id(info.parentID)) ||
          !sameLocation(info.location, own)) { s.ownership = 'rejected'; diag('ownership_unverified'); return; }
      s.ownership = info.parentID === undefined ? 'root' : 'child';
      schedule(s); // Current metadata only; no event replay and no semantic revision on this token.
    });
  }
  async function context(s, handoff) {
    const rows = await core.lookup((signal) => options.client.context({ sessionID: s.sid }, { signal }), handoff);
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
  async function emit(s, key, fact, handoff) {
    if (disposed || s.ownership !== 'root' || s.admitted.has(key)) return;
    if (s.admitted.size >= 2048) { s.blocked = true; diag('admission_capacity'); return; }
    s.admitted.add(key); // Callback failures/unknown delivery never make an item retryable.
    await core.emit({ version: 1, sessionID: s.sid, turnID: s.user, rootSession: true, ...fact }, handoff);
  }
  function schedule(s) {
    if (disposed || (!strict && s.ownership !== 'root') || !liveWork(s)) return;
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
      const final = s.nativeTerminal;
      const relevant = () => strict && final && ['success', 'failure'].includes(candidate.kind)
        ? ownedToken(token) && s.epoch === token.epoch && !s.blocked && !s.interrupted && s.nativeTerminal === final
        : current(token);
      const work = core.submit(s.sid, relevant, async (handoff) => {
        if (strict) await verifyStrict(s, candidate, handoff, final);
        else await verify(s, candidate, token, handoff);
      }, candidate.ingress ?? s.terminalIngress ?? now());
      if (!work) { s.blocked = true; s.pending.clear(); }
      void work?.finally(() => {
        s.verifying.delete(flight);
        if (!strict && ownedToken(token) && !current(token)) { s.attempted.delete(attempt); schedule(s); }
      });
    }
  }
  async function verify(s, candidate, token, handoff) {
    const rows = await context(s, handoff);
    if (!current(token)) return;
    if (!rows || !liveWork(s) || !associated(s, rows)) return;
    if (candidate.kind === 'success') {
      if (s.terminal !== 'success' || s.result || s.retry || s.compacting || s.pending.size || s.admissions.size || !s.final) return;
      const index = rows.findIndex((row) => row.id === s.final && row.type === 'assistant');
      const answer = rows[index];
      if (!answer || answer.finish !== 'stop' || !answer.completed || answer.failed ||
          rows.slice(index + 1).some((row) => row.type !== 'idle')) return;
      s.result = 'success';
      await emit(s, `terminal:${s.epoch}`, { kind: 'turn_idle_verified', messageID: s.final }, handoff);
    } else if (candidate.kind === 'failure') {
      if (s.terminal !== 'failure' || s.result) return;
      s.result = 'failure';
      await emit(s, `terminal:${s.epoch}`, { kind: 'terminal_error' }, handoff);
    } else {
      if (s.pending.get(candidate.id) !== candidate || s.resolved.has(candidate.id)) return;
      if (candidate.messageID && (!s.liveAssistants.has(candidate.messageID) ||
          !rows.some((row) => row.type === 'assistant' && row.id === candidate.messageID))) return;
      await emit(s, `request:${candidate.kind}:${candidate.id}`, { kind: candidate.kind, requestID: candidate.id }, handoff);
    }
  }
  function matches(proof, run) {
    return object(proof) && proof.sessionID === run.sessionID && proof.turnID === run.turnID &&
      proof.location === own && proof.rootSession === true;
  }
  async function verifyStrict(s, candidate, handoff, final) {
    const run = Object.freeze({ sessionID: s.sid, turnID: s.runID, userID: s.user,
      location: own, observedLocation: s.nativeScope, terminalEventID: final?.id, terminalCreated: final?.created, terminalSequence: final?.sequence });
    const scope = await core.lookup((signal) => options.native.session(run, signal), handoff);
    if (!handoff.isCurrent() || !matches(scope, run)) { diag('root_execution_unverified'); return; }
    s.ownership = 'root';
    const terminal = candidate.kind === 'success' || candidate.kind === 'failure';
    if (terminal) {
      if (s.compacting || !candidate.messageID || !final) { diag('terminal_assistant_unverified'); return; }
      const answer = await core.lookup((signal) => options.native.assistant({ ...run, messageID: candidate.messageID }, signal), handoff);
      if (!handoff.isCurrent() || !matches(answer, run) || answer.messageID !== candidate.messageID ||
          answer.role !== 'assistant' || answer.summary !== false || answer.final !== true ||
          answer.outcome !== (candidate.kind === 'failure' ? 'error' : 'success')) {
        diag('terminal_assistant_unverified'); return;
      }
    } else if (candidate.kind === 'question_asked') {
      const source = await core.lookup((signal) => options.native.questionSource({ ...run, requestID: candidate.id,
        messageID: candidate.messageID, callID: candidate.callID }, signal), handoff);
      if (!handoff.isCurrent() || !matches(source, run) || source.requestID !== candidate.id ||
          source.messageID !== candidate.messageID || source.callID !== candidate.callID || source.role !== 'assistant' ||
          source.tool !== 'question' || source.summary !== false) { diag('question_source_unverified'); return; }
    }
    const kind = terminal ? candidate.kind === 'success' ? 'turn_idle_verified' : 'terminal_error' : candidate.kind;
    const envelope = terminal ? final : candidate;
    let snapshot;
    const takeSnapshot = async () => {
      if (kind === 'question_asked') return checkpoint.verify(handoff);
      const pending = await core.lookup((signal) => options.native.currentPermission({ ...run, requestID: candidate.id }, signal), handoff);
      return handoff.isCurrent() && matches(pending, run) && pending.pending === true && pending.requestID === candidate.id &&
        pending.messageID === candidate.messageID && pending.callID === candidate.callID;
    };
    const revalidate = terminal ? undefined : async () => {
      snapshot ??= takeSnapshot();
      return Boolean(await snapshot && handoff.isCurrent());
    };
    const fact = { kind, ...(kind === 'turn_idle_verified' ? { messageID: candidate.messageID } : {}),
      ...(!terminal ? { requestID: candidate.id } : {}), provenance: { generation: 'v2',
        observationID: canonicalID('v2', s.sid, s.runID, kind, terminal ? candidate.messageID : candidate.id),
        nativeEventID: envelope.eventID ?? envelope.id, nativeTime: envelope.created, timeBasis: 'envelope_created' } };
    if (terminal) s.result = candidate.kind;
    // All metadata precedes ONE final marker/pending snapshot, including callback capacity waits.
    await emit(s, terminal ? `terminal:${s.epoch}` : `request:${kind}:${candidate.id}`, fact, { ...handoff, revalidate });
  }
  function resetFinal(s) { s.final = ''; s.terminal = ''; }
  function block(s) { s.blocked = true; resetFinal(s); s.revision++; diag('metadata_capacity'); }
  function suppress(sid, code) {
    const s = sessions.get(sid);
    if (s) { s.blocked = true; s.pending.clear(); s.revision++; }
    diag(code);
  }
  function observe(event) {
    const ingress = now();
    try { ingest(event, ingress); } finally { if (strict) core.recheck(); }
  }
  function ingest(event, ingress) {
    if (disposed) return;
    if (!object(event) || typeof event.type !== 'string' || !object(event.data)) {
      if (strict && supported.has(event?.type)) fence('invalid_native_envelope');
      else diag('invalid_event');
      return;
    }
    const type = event.type; let p = event.data, correlation;
    if (strict && checkpoint?.consume(event)) return;

    if (type === 'location.shutdown') {
      if (strict) {
        let scope; try { scope = options.native.correlate(event)?.location; } catch {}
        if (scope === undefined || scope === own) fence('location_shutdown');
        return;
      }
      if (sameLocation(event.location, own)) dispose(); return;
    }
    if (!supported.has(type) && !(strict && type.startsWith('session.'))) return;
    if (strict) {
      try { correlation = options.native.correlate(event); } catch {
        if (id(p.sessionID)) suppress(p.sessionID, 'native_mapping_failed');
        else fence('native_mapping_failed');
        return;
      }
      if (!correlation) return; // Explicitly irrelevant native event, including global elicitation.
      const sid = correlation.sessionID;
      if (sid === 'global') return;
      if (!id(sid)) { fence('native_correlation_unverified'); return; }
      if (!id(event.id) || !timestamp(event.created)) { suppress(sid, 'invalid_native_envelope'); return; }
      p = { ...p, sessionID: sid };
      if (correlation.messageID) p.assistantMessageID = correlation.messageID;

    }
    const sid = strict ? correlation.sessionID : type === 'form.created' ? p.form?.sessionID : p.sessionID;
    if (!id(sid) || sid === 'global') { diag('invalid_session'); return; }
    // Invalidation must precede foreign-location filtering and envelope deduplication.
    if (type === 'session.moved' || type === 'session.deleted') {
      const tracked = sessions.get(sid);
      if (tracked) {
        if (strict) { tracked.blocked = true; tracked.pending.clear(); tracked.nativeScope = correlation.location; tracked.revision++; }
        else invalidate(tracked);
      }
      return;
    }
    if (!strict && event.location !== undefined && !sameLocation(event.location, own)) return;
    if (strict && correlation.location !== undefined && correlation.location !== own) {
      const tracked = sessions.get(sid); if (tracked) { tracked.blocked = true; tracked.pending.clear(); tracked.revision++; }
      diag('scope_mismatch'); return;
    }
    if (event.id !== undefined) {
      if (!id(event.id)) { diag('invalid_event'); return; }
      if (!strict && eventIDs.has(event.id)) return;
      if (!strict) { eventIDs.set(event.id, true); if (eventIDs.size > 2048) eventIDs.delete(eventIDs.keys().next().value); }
    }
    const s = state(sid);
    if (!s) return;
    if (s.ownership === 'rejected' || s.ownership === 'child') return;
    if (strict) {
      if (type.startsWith('session.step.') && !id(correlation.messageID) && correlation.compaction !== true) {
        suppress(sid, 'native_correlation_unverified'); return;
      }
      const badCorrelation = ['location', 'messageID', 'requestID', 'callID'].some((key) =>
        correlation[key] !== undefined && !id(correlation[key]));
      const seq = correlation.sequence;
      if (badCorrelation || (seq !== undefined && (!object(seq) || seq.aggregate !== sid ||
          !Number.isSafeInteger(seq.seq) || seq.seq < 0 || (seq.version !== undefined &&
          (!Number.isSafeInteger(seq.version) || seq.version < 0)))) ||
          (correlation.compaction !== undefined && typeof correlation.compaction !== 'boolean')) {
        s.blocked = true; s.pending.clear(); s.revision++; diag('native_correlation_unverified'); return;
      }
      const nativeSequence = seq ? { aggregate: seq.aggregate, seq: seq.seq, ...(seq.version !== undefined ? { version: seq.version } : {}) } : undefined;
      if (s.nativeScope !== undefined && correlation.location === undefined && s.nativeScope !== own) {
        s.blocked = true; diag('scope_mismatch'); return;
      }
      const fingerprint = JSON.stringify([type, event.created, sid, correlation.location, correlation.messageID,
        correlation.requestID, correlation.callID, correlation.compaction, nativeSequence, id(p.inboxID) ? p.inboxID : undefined,
        id(p.item?.type) ? p.item.type : undefined, p.item?.delivery === 'queue', p.delivery === 'queue', p.finish === 'stop']);
      if (eventIDs.has(event.id)) {
        if (eventIDs.get(event.id) === fingerprint) return;
        const original = sessions.get(JSON.parse(eventIDs.get(event.id))[2]);
        if (original) { original.blocked = true; original.pending.clear(); original.revision++; }
        s.blocked = true; s.pending.clear(); s.revision++; diag('native_identity_contradiction'); return;
      }
      eventIDs.set(event.id, fingerprint);
      if (eventIDs.size > 2048) eventIDs.delete(eventIDs.keys().next().value);
      if (s.nativeTerminal && ['session.inbox.enqueued', 'session.inbox.delivered', 'session.step.started'].includes(type)) {
        s.blocked = true; s.pending.clear(); s.revision++;
      }
      if (type.startsWith('session.') && (!object(seq) || seq.aggregate !== sid || !Number.isSafeInteger(seq.seq) || seq.seq < 0)) {
        s.blocked = true; s.pending.clear(); s.revision++; diag('invalid_native_sequence'); return;
      }
      if (seq) {
        if (s.sequence && seq.seq <= s.sequence.seq) {
          s.blocked = true; s.pending.clear(); s.revision++; diag('native_sequence_contradiction'); return;
        }
        s.sequence = { ...nativeSequence, id: event.id, type, created: event.created,
          messageID: correlation.messageID, compaction: correlation.compaction };
      }
      if (correlation.location !== undefined) s.nativeScope = correlation.location;
      if (['session.retry.scheduled', 'session.compaction.started', 'session.execution.interrupted'].includes(type) ||
          correlation.compaction === true || type === 'session.execution.started') {
        for (const key of s.pending.keys()) s.resolved.add(key);
        s.pending.clear();
      }
      if (correlation.compaction === true) s.compacting = true;
      if (s.resolved.size >= 256) { s.blocked = true; diag('request_capacity'); return; }
    }
    let changed = true;
    if (type === 'session.execution.started') {
      // Retry inconclusive ownership once on fresh work, never replay old facts.
      if (s.ownership === 'unverified') s.ownership = 'new';
      s.runID = strict ? event.id : ''; s.nativeTerminal = undefined;
      s.epoch++; s.started = true; s.user = ''; s.assistant = ''; resetFinal(s);
      s.retry = false; s.interrupted = false; s.compacting = false; s.compactedUser = ''; s.result = '';
      s.blocked = s.admissionsOverflow;
      s.admitted.clear(); // Previous epochs cannot be admitted by the current semantic token.
      s.pending.clear(); s.liveAssistants.clear(); s.attempted.clear();
      // Undelivered inbox admissions intentionally survive execution start.
    } else if (type === 'session.inbox.enqueued') {
      if (!id(p.inboxID) || !object(p.item) || !id(p.item.type)) { diag('invalid_inbox'); return; }
      if (s.admissions.size >= 64 && !s.admissions.has(p.inboxID)) { s.admissionsOverflow = true; block(s); }
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
      if (strict && correlation.compaction === true) { s.compacting = true; resetFinal(s); s.revision++; return; }
      if (!id(p.assistantMessageID) || !liveWork(s)) { diag('unmatched_step'); return; }
      if (strict) s.compacting = false;
      if (strict && s.assistant && s.assistant !== p.assistantMessageID) {
        for (const key of s.pending.keys()) s.resolved.add(key);
        s.pending.clear();
      }
      s.assistant = p.assistantMessageID; retainedAdd(s.liveAssistants, p.assistantMessageID, 64);
      s.retry = false; resetFinal(s);
    } else if (type === 'session.step.ended') {
      if (strict && correlation.compaction === true) { s.compacting = true; resetFinal(s); s.revision++; return; }
      if (!id(p.assistantMessageID) || p.assistantMessageID !== s.assistant || !liveWork(s)) return;
      s.final = (strict || p.finish === 'stop') && !s.retry && !s.compacting ? p.assistantMessageID : '';
    } else if (type === 'session.step.failed' || type === 'session.retry.scheduled') {
      resetFinal(s); s.retry = type === 'session.retry.scheduled';
      if (strict && type === 'session.step.failed' && !s.compacting && p.assistantMessageID === s.assistant && id(p.assistantMessageID)) s.final = p.assistantMessageID;
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
      if (strict && s.nativeTerminal) { s.blocked = true; s.revision++; diag('native_terminal_contradiction'); return; }
      s.terminal = 'success';
    } else if (type === 'session.execution.failed') {
      if (strict && s.nativeTerminal) { s.blocked = true; s.revision++; diag('native_terminal_contradiction'); return; }
      if (!strict && !object(p.error)) { diag('invalid_failure'); return; }
      // Native interruption is distinct; still reject abort/cancel error discriminants.
      const tag = p.error?._tag ?? p.error?.name ?? p.error?.type;
      if (typeof tag === 'string' && /abort|cancel|interrupt/i.test(tag)) { s.interrupted = true; resetFinal(s); }
      else s.terminal = 'failure';
    } else if (type === 'form.replied' || type === 'form.cancelled' || type === 'permission.replied') {
      const rid = strict ? correlation.requestID : type === 'permission.replied' ? p.requestID : p.id;
      if (!id(rid)) { diag('invalid_request'); return; }
      retainedAdd(s.resolved, rid, strict ? 256 : 2048); s.pending.delete(rid); resetFinal(s);
    } else if (type === 'form.created' || type === 'permission.asked') {
      let rid, messageID, kind;
      if (strict) {
        rid = correlation.requestID; messageID = correlation.messageID;
        kind = type === 'form.created' ? 'question_asked' : 'permission_asked';
        if (!id(rid) || !id(messageID) || !id(correlation.callID) || messageID !== s.assistant || !s.liveAssistants.has(messageID)) {
          s.blocked = true; s.pending.clear(); s.revision++; diag('request_identity_unverified'); return;
        }
      } else if (type === 'form.created') {
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
      if (strict && s.pending.has(rid)) {
        const previous = s.pending.get(rid);
        if (previous.eventID !== event.id || previous.created !== event.created || previous.messageID !== messageID ||
            previous.callID !== correlation.callID) { suppress(sid, 'request_identity_ambiguous'); }
        return;
      }
      if (strict && type === 'form.created' && (!checkpoint?.ready() || !options.native.questionSource)) {
        s.resolved.add(rid); diag(!checkpoint?.ready() ? 'form_checkpoint_unready' : 'question_source_unavailable'); return;
      }
      if (strict && type === 'permission.asked' && !options.native.currentPermission) { diag('permission_pending_authority_unavailable'); return; }
      if (!liveWork(s) || s.resolved.has(rid) || s.pending.has(rid) || (strict && (s.nativeTerminal || s.compacting || !id(correlation.callID)))) return;
      if (s.pending.size >= 64 || (strict && s.pending.size + s.resolved.size >= 256)) block(s);
      else s.pending.set(rid, { id: rid, messageID, kind, ingress,
        ...(strict ? { callID: correlation.callID, eventID: event.id, created: event.created } : {}) });
      resetFinal(s);
    } else changed = false;
    if (strict && ['session.execution.succeeded', 'session.execution.failed'].includes(type)) {
      s.pending.clear();
      s.nativeTerminal = Object.freeze({ id: event.id, created: event.created, sequence: Object.freeze({ ...s.sequence }) }); s.terminalIngress = ingress;
      if (s.compacting) { s.terminal = ''; diag('compaction_terminal_suppressed'); }
    }
    // A terminal boundary ends an over-budget period. Missing queued metadata
    // subsequently fails closed at delivery, while fresh observed work recovers.
    if (s.admissionsOverflow && ['session.execution.succeeded', 'session.execution.failed', 'session.execution.interrupted'].includes(type)) {
      s.admissionsOverflow = false; s.admissions.clear();
    }
    if (changed) s.revision++;
    ensureOwnership(s);
    schedule(s);
  }
  function dispose() {
    if (disposed) return;
    disposed = true; lifecycle++;
    core.invalidate();
    sessions.clear(); eventIDs.clear();
  }
  return { observe, dispose, core, diag,
    setCheckpoint(value) { checkpoint = value; },
    invalidate() {
      lifecycle++; core.invalidate(); checkpoint?.invalidate();
      for (const s of sessions.values()) { s.blocked = true; s.pending.clear(); s.revision++; }
    },
    done: () => core.drain(),
  };
}
