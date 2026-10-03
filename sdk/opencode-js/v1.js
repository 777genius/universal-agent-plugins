// V1 native reducer; root export preserves this API.
import { createCore, canonicalID, timestamp } from './observer-core.js';
const object = (x) => x !== null && typeof x === 'object' && !Array.isArray(x);
const id = (x) => typeof x === 'string' && x.length > 0 && !/[\u0000-\u001f]/u.test(x) && new TextEncoder().encode(x).length <= 256;
const typeOK = (x) => typeof x === 'string' && /^[a-z][a-z0-9.-]{0,79}$/.test(x);
// Native session lifecycle uses info.id; message lifecycle uses info.sessionID.
// An arbitrary/global info.id is not a session identity.
const nativeSessionID = (event) => ['session.updated', 'session.deleted'].includes(event?.type)
  ? event?.properties?.info?.id ?? event?.properties?.sessionID
  : event?.properties?.sessionID ?? event?.properties?.info?.sessionID;

/** @param {import('./v1.d.ts').ObserverOptions} options */
export function createObserver(options) {
  if (typeof options?.emit !== 'function' || typeof options.client?.session?.messages !== 'function' ||
      typeof options.client?.session?.get !== 'function') {
    throw new TypeError('emit, messages and get required');
  }
  const limit = Math.max(2, Math.min(100, Number.isInteger(options.messageLimit) ? options.messageLimit : 30));
  const maxSessions = Math.max(1, Math.min(512, Number.isInteger(options.dedupLimit) ? options.dedupLimit : 512));
  const timeoutMs = Math.max(100, Math.min(2000, Number.isInteger(options.lookupTimeoutMs) ? options.lookupTimeoutMs : 2000));
  const sessions = new Map(), unknownSeen = new Set();
  let activeJobs = 0, disposed = false;
  const lifetime = new AbortController(), handoffs = new Set(), core = createCore({ ...options, lookupTimeoutMs: timeoutMs,
    onDiagnostic: (reason) => options.onDiagnostic?.(reason === 'lookup_capacity' ? 'messages lookup capacity exceeded' :
      ['lookup_timeout', 'lookup_failed'].includes(reason) ? 'lookup failed or timed out' : reason) }), jobsBySession = new Map();
  const now = core.now;
  const strict = typeof options.runtimeEligibility === 'function';
  let runtime = 'supported';
  if (strict) { try { runtime = options.runtimeEligibility(); } catch { runtime = 'unverified'; } }
  const diag = (reason) => { try { options.onDiagnostic?.(reason); } catch { /* advisory sink */ } };
  const state = (sid) => {
    if (!sessions.has(sid)) {
      const fresh = {
        user: '', userCreated: undefined, seenUsers: new Set(), assistant: '', failedAssistant: '',
        overflowPending: false, idleObserved: false, retry: false, retryAssistant: '', cancelled: false,
        questions: new Set(), permissions: new Set(), requestBindings: new Map(), resolved: new Set(), admitted: new Set(),
        turnEpoch: 0, revision: 0, errorPending: 0, rootSession: undefined,
        activeAssistant: '', activeCompleted: false, ambiguousAssistant: false, sourceError: undefined, compaction: false, activeCreated: undefined, compactionFloor: undefined,
      };
      if (strict && sessions.size >= maxSessions) {
        for (const [key, value] of sessions) {
          if (!jobsBySession.has(key) && (value.cancelled || value.admitted.has('idle') || value.admitted.has('error'))) {
            sessions.delete(key); break;
          }
        }
        if (sessions.size >= maxSessions) { diag('session_capacity'); return { ...fresh, cancelled: true }; }
      }
      sessions.set(sid, fresh);
      if (sessions.size > maxSessions) sessions.delete(sessions.keys().next().value);
    }
    return sessions.get(sid);
  };
  async function emit(fact, key, session, nativeTime, timeBasis, revalidate, born = now(), nativeMessageID) {
    if (disposed) return;
    if (strict && (!timestamp(nativeTime) || fact.rootSession !== true || !session?.scopeMatched)) { core.diag('native_provenance_unverified'); return; }
    const controller = new AbortController();
    const epoch = session?.turnEpoch, revision = session?.revision;
    const requestSet = fact.kind === 'question_asked' ? session?.questions : session?.permissions;
    const relevant = () => !disposed && !controller.signal.aborted && now() - born <= 30000 && (!session || (sessions.get(fact.sessionID) === session &&
      session.turnEpoch === epoch && (fact.requestID ? requestSet.has(fact.requestID) && !session.cancelled && !session.compaction && !session.ambiguousAssistant && !session.sourceError : session.revision === revision)));
    if (!relevant()) return;
    if (strict && timestamp(nativeTime)) fact.provenance = {
      generation: 'v1', observationID: canonicalID('v1', fact.sessionID, fact.turnID, fact.kind, fact.requestID ?? fact.messageID ?? nativeMessageID),
      nativeTime, timeBasis, ...(nativeMessageID ? { nativeMessageID } : {}),
    };
    // Legacy candidates without native provenance cannot pass AN admission.
    const handoff = { signal: controller.signal, isCurrent: relevant, revalidate, ingressMonotonicMs: born, clockID: core.clockID, metadataDeadline: born + timeoutMs };
    const timer = setTimeout(() => controller.abort(), Math.max(0, 30000 - (now() - born)));
    const owned = { controller, relevant };
    handoffs.add(owned);
    const admitted = session ? session.admitted : unknownSeen;
    if (admitted.has(key)) { clearTimeout(timer); handoffs.delete(owned); return; }
    admitted.add(key); // Admit before the external callback: its result may be ambiguous.
    if (!session && admitted.size > maxSessions) admitted.delete(admitted.values().next().value);
    try { await core.emit({ version: 1, ...fact }, handoff); }
    catch { diag('observed event callback failed'); }
    finally { clearTimeout(timer); handoffs.delete(owned); }
  }
  async function boundedLookup(call, deadline = now() + timeoutMs) {
    return core.lookup(call, { signal: lifetime.signal,
      isCurrent: () => !disposed, metadataDeadline: deadline });
  }
  async function rootStatus(sid, deadline) {
    const s = state(sid);
    if (!strict && typeof s.rootSession === 'boolean') return s.rootSession;
    const result = await boundedLookup((signal) => options.client.session.get({ path: { id: sid }, signal }), deadline);
    const info = result?.data ?? result;
    if (!object(info) || info.id !== sid || (info.parentID !== undefined && !id(info.parentID))) {
      diag('invalid session ancestry'); return;
    }
    s.rootSession = info.parentID === undefined;
    s.scopeMatched = typeof options.location === 'string' && info.directory === options.location;
    return s.rootSession;
  }
  async function latest(sid, deadline) {
    const result = await boundedLookup((signal) => options.client.session.messages({
      path: { id: sid }, query: { limit }, signal,
    }), deadline);
    const rows = Array.isArray(result) ? result : result?.data;
    if (!Array.isArray(rows) || rows.length > limit) { diag('invalid messages lookup'); return; }
    const messages = rows.map((x) => x?.info ?? x);
    if (!messages.every((m) => object(m) && id(m.id) && m.sessionID === sid && ['user', 'assistant'].includes(m.role))) {
      diag('invalid or mismatched messages'); return;
    }
    return messages;
  }
  function currentTurn(messages, uid) {
    const lastUser = messages.findLast((m) => m.role === 'user');
    if (lastUser) return lastUser.id === uid;
    // A long turn can push its user message outside the bounded window.
    return messages.at(-1)?.role === 'assistant' && messages.at(-1).parentID === uid;
  }
  async function idle(sid, born, deadline) {
    const s = state(sid), revision = s.revision, uid = s.user;
    s.idleObserved = true;
    const failed = Boolean(s.failedAssistant || s.overflowPending);
    if (activeJobs > 256 || !uid || s.compaction || s.ambiguousAssistant || (!s.assistant && !failed) ||
        s.admitted.has('idle') || s.admitted.has('error') || (s.retry && !failed) || s.cancelled ||
        s.errorPending || s.questions.size || s.permissions.size) return;
    const messages = await latest(sid, deadline);
    if (!messages || sessions.get(sid) !== s || revision !== s.revision || s.user !== uid ||
        (s.retry && !failed) || s.cancelled || s.errorPending || s.questions.size || s.permissions.size) return;
    if (!currentTurn(messages, uid)) return;
    const answer = messages.at(-1);
    if (strict && answer?.path?.cwd !== options.location) { diag('scope_unverified'); return; }
    if (s.failedAssistant || s.overflowPending) {
      if (strict && !s.sourceError) return;
      if (!answer || answer.role !== 'assistant' || answer.parentID !== uid ||
          (s.failedAssistant && answer.id !== s.failedAssistant) || (strict && !timestamp(answer.time?.completed)) || !object(answer.error) ||
          (s.sourceError && (answer.id !== s.sourceError.messageID || answer.error.name !== s.sourceError.name)) || s.compaction ||
          /abort|cancel/i.test(String(answer.error.name ?? ''))) return;
      if (s.overflowPending && !s.failedAssistant && answer.error.name !== 'ContextOverflowError') return;
      const rootSession = await rootStatus(sid, deadline);
      if (rootSession === undefined || sessions.get(sid) !== s || revision !== s.revision || s.user !== uid || s.cancelled) return;
      s.cancelled = true; s.revision++;
      await emit({ kind: 'terminal_error', sessionID: sid, turnID: uid, rootSession }, 'error', s, answer.time?.created, 'assistant_created_lower_bound', undefined, born, answer.id);
      return;
    }
    if (!answer || answer.role !== 'assistant' || answer.id !== s.assistant || answer.parentID !== uid ||
        answer.finish !== 'stop' || !Number.isFinite(answer.time?.completed) || answer.error != null) return;
    const rootSession = await rootStatus(sid, deadline);
    if (rootSession === undefined || sessions.get(sid) !== s || revision !== s.revision || s.user !== uid || s.cancelled || s.errorPending) return;
    await emit({ kind: 'turn_idle_verified', sessionID: sid, turnID: uid, messageID: answer.id, rootSession }, 'idle', s, answer.time?.completed, 'assistant_completed', undefined, born);
  }
  async function observe(event, born, deadline) {
    if (!object(event) || !typeOK(event.type) || !object(event.properties)) { diag('invalid native event'); return; }
    const { type, properties: p } = event;
    if (type === 'message.updated') {
      const m = p.info;
      if (!object(m) || !id(m.id) || !id(m.sessionID) || !['user', 'assistant'].includes(m.role)) { diag('invalid message.updated'); return; }
      const s = state(m.sessionID);
      if (m.role === 'user' && m.id !== s.user) {
        const created = m.time?.created;
        // Summary/diff work can republish an older user after a newer turn starts.
        if (s.seenUsers.has(m.id) || (Number.isFinite(created) &&
            Number.isFinite(s.userCreated) && created < s.userCreated)) return;
        s.seenUsers.add(m.id);
        if (s.seenUsers.size > 512) s.seenUsers.delete(s.seenUsers.values().next().value);
        s.userCreated = Number.isFinite(created) ? created : undefined;
        s.user = m.id; s.assistant = ''; s.retry = false; s.retryAssistant = ''; s.cancelled = false;
        s.failedAssistant = ''; s.overflowPending = false; s.idleObserved = false;
        s.questions.clear(); s.permissions.clear(); s.requestBindings.clear(); s.resolved.clear(); s.admitted.clear();
        s.activeAssistant = ''; s.activeCreated = undefined; s.activeCompleted = false; s.ambiguousAssistant = false; s.sourceError = undefined; s.compaction = false;
        s.turnEpoch++; s.revision++;
      }
      if (m.role === 'assistant' && m.parentID === s.user && m.summary !== true) {
        if (strict && s.activeAssistant && s.activeAssistant !== m.id) {
          for (const requestID of [...s.questions, ...s.permissions]) s.resolved.add(requestID);
          s.questions.clear(); s.permissions.clear(); s.revision++;
        }
        if (strict && s.activeAssistant && s.activeAssistant !== m.id && !s.activeCompleted) s.ambiguousAssistant = true;
        if (s.compaction && timestamp(m.time?.created) && timestamp(s.compactionFloor) && m.time.created > s.compactionFloor && m.id !== s.activeAssistant) {
          s.compaction = false; s.sourceError = undefined; s.ambiguousAssistant = false;
        }
        s.activeAssistant = m.id; s.activeCreated = m.time?.created; s.activeCompleted = Number.isFinite(m.time?.completed);
      }
      if (strict && m.role === 'assistant' && m.summary === true && m.parentID === s.user) {
        s.compactionFloor = m.time?.created;
        s.compaction = true; s.questions.clear(); s.permissions.clear(); s.revision++;
      }
      if (m.role === 'assistant' && (!strict || m.summary !== true) && m.parentID === s.user && m.finish === 'stop' &&
          Number.isFinite(m.time?.completed) && m.error == null && m.id !== s.retryAssistant) {
        s.assistant = m.id; s.failedAssistant = ''; s.overflowPending = false; s.retry = false; s.revision++;
      }
      if (m.role === 'assistant' && (!strict || m.summary !== true) && m.parentID === s.user && m.error != null) {
        s.assistant = ''; s.failedAssistant = m.id; s.revision++;
        // Native error-before-idle-before-final-update ordering must not lose the final fact.
        s.questions.clear(); s.permissions.clear();
        if (strict && s.idleObserved && !s.errorPending) await idle(m.sessionID, born, deadline);
      }
      return;
    }
    if (type === 'session.status') {
      if (!id(p.sessionID) || !object(p.status) || !['busy', 'idle', 'retry'].includes(p.status.type)) { diag('invalid session.status'); return; }
      if (p.status.type === 'busy') {
        const s = state(p.sessionID); s.idleObserved = false; s.revision++;
      }
      if (p.status.type === 'retry') {
        const s = state(p.sessionID); s.retryAssistant = s.assistant; s.assistant = '';
        s.failedAssistant = ''; s.overflowPending = false; s.sourceError = undefined; s.idleObserved = false; s.retry = true;
        for (const requestID of [...s.questions, ...s.permissions]) s.resolved.add(requestID);
        s.questions.clear(); s.permissions.clear(); s.revision++;
      }
      if (p.status.type === 'idle') await idle(p.sessionID, born, deadline);
      return;
    }
    if (type === 'session.idle') { if (!id(p.sessionID)) { diag('invalid session.idle'); return; } await idle(p.sessionID, born, deadline); return; }
    if (type === 'question.asked' || type === 'permission.asked') {
      const requestMessageID = p.messageID ?? p.tool?.messageID;
      if (!id(p.sessionID) || !id(p.id) || (requestMessageID !== undefined && !id(requestMessageID))) { diag('invalid request'); return; }
      if (type === 'question.asked' && (!Array.isArray(p.questions) || p.questions.length === 0)) { diag('invalid question shape'); return; }
      if (type === 'permission.asked' && (typeof p.permission !== 'string' || !p.permission)) { diag('invalid permission shape'); return; }
      const s = state(p.sessionID), uid = s.user, epoch = s.turnEpoch;
      if (!uid || (strict && (s.cancelled || s.compaction || s.sourceError || s.admitted.has('idle') || s.admitted.has('error'))) || s.resolved.has(p.id)) { diag('unmatched or resolved request'); return; }
      const pending = type === 'question.asked' ? s.questions : s.permissions;
      if (s.questions.size + s.permissions.size + s.resolved.size >= 256 || (strict && s.requestBindings.size >= 256)) {
        s.cancelled = true; s.questions.clear(); s.permissions.clear(); s.revision++; diag('request_capacity'); return;
      }
      const assistantAtIngress = s.activeAssistant, createdAtIngress = s.activeCreated;
      if (strict) {
        const binding = JSON.stringify([type, requestMessageID, p.tool?.callID, createdAtIngress]);
        if (s.requestBindings.has(p.id)) {
          if (s.requestBindings.get(p.id) !== binding) {
            s.cancelled = true; s.questions.clear(); s.permissions.clear(); s.revision++; diag('request_identity_ambiguous');
          }
          return;
        }
        s.requestBindings.set(p.id, binding);
      }
      pending.add(p.id); s.revision++;
      const messages = await latest(p.sessionID, deadline);
      if (sessions.get(p.sessionID) !== s || s.turnEpoch !== epoch || s.user !== uid || !pending.has(p.id) || s.resolved.has(p.id)) return;
      if (!messages || !currentTurn(messages, uid)) { pending.delete(p.id); diag('unmatched request turn'); return; }
      if (requestMessageID && requestMessageID !== uid && !messages.some((m) => m.role === 'assistant' && m.id === requestMessageID && m.parentID === uid)) {
        pending.delete(p.id); diag('unmatched request message'); return;
      }
      const rootSession = await rootStatus(p.sessionID, deadline);
      if (rootSession === undefined || sessions.get(p.sessionID) !== s || s.turnEpoch !== epoch || s.user !== uid || !pending.has(p.id) || s.resolved.has(p.id)) return;
      const nativeAssistant = messages.find((m) => m.role === 'assistant' && m.id === requestMessageID && m.parentID === uid);
      if (strict && (nativeAssistant?.path?.cwd !== options.location || nativeAssistant.summary === true ||
          s.ambiguousAssistant || messages.findLast((m) => m.role === 'assistant' && m.summary !== true)?.id !== requestMessageID)) { diag('scope_unverified'); return; }
      // Actual V1 root client has no pending namespaces. Only a qualified
      // host synchronous callback binding can authorize future-live attention.
      const authority = options.callbackAuthority === 'qualified_native_sync';
      const nativeBound = authority && requestMessageID === assistantAtIngress && assistantAtIngress === s.activeAssistant && id(p.tool?.callID) &&
        timestamp(createdAtIngress) && nativeAssistant?.time?.created === createdAtIngress &&
        nativeAssistant?.summary !== true && !s.compaction && !s.sourceError && !s.ambiguousAssistant;
      if (strict && !nativeBound) { diag('callback_attention_authority_unverified'); return; }
      s.assistant = ''; s.revision++;
      await emit({ kind: type === 'question.asked' ? 'question_asked' : 'permission_asked', sessionID: p.sessionID,
        turnID: uid, requestID: p.id, rootSession }, `${type}:${p.id}`, s,
        strict && nativeBound ? createdAtIngress : undefined, 'assistant_created_lower_bound', undefined, born);
      return;
    }
    if (type === 'question.replied' || type === 'question.rejected' || type === 'permission.replied') {
      if (!id(p.sessionID) || !id(p.requestID)) { diag('invalid resolution'); return; }
      const s = state(p.sessionID);
      const pending = type.startsWith('question.') ? s.questions : s.permissions;
      const current = pending.delete(p.requestID);
      s.resolved.add(p.requestID);
      if (s.resolved.size > 256) { s.cancelled = true; s.questions.clear(); s.permissions.clear(); s.revision++; diag('request_capacity'); }
      if (current) { s.assistant = ''; s.revision++; } // Require a later final update only for a current request.
      return;
    }
    if (strict && ['session.updated', 'session.deleted'].includes(type) && id(nativeSessionID(event))) {
      const sid = nativeSessionID(event), s = state(sid), info = p.info;
      // Native Session.touch/title updates publish the complete next Info.
      // An unchanged root/location is not a turn transition: keep its revision
      // and pending work. This snapshot grants no authority; strict rootStatus
      // still reads public session.get after the final messages checkpoint.
      if (type === 'session.updated' && object(info) && info.id === sid &&
          (p.sessionID === undefined || p.sessionID === sid) && info.parentID === undefined &&
          typeof options.location === 'string' && info.directory === options.location &&
          object(info.time) &&
          Number.isSafeInteger(info.time.created) && info.time.created >= 0 &&
          Number.isSafeInteger(info.time.updated) && info.time.updated >= 0 &&
          info.time.archived === undefined && info.time.compacting === undefined && info.revert === undefined) return;
      s.rootSession = undefined; s.scopeMatched = false; s.cancelled = true;
      s.questions.clear(); s.permissions.clear(); s.revision++; return;
    }
    if (strict && type === 'session.compacted') {
      if (id(p.sessionID)) { const s = state(p.sessionID); s.compaction = true; s.sourceError = undefined; s.questions.clear(); s.permissions.clear(); s.revision++; }
      return;
    }
    if (type === 'session.error') {
      if (!id(p.sessionID) || !object(p.error) || (p.messageID !== undefined && !id(p.messageID))) { diag('invalid session.error'); return; }
      const s = state(p.sessionID), uid = s.user, epoch = s.turnEpoch;
      if (!uid || s.admitted.has('idle')) return;
      if (!strict) {
        s.errorPending++; s.revision++;
        const messages = await latest(p.sessionID, deadline); s.errorPending--;
        if (!messages || sessions.get(p.sessionID) !== s || s.turnEpoch !== epoch || s.user !== uid ||
            s.admitted.has('idle') || !currentTurn(messages, uid)) return;
        if (p.messageID && !messages.some((m) => m.role === 'assistant' && m.id === p.messageID && m.parentID === uid)) {
          diag('unmatched error message'); return;
        }
        const rootSession = await rootStatus(p.sessionID, deadline);
        if (rootSession === undefined || sessions.get(p.sessionID) !== s || s.turnEpoch !== epoch || s.user !== uid || s.admitted.has('idle')) return;
        if (p.error.name === 'ContextOverflowError') {
          s.overflowPending = true; s.revision++;
          if (s.idleObserved) await idle(p.sessionID, born, deadline);
          return;
        }
        s.cancelled = true; s.revision++;
        if (/abort|cancel/i.test(String(p.error.name ?? ''))) return;
        await emit({ kind: 'terminal_error', sessionID: p.sessionID, turnID: uid, rootSession }, 'error', s, undefined, undefined, undefined, born);
        return;
      }
      // Even an uncorrelated source error closes attention; it cannot invent a terminal fact.
      s.questions.clear(); s.permissions.clear(); s.revision++;
      if (p.messageID && p.messageID !== s.activeAssistant) { diag('unmatched error message'); return; }
      // A source error can describe compaction or occur before an assistant exists.
      // It never selects a later assistant from history to invent correlation.
      if (/abort|cancel/i.test(String(p.error.name ?? ''))) {
        s.cancelled = true; s.questions.clear(); s.permissions.clear(); s.revision++; return;
      }
      const candidate = s.activeAssistant;
      if (!candidate || s.compaction || s.ambiguousAssistant) { diag('unmatched error message'); return; }
      s.sourceError = { messageID: candidate, name: p.error.name };
      s.questions.clear(); s.permissions.clear(); s.errorPending++; s.revision++;
      const messages = await latest(p.sessionID, deadline);
      s.errorPending--;
      if (!messages || sessions.get(p.sessionID) !== s || s.turnEpoch !== epoch || s.user !== uid ||
          s.admitted.has('idle') || !currentTurn(messages, uid)) return;
      if (p.error.name === 'ContextOverflowError') s.overflowPending = true;
      // Only the observed final matching assistant error plus idle is terminal.
      if (s.idleObserved) await idle(p.sessionID, born, deadline);
      return;
    }
    await emit({ kind: 'unknown', nativeType: type, ...(id(p.sessionID) ? { sessionID: p.sessionID } : {}) },
      `unknown:${type}:${id(p.sessionID) ? p.sessionID : ''}`, undefined, undefined, undefined, undefined, born);
  }
  return {
    observe(event) {
      if (disposed) return Promise.resolve();
      if (strict && runtime !== 'supported') { diag(runtime === 'unsupported' ? 'runtime_unsupported' : 'runtime_unverified'); return Promise.resolve(); }
      // Control ingress is never queued behind lookups or external callbacks.
      const relevant = ['session.idle', 'session.error', 'question.asked', 'permission.asked'].includes(event?.type) ||
        (event?.type === 'session.status' && event.properties?.status?.type === 'idle') ||
        (event?.type === 'message.updated' && event.properties?.info?.error != null);
      const control = ['message.updated', 'question.replied', 'question.rejected', 'permission.replied', 'session.compacted'].includes(event?.type) ||
        (strict && ['session.updated', 'session.deleted'].includes(event?.type)) ||
        (event?.type === 'session.status' && event.properties?.status?.type !== 'idle');
      const sid = nativeSessionID(event);
      if ((relevant || !control) && activeJobs >= 256) {
        if (id(sid)) { const s = state(sid); s.cancelled = true; s.questions.clear(); s.permissions.clear(); s.revision++; }
        for (const owned of handoffs) if (!owned.relevant()) owned.controller.abort();
        diag('job_capacity'); return Promise.resolve();
      }
      const counted = relevant || !control;
      if (counted) { activeJobs++; if (id(sid)) jobsBySession.set(sid, (jobsBySession.get(sid) ?? 0) + 1); }
      const born = now();
      const work = observe(event, born, born + timeoutMs);
      for (const owned of handoffs) if (!owned.relevant()) owned.controller.abort();
      return work.finally(() => {
        if (counted) {
          activeJobs--;
          const remaining = (jobsBySession.get(sid) ?? 1) - 1;
          if (remaining) jobsBySession.set(sid, remaining); else jobsBySession.delete(sid);
        }
      });
    },
    dispose() { disposed = true; sessions.clear(); lifetime.abort(); core.invalidate(); for (const owned of handoffs) owned.controller.abort(); },
  };
}
