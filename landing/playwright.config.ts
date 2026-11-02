import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';

const repositoryBase = process.env.UAP_E2E_REPOSITORY_BASE?.replace(/^\/+|\/+$/g, '') || '';
const serverConfig = fileURLToPath(new URL('./tests/browser/serve.json', import.meta.url));
const quotedServerConfig = "'" + serverConfig.replaceAll("'", "'\\''") + "'";
const webRoot = process.env.UAP_E2E_WEB_ROOT || '.output/public';

export default defineConfig({
  testDir: './tests/browser',
  timeout: 30_000,
  use: {
    baseURL: `http://127.0.0.1:4173/${repositoryBase ? `${repositoryBase}/` : ''}`,
    // Real Mux HLS playback needs H.264 codecs; Playwright's headless shell
    // cannot reliably qualify video decoding. Hosted CI supplies Chrome.
    channel: process.env.UAP_E2E_LIVE_VIDEO === '1' ? 'chrome' : undefined,
    trace: 'retain-on-failure',
  },
  webServer: {
    command: `corepack pnpm@8.15.1 exec serve ${webRoot} -c ${quotedServerConfig} -l tcp://127.0.0.1:4173`,
    port: 4173,
    reuseExistingServer: false,
    timeout: 60_000,
  },
});
