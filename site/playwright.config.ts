import { defineConfig } from '@playwright/test'

const baseURL = 'http://127.0.0.1:4173/universal-agent-plugins/'
const chromiumExecutablePath = process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH

export default defineConfig({
  testDir: './tests/browser',
  outputDir: './test-results',
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL,
    trace: 'retain-on-failure',
    launchOptions: chromiumExecutablePath
      ? { executablePath: chromiumExecutablePath, args: ['--no-sandbox'] }
      : undefined,
  },
  projects: [
    {
      name: 'chromium-desktop',
      use: { browserName: 'chromium', viewport: { width: 1440, height: 900 } },
    },
    {
      name: 'chromium-mobile',
      use: {
        browserName: 'chromium',
        viewport: { width: 390, height: 844 },
        deviceScaleFactor: 3,
        hasTouch: true,
        isMobile: true,
      },
    },
  ],
  webServer: {
    command: 'node scripts/serve-generated.mjs',
    url: baseURL,
    reuseExistingServer: false,
    timeout: 15_000,
  },
})
