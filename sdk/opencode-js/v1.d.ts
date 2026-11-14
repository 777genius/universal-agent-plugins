import type { Event as OpenCodeEvent } from "@opencode-ai/sdk";

export type OpenCodeNativeEvent = OpenCodeEvent;
import type { ObservedEvent, Handoff, PreparationHandoff } from './common.js';
export type { ObservedEvent, Handoff, PreparationHandoff } from './common.js';
export type ObserverOptions = {
  client: { session: {
    messages(input: {path: {id: string}, query: {limit: number}, signal?: AbortSignal}): Promise<unknown>;
    get(input: {path: {id: string}, signal?: AbortSignal}): Promise<unknown>;
  } };
  /** Literal true continues to the final native snapshot and synchronous emit. */
  beforeEmit?(event: ObservedEvent, handoff: PreparationHandoff): boolean | Promise<boolean>;
  emit(event: ObservedEvent, handoff: Handoff): void | Promise<void>;
  /** Supplying runtime authority enables strict provenance/root/scope eligibility. */
  runtimeEligibility?(): 'supported' | 'unverified' | 'unsupported';
  location?: string;
  /** Actual host-registered await-free V1 hook; hydration cannot open requests. */
  callbackAuthority?: 'qualified_native_sync';
  clock?: import('./common.js').MonotonicClock;
  onDiagnostic?(reason: string): void;
  messageLimit?: number;
  dedupLimit?: number;
  lookupTimeoutMs?: number;
  maxConcurrentLookups?: number;
};
export declare function createObserver(options: ObserverOptions): {
  observe(event: unknown): Promise<void>;
  dispose(): void;
};
