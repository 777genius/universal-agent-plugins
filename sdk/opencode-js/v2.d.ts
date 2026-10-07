import type { Handoff, ObservedEvent, PreparationHandoff } from './common.js';
export type { ObservedEvent, Handoff, PreparationHandoff } from './common.js';
export type V2Envelope = {
  id: string; created: number; type: string; data: unknown; location?: unknown;
};
/** Structural public event subscription only; no ambient transport/client. */
export type V2Context = {
  app: { version: string };
  event: { subscribe(input: { signal: AbortSignal }): AsyncIterable<unknown> };
};
/** Projection of qualified native fields, never locally invented identities.
 * Sequence is optional for ephemeral events, mandatory for durable session events.
 * Native public seq is sparse; positive increments are not a loss detector.
 * Preserve native version when present. undefined means irrelevant; malformed
 * relevant correlation must retain its session ID so that scope is fenced.
 * aggregate must denote the proven session sequence domain. */
export type V2Correlation = {
  sessionID: string; location?: string; messageID?: string; requestID?: string; callID?: string;
  sequence?: { aggregate: string; seq: number; version?: number };
  /** Actual native step type compaction, including manual control with inputID. */
  compaction?: boolean;
};
export type V2Run = { sessionID: string; /** Observed execution.started native key, separate from public user turnID. */ turnID: string; userID: string; location: string; observedLocation?: string; terminalEventID?: string; terminalCreated?: number; terminalSequence?: { aggregate: string; seq: number; version?: number } };
/** Use actual projected ancestry: fork event parentID is lineage, not a child.
 * SessionInfo parentID is true subagent ancestry; public create cannot fabricate it. */
export type V2SessionProof = V2Run & { rootSession: boolean };
export type V2AssistantProof = V2SessionProof & {
  messageID: string; role: 'assistant'; summary: boolean; final: boolean; outcome: 'success' | 'error';
};
export type V2PermissionProof = V2SessionProof & {
  requestID: string; messageID: string; callID: string; pending: boolean;
};
export type V2QuestionSourceProof = V2SessionProof & {
  requestID: string; messageID: string; callID: string; role: 'assistant'; tool: 'question'; summary: boolean;
};
/** A narrowly typed adapter for ctx.rpc.register portable no-method events.
 * register owns a private registration/type unique to the provided namespace;
 * emit uses registration.events.emit. namespace is a local fence, never identity.
 * read validates the closed native marker schema and returns its nonce only.
 * No form state, REST client, ambient credentials or source lease is implied. */
export type V2CheckpointPort = {
  register(signal: AbortSignal, namespace: string): Promise<{
    type: string;
    emit(nonce: string): Promise<void>;
    read(envelope: V2Envelope): string | undefined;
    dispose(): void | Promise<void>;
  }>;
};
/** Integration ports attest actual native state. These are NOT additional
 * Context methods. Bind only to qualified public APIs; absence stays closed. */
export type V2NativePorts = {
  correlate(envelope: V2Envelope): V2Correlation | undefined;
  session(run: V2Run, signal: AbortSignal): Promise<V2SessionProof | undefined>;
  assistant(run: V2Run & { messageID: string }, signal: AbortSignal): Promise<V2AssistantProof | undefined>;
  /** Verify actual native form source and assistant/tool/execution binding,
   * using public session context. This is NOT a pending-form read. */
  questionSource?(run: V2Run & { requestID: string; messageID: string; callID: string }, signal: AbortSignal): Promise<V2QuestionSourceProof | undefined>;
  /** ctx.permission.list/get adapter, retaining native source message/call. */
  currentPermission?(run: V2Run & { requestID: string }, signal: AbortSignal): Promise<V2PermissionProof | undefined>;
};
export type V2ObserverOptions = {
  context: V2Context;
  /** Canonical resolved server location; never a substitute for observed scope. */
  location: string;
  /** Supplied from lane A's shared qualification authority, not a SDK range. */
  runtimeEligibility(version: string): 'supported' | 'unverified' | 'unsupported';
  native: V2NativePorts;
  /** Absence keeps live questions closed with form_checkpoint_unavailable. */
  checkpoint?: V2CheckpointPort;
  /** Literal true continues to the final native snapshot and synchronous emit. */
  beforeEmit?(event: ObservedEvent, handoff: PreparationHandoff): boolean | Promise<boolean>;
  emit(event: ObservedEvent, handoff: Handoff): void | Promise<void>;
  onDiagnostic?(reason: string): void;
  clock?: import('./common.js').MonotonicClock;
  maxSessions?: number; maxJobs?: number; maxConcurrentLookups?: number; lookupTimeoutMs?: number;
};
export declare function createV2Observer(options: V2ObserverOptions): {
  /** Begins one owned subscription; repeated start is a no-op. */
  start(): void;
  /** Synchronous token invalidation, then abort. */
  dispose(): void;
  /** Resolves once lifetime subscription and all retained jobs finish. */
  done(): Promise<void>;
};
