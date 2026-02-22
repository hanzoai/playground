import { defineConfig, devices } from '@playwright/test';
import * as path from 'path';
import { fileURLToPath } from 'url';
import dotenv from 'dotenv';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Load .env.e2e from client root
dotenv.config({ path: path.resolve(__dirname, '..', '.env.e2e') });

const baseURL = process.env.E2E_BASE_URL || 'https://app.hanzo.bot';

export default defineConfig({
  testDir: '.',
  testMatch: ['auth.setup.ts', 'tests/**/*.spec.ts'],
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1, // sequential — tests share production state
  reporter: process.env.CI
    ? [['html', { outputFolder: '../playwright-report' }], ['github']]
    : [['html', { outputFolder: '../playwright-report', open: 'on-failure' }]],
  outputDir: '../test-results',

  use: {
    baseURL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    actionTimeout: 15_000,
    navigationTimeout: 30_000, // OIDC cross-origin redirects are slow
  },

  timeout: 60_000, // per test

  globalSetup: path.resolve(__dirname, 'global-setup.ts'),
  globalTeardown: path.resolve(__dirname, 'global-teardown.ts'),

  projects: [
    // Auth setup: runs first, saves session to .auth/user.json
    {
      name: 'auth-setup',
      testMatch: 'auth.setup.ts',
      use: { ...devices['Desktop Chrome'] },
    },

    // Chrome — depends on auth-setup
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        storageState: path.resolve(__dirname, '.auth', 'user.json'),
      },
      dependencies: ['auth-setup'],
      testMatch: 'tests/**/*.spec.ts',
    },

    // Firefox — depends on auth-setup
    {
      name: 'firefox',
      use: {
        ...devices['Desktop Firefox'],
        storageState: path.resolve(__dirname, '.auth', 'user.json'),
      },
      dependencies: ['auth-setup'],
      testMatch: 'tests/**/*.spec.ts',
    },
  ],
});
