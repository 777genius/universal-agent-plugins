import { emptyDownloadsResponse, parseGitHubRelease } from '~/utils/releaseDownloads';
import type { DownloadsApiResponse, GitHubRelease } from '~/utils/releaseDownloads';

const RELEASE_CACHE_TTL = 10 * 60 * 1000;

let cachedRelease: DownloadsApiResponse | null = null;
let cachedAt = 0;

export default defineEventHandler(async (event) => {
  const config = useRuntimeConfig(event);
  const githubRepo = config.public.githubRepo || '777genius/universal-agent-plugins';
  const token = config.github?.token;

  setHeader(event, 'cache-control', 'public, max-age=600, stale-while-revalidate=86400');

  if (cachedRelease && Date.now() - cachedAt < RELEASE_CACHE_TTL) {
    return cachedRelease;
  }

  try {
    const release = await $fetch<GitHubRelease>(
      `https://api.github.com/repos/${githubRepo}/releases/latest`,
      {
        headers: {
          Accept: 'application/vnd.github+json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
      },
    );

    const parsed = parseGitHubRelease(release);
    cachedRelease = parsed;
    cachedAt = Date.now();
    return parsed;
  } catch {
    const fallback = emptyDownloadsResponse();
    cachedRelease = fallback;
    cachedAt = Date.now();
    return fallback;
  }
});
