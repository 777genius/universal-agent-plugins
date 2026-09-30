import type { Event as OpenCodeEvent } from "@opencode-ai/sdk";

export type OpenCodeNativeEvent = OpenCodeEvent;
export type ObservedEvent =
  | { version: 1; kind: "turn_idle_verified"; sessionID: string; turnID: string; messageID: string; rootSession: boolean }
  | { version: 1; kind: "question_asked" | "permission_asked"; sessionID: string; turnID: string; requestID: string; rootSession: boolean }
  | { version: 1; kind: "terminal_error"; sessionID: string; turnID: string; rootSession: boolean }
  | { version: 1; kind: "unknown"; nativeType: string; sessionID?: string };
export type ObserverOptions = {
  client: { session: {
    messages(input: {path: {id: string}, query: {limit: number}, signal?: AbortSignal}): Promise<unknown>;
    get(input: {path: {id: string}, signal?: AbortSignal}): Promise<unknown>;
  } };
  emit(event: ObservedEvent): void | Promise<void>;
  onDiagnostic?(reason: string): void;
  messageLimit?: number;
  dedupLimit?: number;
  lookupTimeoutMs?: number;
  maxConcurrentLookups?: number;
};
export declare function createObserver(options: ObserverOptions): {
  observe(event: unknown): Promise<void>;
};
