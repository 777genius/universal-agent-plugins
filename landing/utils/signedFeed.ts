const encoder = new TextEncoder();
const decoder = new TextDecoder('utf-8', { fatal: true });

export interface ArtifactResponse {
  bytes: Uint8Array;
  etag?: string;
  notModified: boolean;
}

export async function fetchSignedArtifact(
  url: URL,
  maximum: number,
  fetcher: typeof fetch,
  namespace: string,
  etag?: string,
): Promise<ArtifactResponse> {
  const headers = new Headers({ accept: 'application/json' });
  if (etag) headers.set('if-none-match', etag);
  const response = await fetcher(url, {
    cache: 'no-cache',
    credentials: 'omit',
    headers,
    redirect: 'error',
  });
  if (response.status === 304) return { bytes: new Uint8Array(), etag, notModified: true };
  if (!response.ok || (response.url && new URL(response.url).origin !== url.origin)) {
    throw new Error(`${namespace} request failed with HTTP ${response.status}`);
  }
  const declared = Number(response.headers.get('content-length'));
  if (Number.isFinite(declared) && declared > maximum) {
    throw new Error(`${namespace} response exceeds its size limit`);
  }
  const bytes = new Uint8Array(await response.arrayBuffer());
  if (!bytes.length || bytes.length > maximum) {
    throw new Error(`${namespace} response exceeds its size limit`);
  }
  return { bytes, etag: response.headers.get('etag') ?? undefined, notModified: false };
}

export function parseCanonicalJSON<T>(
  bytes: Uint8Array,
  namespace: string,
  label: string,
): T {
  const text = decoder.decode(bytes);
  const value = JSON.parse(text) as T;
  if (canonicalJSON(value, namespace) !== text) {
    throw new Error(`${namespace} ${label} is not canonical JSON`);
  }
  return value;
}

export function canonicalJSON(value: unknown, namespace = 'Signed feed'): string {
  validateCanonical(value, namespace);
  const sort = (item: unknown): unknown =>
    Array.isArray(item)
      ? item.map(sort)
      : item && typeof item === 'object'
        ? Object.fromEntries(
            Object.entries(item)
              .sort(([left], [right]) => (left < right ? -1 : left > right ? 1 : 0))
              .map(([key, child]) => [key, sort(child)]),
          )
        : item;
  return `${JSON.stringify(sort(value))}\n`;
}

function validateCanonical(value: unknown, namespace: string) {
  if (value === null || typeof value === 'boolean') return;
  if (typeof value === 'number') {
    if (!Number.isSafeInteger(value)) {
      throw new Error(`${namespace} JSON contains a non-integer number`);
    }
    return;
  }
  if (typeof value === 'string') {
    if (value !== value.normalize('NFC')) {
      throw new Error(`${namespace} JSON contains non-NFC text`);
    }
    return;
  }
  if (Array.isArray(value)) return value.forEach((child) => validateCanonical(child, namespace));
  if (!value || typeof value !== 'object') {
    throw new Error(`${namespace} JSON contains an unsupported value`);
  }
  const folded = new Set<string>();
  Object.entries(value).forEach(([key, child]) => {
    if (key !== key.normalize('NFC') || folded.has(key.toLocaleLowerCase())) {
      throw new Error(`${namespace} JSON contains colliding keys`);
    }
    folded.add(key.toLocaleLowerCase());
    validateCanonical(child, namespace);
  });
}

export async function sha256Digest(bytes: Uint8Array) {
  const digest = new Uint8Array(
    await crypto.subtle.digest('SHA-256', Uint8Array.from(bytes)),
  );
  return `sha256:${[...digest].map((byte) => byte.toString(16).padStart(2, '0')).join('')}`;
}

export function signedMessage(domain: string, snapshot: Uint8Array) {
  const prefix = encoder.encode(`${domain}\0`);
  const result = new Uint8Array(prefix.length + 8 + snapshot.length);
  result.set(prefix);
  new DataView(result.buffer).setBigUint64(prefix.length, BigInt(snapshot.length));
  result.set(snapshot, prefix.length + 8);
  return result;
}

export function parseUTCTimestamp(value: string, namespace: string) {
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/.test(value)) {
    throw new Error(`${namespace} timestamp must use second-precision UTC`);
  }
  const parsed = new Date(value);
  if (!Number.isFinite(parsed.getTime())) throw new Error(`${namespace} timestamp is invalid`);
  return parsed;
}

export function assertExactKeys(
  value: object,
  expected: string[],
  namespace: string,
  label: string,
) {
  const actual = Object.keys(value).sort();
  const wanted = [...expected].sort();
  if (actual.length !== wanted.length || actual.some((key, index) => key !== wanted[index])) {
    throw new Error(`${namespace} ${label} fields do not match schema 1`);
  }
}

export function decodeBase64(value: string) {
  const binary = atob(value);
  return Uint8Array.from(binary, (char) => char.charCodeAt(0));
}

export function encodeBase64(bytes: Uint8Array) {
  let binary = '';
  for (let offset = 0; offset < bytes.length; offset += 0x8000) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + 0x8000));
  }
  return btoa(binary);
}

export function bytesEqual(left: Uint8Array, right: Uint8Array) {
  return left.length === right.length && left.every((byte, index) => byte === right[index]);
}
