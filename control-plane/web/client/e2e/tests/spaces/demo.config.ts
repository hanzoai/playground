import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: '.',
  testMatch: 'spaces-demo.spec.ts',
  fullyParallel: false,
  retries: 0,
  workers: 1,
  timeout: 60_000,
  outputDir: '../../../test-results',

  use: {
    baseURL: 'http://localhost:5173',
    trace: 'off',
    screenshot: 'on',
    actionTimeout: 10_000,
    navigationTimeout: 15_000,
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
