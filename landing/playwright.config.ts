import { defineConfig } from '@playwright/test';

const repositoryBase = process.env.UAP_E2E_REPOSITORY_BASE?.replace(/^\/+|\/+$/g, '') || '';
const webRoot = process.env.UAP_E2E_WEB_ROOT || '.output/public';

export default defineConfig({
  testDir: './tests/browser',
  timeout: 30_000,
  use: {
    baseURL: `http://127.0.0.1:4173/${repositoryBase ? `${repositoryBase}/` : ''}`,
    trace: 'retain-on-failure',
  },
  webServer: {
    command: `corepack pnpm@8.15.1 exec serve ${webRoot} -l tcp://127.0.0.1:4173`,
    port: 4173,
    reuseExistingServer: false,
    timeout: 60_000,
  },
});
