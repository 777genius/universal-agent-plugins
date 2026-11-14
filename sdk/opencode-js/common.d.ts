/** Private candidate provenance. No receive clock is a native birth clock. */
export type Provenance = {
  generation: 'v1' | 'v2';
  observationID: string;
  nativeEventID?: string;
  /** Actual verified final assistant for strict V1 terminal errors. */
  nativeMessageID?: string;
  nativeTime: number;
  timeBasis: 'envelope_created' | 'assistant_created_lower_bound' | 'assistant_completed';
};
export type ObservedEvent =
  | { version: 1; kind: 'turn_idle_verified'; sessionID: string; turnID: string; messageID: string; rootSession: boolean; provenance?: Provenance }
  | { version: 1; kind: 'question_asked' | 'permission_asked'; sessionID: string; turnID: string; requestID: string; rootSession: boolean; provenance?: Provenance }
  | { version: 1; kind: 'terminal_error'; sessionID: string; turnID: string; rootSession: boolean; provenance?: Provenance }
  | { version: 1; kind: 'unknown'; nativeType: string; sessionID?: string };
export type MonotonicClock = { id: string; now(): number };
/** Process-local default cannot be compared to Go/shared boot ticks. Preparation
 * shares the original metadata deadline and has no final-checkpoint access. */
export type PreparationHandoff = {
  readonly signal: AbortSignal;
  readonly ingressMonotonicMs: number;
  readonly clockID: string;
  readonly metadataDeadline: number;
  isCurrent(): boolean;
};
/** Put async preparation in beforeEmit. SDK then takes its final snapshot.
 * Final emit checks isCurrent and spawns synchronously before its first await;
 * the returned promise lasts through actual owned child close. revalidate is
 * memoized: repeated calls reuse the SDK snapshot, never advance the barrier. */
export type Handoff = PreparationHandoff & { revalidate?(): Promise<boolean> };
