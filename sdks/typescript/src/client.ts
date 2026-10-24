import createOpenAPIClient from 'openapi-fetch';
import type { paths } from './generated/schema.js';
import type { ChangeRequest, LocalSkillRequest, Operation, SourceRequest, WireError } from './types.js';

export interface ClientOptions {
  baseUrl: string;
  token: string;
  /** Exact origin from the private host bootstrap. Defaults to the base URL origin. */
  origin?: string;
  fetch?: typeof globalThis.fetch;
}
export interface RequestOptions { signal?: AbortSignal }
export interface WaitOptions extends RequestOptions {
  pollIntervalMs?: number;
  timeoutMs?: number;
  /** Observation only. A callback error stops waiting, never the accepted operation. */
  onProgress?: (operation: Operation) => void;
}
export class ManagementError extends Error {
  readonly code: string;
  readonly status: number;
  readonly retryable: boolean;
  constructor(code: string, message: string, status = 0, retryable = false) {
    super(message);
    this.name = 'ManagementError';
    this.code = code;
    this.status = status;
    this.retryable = retryable;
  }
}

const terminalStates = new Set(['succeeded', 'partial', 'failed', 'canceled', 'interrupted']);
export function isTerminal(operation: Operation): boolean {
  if (terminalStates.has(operation.state)) return true;
  if (operation.state === 'queued' || operation.state === 'running') return false;
  throw new ManagementError('UNKNOWN_OPERATION_STATE', 'The engine returned an unsupported operation state. Upgrade the client before continuing.');
}
function responseError(response: { error?: WireError; response: Response }): void {
  if (response.response.ok) return;
  const error = response.error;
  throw new ManagementError(
    typeof error?.code === 'string' ? error.code : 'HTTP_ERROR',
    typeof error?.message === 'string' ? error.message : `Management request failed (${response.response.status}).`,
    response.response.status, error?.retryable === true,
  );
}
function unwrap<T>(response: { data?: T; error?: WireError; response: Response }): T {
  responseError(response);
  if (response.data === undefined) throw new ManagementError('EMPTY_RESPONSE', 'The engine returned an empty response.', response.response.status);
  return response.data;
}
async function delay(ms: number, signal: AbortSignal): Promise<void> {
  signal.throwIfAborted();
  await new Promise<void>((resolve, reject) => {
    const done = () => { signal.removeEventListener('abort', canceled); resolve(); };
    const timer = setTimeout(done, ms);
    const canceled = () => {
      clearTimeout(timer);
      signal.removeEventListener('abort', canceled);
      reject(signal.reason ?? new DOMException('Waiting was canceled.', 'AbortError'));
    };
    signal.addEventListener('abort', canceled, { once: true });
    if (signal.aborted) canceled();
  });
}
function positive(value: number, label: string): number {
  if (!Number.isFinite(value) || value <= 0) throw new RangeError(`${label} must be positive and finite.`);
  return value;
}

/** Pure HTTP client. It never launches a process, invents target paths or retries a mutation. */
export function createClient(options: ClientOptions) {
  const endpoint = new URL(options.baseUrl);
  if (!['http:', 'https:'].includes(endpoint.protocol) || endpoint.username || endpoint.password || endpoint.search || endpoint.hash) {
    throw new ManagementError('INVALID_ENDPOINT', 'Expected an HTTP(S) management base URL without credentials, query or fragment.');
  }
  if (!options.token) throw new ManagementError('MISSING_CAPABILITY', 'A private management capability is required.');
  const transport = createOpenAPIClient<paths>({
    baseUrl: options.baseUrl.replace(/\/$/, ''),
    headers: { Authorization: `Bearer ${options.token}`, Origin: options.origin ?? endpoint.origin },
    redirect: 'error',
    ...(options.fetch ? { fetch: options.fetch } : {}),
  });
  const client = {
    session: async (request: RequestOptions = {}) => unwrap(await transport.GET('/api/v1/session', { ...request })),
    catalog: async (query?: string, request: RequestOptions = {}) => unwrap(await transport.GET('/api/v1/catalog', {
      ...request, ...(query === undefined ? {} : { params: { query: { query } } }),
    })),
    catalogEntry: async (id: string, request: RequestOptions = {}) => unwrap(await transport.GET('/api/v1/catalog/{id}', { ...request, params: { path: { id } } })),
    discoverSkills: async (body: SourceRequest, request: RequestOptions = {}) => unwrap(await transport.POST('/api/v1/skills/catalog', { ...request, body })),
    skills: {
      local: {
        prepare: async (body: LocalSkillRequest, request: RequestOptions = {}) => unwrap(await transport.POST('/api/v1/skills/local/plans', { ...request, body })),
      },
      preview: async (catalogId: string, candidateId: string, request: RequestOptions = {}) => unwrap(await transport.GET('/api/v1/skills/catalog/{catalogId}/candidates/{candidateId}/preview', { ...request, params: { path: { catalogId, candidateId } } })),
    },
    environment: async (request: RequestOptions = {}) => unwrap(await transport.GET('/api/v1/environment', { ...request })),
    prepare: async (body: ChangeRequest, request: RequestOptions = {}) => unwrap(await transport.POST('/api/v1/plans', { ...request, body })),
    plans: {
      /** Release an unaccepted review when a dialog closes or the selection changes. */
      discard: async (id: string, request: RequestOptions = {}): Promise<void> => {
        responseError(await transport.DELETE('/api/v1/plans/{id}', { ...request, params: { path: { id } } }));
      },
    },
    /** Persist requestId in the host before calling. A retry must reuse both IDs. */
    apply: async (planId: string, requestId: string, request: RequestOptions = {}) => unwrap(await transport.POST('/api/v1/plans/{id}/apply', { ...request, params: { path: { id: planId } }, body: { requestId } })),
    operation: async (id: string, request: RequestOptions = {}) => unwrap(await transport.GET('/api/v1/operations/{id}', { ...request, params: { path: { id } } })),
    cancel: async (id: string, request: RequestOptions = {}) => unwrap(await transport.POST('/api/v1/operations/{id}/cancel', { ...request, params: { path: { id } } })),
    /** Polls authoritative snapshots. Failure/partial/interrupted are returned for the host to render. */
    async wait(id: string, options: WaitOptions = {}): Promise<Operation> {
      const interval = positive(options.pollIntervalMs ?? 500, 'pollIntervalMs');
      const timeoutMs = options.timeoutMs === undefined ? undefined : positive(options.timeoutMs, 'timeoutMs');
      const controller = new AbortController();
      const abort = () => controller.abort(options.signal?.reason);
      options.signal?.addEventListener('abort', abort, { once: true });
      if (options.signal?.aborted) abort();
      const timeout = timeoutMs === undefined ? undefined : setTimeout(() => controller.abort(new ManagementError('WAIT_TIMEOUT', 'Stopped waiting. The accepted operation may still be running.')), timeoutMs);
      let previous: string | undefined;
      try {
        for (;;) {
          controller.signal.throwIfAborted();
          const operation = await client.operation(id, { signal: controller.signal });
          const snapshot = JSON.stringify(operation);
          if (snapshot !== previous) { previous = snapshot; options.onProgress?.(operation); }
          if (isTerminal(operation)) return operation;
          await delay(interval, controller.signal);
        }
      } finally {
        if (timeout !== undefined) clearTimeout(timeout);
        options.signal?.removeEventListener('abort', abort);
      }
    },
  };
  return client;
}
export type ManagementClient = ReturnType<typeof createClient>;
