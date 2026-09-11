import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { stripTypeScriptTypes } from 'node:module';
import { test } from 'node:test';
import { runInNewContext } from 'node:vm';
import { withAppBase } from '../utils/localizedRoutes.ts';
import { emptyDownloadsResponse, parseGitHubRelease } from '../utils/releaseDownloads.ts';

const source = readFileSync(
  new URL('../composables/useReleaseDownloads.ts', import.meta.url),
  'utf8',
)
  .replace(/^import[\s\S]*?from '[^']+';\n/gm, '')
  .replaceAll('export ', '');
const script = stripTypeScriptTypes(source) + '\nuseReleaseDownloads;';
for (const base of ['/', '/universal-agent-plugins/']) {
  test(`release handler uses ${base} with stable cache key and ten-minute TTL`, async () => {
    const storage = new Map<string, string>();
    const requests: string[] = [];
    const keys: string[] = [];
    const pending: Promise<void>[] = [];
    let now = 1_000_000;
    const release = { ...emptyDownloadsResponse(), ok: true, version: '1.2.3' };
    const useRelease = runInNewContext(script, {
      withAppBase,
      emptyDownloadsResponse,
      parseGitHubRelease,
      Date: { now: () => now },
      window: {
        sessionStorage: {
          getItem: (key: string) => storage.get(key),
          setItem: (key: string, value: string) => storage.set(key, value),
        },
      },
      useRuntimeConfig: () => ({ app: { baseURL: base }, public: {} }),
      $fetch: async (url: string, options?: { responseType?: string }) => {
        requests.push(url);
        if (url.startsWith('https://api.github.com/')) throw new Error('Blocked GitHub');
        assert.equal(url, `${base}api/releases/latest`);
        // Static hosts can serve extensionless JSON as application/octet-stream.
        return options?.responseType === 'json' ? release : JSON.stringify(release);
      },
      useAsyncData: (key: string, handler: () => Promise<unknown>, options: any) => {
        keys.push(key);
        const data = { value: options.default() };
        pending.push(
          handler().then((value) => {
            data.value = value;
          }),
        );
        return { data };
      },
    });
    const first = useRelease();
    await Promise.all(pending);
    assert.equal(first.data.value.version, '1.2.3');
    assert.deepEqual([...storage.keys()], ['plugin-kit-ai_release_meta']);
    now += 10 * 60 * 1000;
    useRelease();
    await Promise.all(pending);
    assert.equal(
      requests.filter((url) => url.startsWith('/')).length,
      1,
      'TTL boundary retains cache',
    );
    now++;
    useRelease();
    await Promise.all(pending);
    assert.equal(
      requests.filter((url) => url.startsWith('/')).length,
      2,
      'expired cache refetches',
    );
    assert.deepEqual(keys, Array(3).fill('universal-agent-plugins-releases'));
  });
}
