// A content-free observer for OpenCode's native event hook.
const object = (x) => x !== null && typeof x === 'object' && !Array.isArray(x);
const id = (x) => typeof x === 'string' && x.length > 0 && !/[\u0000-\u001f]/u.test(x) && new TextEncoder().encode(x).length <= 256;
const typeOK = (x) => typeof x === 'string' && /^[a-z][a-z0-9.-]{0,79}$/.test(x);

/** @param {import('./index.d.ts').ObserverOptions} options */
export function createObserver(options) {
  if (typeof options?.emit !== 'function' || typeof options.client?.session?.messages !== 'function' ||
      typeof options.client?.session?.get !== 'function') {
    throw new TypeError('emit, messages and get required');
  }
  const limit = Math.max(2, Math.min(100, Number.isInteger(options.messageLimit) ? options.messageLimit : 30));
  const maxSessions = Math.max(1, Math.min(4096, Number.isInteger(options.dedupLimit) ? options.dedupLimit : 512));
  const timeoutMs = Math.max(100, Math.min(10000, Number.isInteger(options.lookupTimeoutMs) ? options.lookupTimeoutMs : 2000));
  const maxLookups = Math.max(1, Math.min(64, Number.isInteger(options.maxConcurrentLookups) ? options.maxConcurrentLookups : 16));
  const sessions = new Map(), unknownSeen = new Set();
  let activeLookups = 0;
  const diag = (reason) => { try { options.onDiagnostic?.(reason); } catch { /* advisory sink */ } };
  const state = (sid) => {
    if (!sessions.has(sid)) {
      sessions.set(sid, {
        user: '', userCreated: undefined, seenUsers: new Set(), assistant: '', failedAssistant: '',
        overflowPending: false, idleObserved: false, retry: false, retryAssistant: '', cancelled: false,
        questions: new Set(), permissions: new Set(), resolved: new Set(), admitted: new Set(),
        turnEpoch: 0, revision: 0, errorPending: 0, rootSession: undefined,
      });
      if (sessions.size > maxSessions) sessions.delete(sessions.keys().next().value);
    }
    return sessions.get(sid);
  };
  async function emit(fact, key, session) {
    const admitted = session ? session.admitted : unknownSeen;
    if (admitted.has(key)) return;
    admitted.add(key); // Admit before the external callback: its result may be ambiguous.
    if (!session && admitted.size > maxSessions) admitted.delete(admitted.values().next().value);
    try { await options.emit({ version: 1, ...fact }); }
    catch { diag('observed event callback failed'); }
  }
  async function boundedLookup(call) {
    if (activeLookups >= maxLookups) { diag('messages lookup capacity exceeded'); return; }
    const controller = new AbortController();
    let timer;
    activeLookups++;
    const request = Promise.resolve().then(() => call(controller.signal));
    request.then(() => { activeLookups--; }, () => { activeLookups--; });
    try {
      return await Promise.race([
        request,
        new Promise((_, reject) => {
          timer = setTimeout(() => { controller.abort(); reject(new Error('lookup timeout')); }, timeoutMs);
        }),
      ]);
    } catch { diag('lookup failed or timed out'); return; }
    finally { clearTimeout(timer); }
  }
  async function rootStatus(sid) {
    const s = state(sid);
    if (typeof s.rootSession === 'boolean') return s.rootSession;
    const result = await boundedLookup((signal) => options.client.session.get({ path: { id: sid }, signal }));
    const info = result?.data ?? result;
    if (!object(info) || info.id !== sid || (info.parentID !== undefined && !id(info.parentID))) {
      diag('invalid session ancestry'); return;
    }
    s.rootSession = info.parentID === undefined;
    return s.rootSession;
  }
  async function latest(sid) {
    const result = await boundedLookup((signal) => options.client.session.messages({
      path: { id: sid }, query: { limit }, signal,
    }));
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
  async function idle(sid) {
    const s = state(sid), revision = s.revision, uid = s.user;
    s.idleObserved = true;
    const failed = Boolean(s.failedAssistant || s.overflowPending);
    if (!uid || (!s.assistant && !failed) ||
        s.admitted.has('idle') || s.admitted.has('error') || (s.retry && !failed) || s.cancelled ||
        s.errorPending || s.questions.size || s.permissions.size) return;
    const messages = await latest(sid);
    if (!messages || sessions.get(sid) !== s || revision !== s.revision || s.user !== uid ||
        (s.retry && !failed) || s.cancelled || s.errorPending || s.questions.size || s.permissions.size) return;
    if (!currentTurn(messages, uid)) return;
    const answer = messages.at(-1);
    if (s.failedAssistant || s.overflowPending) {
      if (!answer || answer.role !== 'assistant' || answer.parentID !== uid ||
          (s.failedAssistant && answer.id !== s.failedAssistant) || !object(answer.error) ||
          /abort|cancel/i.test(String(answer.error.name ?? ''))) return;
      if (s.overflowPending && !s.failedAssistant && answer.error.name !== 'ContextOverflowError') return;
      const rootSession = await rootStatus(sid);
      if (rootSession === undefined || sessions.get(sid) !== s || revision !== s.revision || s.user !== uid || s.cancelled) return;
      s.cancelled = true; s.revision++;
      await emit({ kind: 'terminal_error', sessionID: sid, turnID: uid, rootSession }, 'error', s);
      return;
    }
    if (!answer || answer.role !== 'assistant' || answer.id !== s.assistant || answer.parentID !== uid ||
        answer.finish !== 'stop' || !Number.isFinite(answer.time?.completed) || answer.error != null) return;
    const rootSession = await rootStatus(sid);
    if (rootSession === undefined || sessions.get(sid) !== s || revision !== s.revision || s.user !== uid || s.cancelled || s.errorPending) return;
    await emit({ kind: 'turn_idle_verified', sessionID: sid, turnID: uid, messageID: answer.id, rootSession }, 'idle', s);
  }
  async function observe(event) {
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
        s.questions.clear(); s.permissions.clear(); s.resolved.clear(); s.admitted.clear();
        s.turnEpoch++; s.revision++;
      }
      if (m.role === 'assistant' && m.parentID === s.user && m.finish === 'stop' &&
          Number.isFinite(m.time?.completed) && m.error == null && m.id !== s.retryAssistant) {
        s.assistant = m.id; s.failedAssistant = ''; s.overflowPending = false; s.retry = false; s.revision++;
      }
      if (m.role === 'assistant' && m.parentID === s.user && m.error != null) {
        s.assistant = ''; s.failedAssistant = m.id; s.revision++;
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
        s.failedAssistant = ''; s.overflowPending = false; s.idleObserved = false; s.retry = true; s.revision++;
      }
      if (p.status.type === 'idle') await idle(p.sessionID);
      return;
    }
    if (type === 'session.idle') { if (!id(p.sessionID)) { diag('invalid session.idle'); return; } await idle(p.sessionID); return; }
    if (type === 'question.asked' || type === 'permission.asked') {
      const requestMessageID = p.messageID ?? p.tool?.messageID;
      if (!id(p.sessionID) || !id(p.id) || (requestMessageID !== undefined && !id(requestMessageID))) { diag('invalid request'); return; }
      if (type === 'question.asked' && (!Array.isArray(p.questions) || p.questions.length === 0)) { diag('invalid question shape'); return; }
      if (type === 'permission.asked' && (typeof p.permission !== 'string' || !p.permission)) { diag('invalid permission shape'); return; }
      const s = state(p.sessionID), uid = s.user, epoch = s.turnEpoch;
      if (!uid || s.resolved.has(p.id)) { diag('unmatched or resolved request'); return; }
      const pending = type === 'question.asked' ? s.questions : s.permissions;
      pending.add(p.id); s.revision++;
      const messages = await latest(p.sessionID);
      if (sessions.get(p.sessionID) !== s || s.turnEpoch !== epoch || s.user !== uid || !pending.has(p.id) || s.resolved.has(p.id)) return;
      if (!messages || !currentTurn(messages, uid)) { pending.delete(p.id); diag('unmatched request turn'); return; }
      if (requestMessageID && requestMessageID !== uid && !messages.some((m) => m.role === 'assistant' && m.id === requestMessageID && m.parentID === uid)) {
        pending.delete(p.id); diag('unmatched request message'); return;
      }
      const rootSession = await rootStatus(p.sessionID);
      if (rootSession === undefined || sessions.get(p.sessionID) !== s || s.turnEpoch !== epoch || s.user !== uid || !pending.has(p.id) || s.resolved.has(p.id)) return;
      s.assistant = ''; s.revision++;
      await emit({ kind: type === 'question.asked' ? 'question_asked' : 'permission_asked', sessionID: p.sessionID, turnID: uid, requestID: p.id, rootSession }, `${type}:${p.id}`, s);
      return;
    }
    if (type === 'question.replied' || type === 'question.rejected' || type === 'permission.replied') {
      if (!id(p.sessionID) || !id(p.requestID)) { diag('invalid resolution'); return; }
      const s = state(p.sessionID);
      const pending = type.startsWith('question.') ? s.questions : s.permissions;
      const current = pending.delete(p.requestID);
      s.resolved.add(p.requestID);
      if (s.resolved.size > maxSessions) s.resolved.delete(s.resolved.values().next().value);
      if (current) { s.assistant = ''; s.revision++; } // Require a later final update only for a current request.
      return;
    }
    if (type === 'session.error') {
      if (!id(p.sessionID) || !object(p.error) || (p.messageID !== undefined && !id(p.messageID))) { diag('invalid session.error'); return; }
      const s = state(p.sessionID), uid = s.user, epoch = s.turnEpoch;
      if (!uid || s.admitted.has('idle')) return;
      s.errorPending++; s.revision++;
      const messages = await latest(p.sessionID);
      s.errorPending--;
      if (!messages || sessions.get(p.sessionID) !== s || s.turnEpoch !== epoch || s.user !== uid || s.admitted.has('idle') || !currentTurn(messages, uid)) return;
      if (p.messageID && !messages.some((m) => m.role === 'assistant' && m.id === p.messageID && m.parentID === uid)) {
        diag('unmatched error message'); return;
      }
      const rootSession = await rootStatus(p.sessionID);
      if (rootSession === undefined || sessions.get(p.sessionID) !== s || s.turnEpoch !== epoch || s.user !== uid || s.admitted.has('idle')) return;
      if (p.error.name === 'ContextOverflowError') {
        // The same native error also precedes automatic compaction and retry.
        // Only a final idle with an assistant error proves it was terminal.
        s.overflowPending = true; s.revision++;
        if (s.idleObserved) await idle(p.sessionID);
        return;
      }
      s.cancelled = true; s.revision++;
      if (/abort|cancel/i.test(String(p.error.name ?? ''))) return;
      await emit({ kind: 'terminal_error', sessionID: p.sessionID, turnID: uid, rootSession }, 'error', s);
      return;
    }
    await emit({ kind: 'unknown', nativeType: type, ...(id(p.sessionID) ? { sessionID: p.sessionID } : {}) },
      `unknown:${type}:${id(p.sessionID) ? p.sessionID : ''}`);
  }
  return { observe };
}
