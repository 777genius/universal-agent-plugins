export type SignedFeedKind = 'discovery' | 'security';

export interface SignedFeedOriginOptions {
  kind: SignedFeedKind;
  baseURL: string;
  pageOrigin: string;
  registryPagesOrigin?: string;
}

export function resolveSignedFeedOrigins(options: SignedFeedOriginOptions): URL[] {
  const baked = new URL(
    `${normalizeBasePath(options.baseURL)}${options.kind}/`,
    options.pageOrigin,
  );
  const registryOrigin = normalizeOrigin(options.registryPagesOrigin);
  if (!registryOrigin) return [baked];
  const live = new URL(`${options.kind}/`, registryOrigin);
  if (live.href === baked.href) return [baked];
  return [baked, live];
}

export async function loadFirstAvailable<T>(
  origins: readonly URL[],
  load: (origin: URL) => Promise<T>,
): Promise<T> {
  if (!origins.length) throw new Error('No signed feed origins were provided');
  let lastError: unknown;
  for (const origin of origins) {
    try {
      return await load(origin);
    } catch (error) {
      lastError = error;
    }
  }
  throw lastError instanceof Error ? lastError : new Error(String(lastError));
}

function normalizeBasePath(baseURL: string): string {
  const trimmed = baseURL.trim() || '/';
  return trimmed.endsWith('/') ? trimmed : `${trimmed}/`;
}

function normalizeOrigin(value?: string): string {
  const trimmed = value?.trim();
  if (!trimmed) return '';
  try {
    const url = new URL(trimmed.endsWith('/') ? trimmed : `${trimmed}/`);
    if (url.protocol !== 'https:' && url.protocol !== 'http:') return '';
    return url.href;
  } catch {
    return '';
  }
}
