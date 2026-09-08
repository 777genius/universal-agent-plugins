import { fileURLToPath } from 'node:url';
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: '.',
  testMatch: 'candidate.spec.ts',
  timeout: 30_000,
  retries: 0,
  use: {
    baseURL: `http://127.0.0.1:4183${process.env.UAP_CANDIDATE_BASE || '/'}`,
    trace: 'retain-on-failure',
  },
  webServer: {
    cwd: fileURLToPath(new URL('.', import.meta.url)),
    command: 'node node_modules/nuxt/bin/nuxt.mjs dev --host 127.0.0.1 --port 4183',
    url: `http://127.0.0.1:4183${process.env.UAP_CANDIDATE_BASE || '/'}plugins/gitlab/`,
    reuseExistingServer: false,
    timeout: 60_000,
  },
});
