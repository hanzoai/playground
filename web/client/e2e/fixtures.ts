import { test as base, expect } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const tokenFile = path.resolve(__dirname, '.auth', 'iam-tokens.json');

/**
 * Custom test fixture that injects IAM session tokens into sessionStorage.
 *
 * Playwright's storageState only saves cookies + localStorage.
 * The @hanzo/iam SDK stores tokens in sessionStorage, so we save them
 * separately in auth.setup.ts and inject them here via addInitScript
 * before any page navigation occurs.
 */
export const test = base.extend({
  context: async ({ context }, use) => {
    if (fs.existsSync(tokenFile)) {
      const tokens = JSON.parse(fs.readFileSync(tokenFile, 'utf-8'));
      await context.addInitScript((tokenData: Record<string, string>) => {
        for (const [key, value] of Object.entries(tokenData)) {
          sessionStorage.setItem(key, value);
        }
      }, tokens);
    }
    await use(context);
  },
});

export { expect };
