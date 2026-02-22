import { test as setup } from '@playwright/test';
import * as path from 'path';
import * as fs from 'fs';
import { fileURLToPath } from 'url';
import { ensureTestAccount, performBrowserLogin } from './helpers/iam-auth.helper';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const authFile = path.resolve(__dirname, '.auth', 'user.json');

/**
 * Auth setup project — runs once before all test projects.
 *
 * 1. Creates test account on hanzo.id (idempotent)
 * 2. Performs full PKCE browser login
 * 3. Saves session state (cookies, localStorage) to .auth/user.json
 *
 * All subsequent tests reuse this cached auth state.
 */
setup('authenticate via IAM PKCE flow', async ({ page }) => {
  // Ensure .auth directory exists
  const authDir = path.dirname(authFile);
  if (!fs.existsSync(authDir)) {
    fs.mkdirSync(authDir, { recursive: true });
  }

  // Step 1: Create test account if it doesn't exist
  await ensureTestAccount();

  // Step 2: Full browser PKCE login
  await performBrowserLogin(page);

  // Step 3: Save auth state for reuse
  await page.context().storageState({ path: authFile });
  console.log(`Auth state saved to ${authFile}`);
});
