import { emptyDownloadsResponse, parseGitHubRelease } from '~/utils/releaseDownloads';
import type {
  DownloadArch,
  DownloadsApiResponse,
  DownloadOs,
  GitHubRelease,
} from '~/utils/releaseDownloads';
import type { Ref } from 'vue';

type ResolveResult = { url: string; version: string | null } | null;

// Keep the storage key stable across the product rename so existing browser
// sessions retain their cached release metadata and do not trigger a burst of
// duplicate API requests during rollout.
const CACHE_KEY = 'plugin-kit-ai_release_meta';
const CACHE_TTL = 10 * 60 * 1000;
let clientRefreshPromise: Promise<DownloadsApiResponse | null> | null = null;

function isClient(): boolean {
  return typeof window !== 'undefined';
}

function readCache(): DownloadsApiResponse | null {
  if (!isClient()) {
    return null;
  }

  try {
    const raw = window.sessionStorage.getItem(CACHE_KEY);
    if (!raw) {
      return null;
    }

    const parsed = JSON.parse(raw) as { ts: number; data: DownloadsApiResponse };
    if (Date.now() - parsed.ts > CACHE_TTL) {
      return null;
    }

    return parsed.data;
  } catch {
    return null;
  }
}

function writeCache(data: DownloadsApiResponse): void {
  if (!isClient()) {
    return;
  }

  try {
    window.sessionStorage.setItem(CACHE_KEY, JSON.stringify({ ts: Date.now(), data }));
  } catch {
    // Ignore unavailable session storage.
  }
}

async function fetchLatestReleaseDirect(githubRepo: string): Promise<DownloadsApiResponse | null> {
  try {
    const release = await $fetch<GitHubRelease>(
      `https://api.github.com/repos/${githubRepo}/releases/latest`,
      {
        headers: {
          Accept: 'application/vnd.github+json',
        },
      },
    );

    const parsed = parseGitHubRelease(release);
    writeCache(parsed);
    return parsed;
  } catch {
    return null;
  }
}

async function refreshLatestRelease(
  data: Ref<DownloadsApiResponse>,
  githubRepo: string,
): Promise<void> {
  if (!isClient()) {
    return;
  }

  if (!clientRefreshPromise) {
    clientRefreshPromise = fetchLatestReleaseDirect(githubRepo);
  }

  try {
    const latest = await clientRefreshPromise;
    if (latest) {
      data.value = latest;
    }
  } finally {
    clientRefreshPromise = null;
  }
}

export const useReleaseDownloads = () => {
  const config = useRuntimeConfig();
  const githubRepo = config.public.githubRepo || '777genius/universal-agent-plugins';
  const fallbackUrl =
    config.public.githubReleasesUrl || `https://github.com/${githubRepo}/releases`;

  const { data, pending, error } = useAsyncData<DownloadsApiResponse>(
    'universal-agent-plugins-releases',
    async () => {
      const cached = readCache();
      if (cached) {
        return cached;
      }

      try {
        const parsed = await $fetch<DownloadsApiResponse>('/api/releases/latest');
        writeCache(parsed);
        return parsed;
      } catch {
        return emptyDownloadsResponse();
      }
    },
    {
      server: true,
      lazy: false,
      default: () => emptyDownloadsResponse(),
    },
  );

  if (isClient()) {
    void refreshLatestRelease(data, githubRepo);
  }

  const resolve = (os: DownloadOs, arch: DownloadArch | 'unknown'): ResolveResult => {
    const api = data.value;
    if (!api.ok) {
      return null;
    }

    if (os === 'windows') {
      const variant = api.variants.windows.x64;
      return variant.url ? { url: variant.url, version: variant.version || api.version } : null;
    }

    if (os === 'linux') {
      const variant = api.variants.linux.appimage.url
        ? api.variants.linux.appimage
        : api.variants.linux.deb;
      return variant.url ? { url: variant.url, version: variant.version || api.version } : null;
    }

    if (os === 'macos') {
      const universal = api.variants.macos.universal;
      if (universal.url) {
        return { url: universal.url, version: universal.version || api.version };
      }

      const byArch = arch === 'arm64' ? api.variants.macos.arm64 : api.variants.macos.x64;
      if (byArch.url) {
        return { url: byArch.url, version: byArch.version || api.version };
      }

      const any = api.variants.macos.arm64.url ? api.variants.macos.arm64 : api.variants.macos.x64;
      return any.url ? { url: any.url, version: any.version || api.version } : null;
    }

    return null;
  };

  const resolveUrlOrFallback = (os: DownloadOs, arch: DownloadArch | 'unknown'): string =>
    resolve(os, arch)?.url || fallbackUrl;

  return { data, pending, error, fallbackUrl, resolve, resolveUrlOrFallback };
};
