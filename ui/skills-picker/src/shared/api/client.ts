import createClient from 'openapi-fetch';
import type { paths } from './generated/schema';
import type { PlanRequest } from './types';

const storageKey = 'uap.skills.capability';
export function bootstrapCapability() {
  const fragment = window.location.hash.slice(1);
  if (fragment) {
    const token = new URLSearchParams(fragment).get('capability') ?? fragment;
    sessionStorage.setItem(storageKey, token);
    history.replaceState(null, '', window.location.pathname + window.location.search);
  }
  return sessionStorage.getItem(storageKey) ?? '';
}
export class APIError extends Error {
  constructor(public code: string, message: string, public status: number) { super(message); }
}
const client = createClient<paths>({ baseUrl: window.location.origin });
client.use({ onRequest({ request }) {
  request.headers.set('Authorization', `Bearer ${sessionStorage.getItem(storageKey) ?? ''}`);
  return request;
}, onResponse({ response }) {
  if (response.status === 410) { sessionStorage.removeItem(storageKey); window.dispatchEvent(new Event('uap-session-expired')); }
} });
function unwrap<T>(result: { data?: T; error?: { code: string; message: string }; response: Response }): T {
  if (result.error) throw new APIError(result.error.code, result.error.message, result.response.status);
  if (result.data === undefined) throw new APIError('EMPTY_RESPONSE', 'The CLI returned an empty response.', result.response.status);
  return result.data;
}
export const api = {
  session: async (signal?: AbortSignal) => unwrap(await client.GET('/api/v1/session', { signal })),
  catalog: async (signal?: AbortSignal) => unwrap(await client.GET('/api/v1/catalog', { signal })),
  preview: async (id: string, signal?: AbortSignal) => unwrap(await client.GET('/api/v1/candidates/{id}/preview', { params: { path: { id } }, signal })),
  targets: async (signal?: AbortSignal) => unwrap(await client.GET('/api/v1/targets', { signal })),
  scan: async (scopePath: string) => unwrap(await client.POST('/api/v1/catalog', { body: { scopePath } })),
  plan: async (body: PlanRequest) => unwrap(await client.POST('/api/v1/plans', { body })),
  apply: async (id: string) => unwrap(await client.POST('/api/v1/plans/{id}/apply', { params: { path: { id } } })),
  operation: async (id: string, signal?: AbortSignal) => unwrap(await client.GET('/api/v1/operations/{id}', { params: { path: { id } }, signal })),
};
