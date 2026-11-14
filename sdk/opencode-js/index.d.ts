import type { Event as OpenCodeEvent } from "@opencode-ai/sdk";

export type OpenCodeNativeEvent = OpenCodeEvent;
export type ObservedEvent =
  | { version: 1; kind: "turn_idle_verified"; sessionID: string; turnID: string; messageID: string; rootSession: boolean }
  | { version: 1; kind: "question_asked" | "permission_asked"; sessionID: string; turnID: string; requestID: string; rootSession: boolean }
  | { version: 1; kind: "terminal_error"; sessionID: string; turnID: string; rootSession: boolean }
  | { version: 1; kind: "unknown"; nativeType: string; sessionID?: string };
export type { ObserverOptions } from './v1.js';
export { createObserver } from './v1.js';

/** Native V2 ownership identity. Directory and workspaceID must both match. */
export type V2Location = { directory: string; workspaceID?: string };
/** V2 API returns bare values, unlike V1 SDK response wrappers. */
export type V2Client = {
  get(input: { sessionID: string }, options?: { signal?: AbortSignal }): Promise<unknown>;
  context(input: { sessionID: string }, options?: { signal?: AbortSignal }): Promise<unknown>;
};
export type V2NativeEvent = {
  type: string;
  data: unknown;
  id?: string;
  location?: V2Location;
};
export type V2ObservedEvent = Exclude<ObservedEvent, { kind: "unknown" }>;
export type V2ObserverOptions = {
  client: V2Client;
  location: V2Location;
  emit(event: V2ObservedEvent): void | Promise<void>;
  onDiagnostic?(reason: string): void;
  maxSessions?: number;
  maxConcurrentLookups?: number;
  lookupTimeoutMs?: number;
};
export declare function createV2Observer(options: V2ObserverOptions): {
  /** Reduces metadata synchronously; verification/delivery must not block ingress. */
  observe(event: unknown): void;
  /** Fences late results immediately without waiting for an uncancellable host call. */
  dispose(): void;
};
