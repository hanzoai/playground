import { type Page, expect } from '@playwright/test';

/**
 * IAM authentication helpers for E2E tests.
 *
 * Handles:
 *  - Account creation via hanzo.id /api/signup
 *  - Full PKCE browser login flow (cross-origin OIDC)
 *  - Token extraction from callback
 */

interface IamConfig {
  serverUrl: string;     // https://hanzo.id
  clientId: string;      // hanzobot-client-id
  email: string;
  password: string;
  organization?: string; // defaults to 'hanzo'
  application?: string;  // defaults to 'app-hanzo'
}

function getConfig(): IamConfig {
  return {
    serverUrl: process.env.E2E_IAM_SERVER_URL || 'https://hanzo.id',
    clientId: process.env.E2E_IAM_CLIENT_ID || '',
    email: process.env.E2E_IAM_USER_EMAIL || '',
    password: process.env.E2E_IAM_USER_PASSWORD || '',
    organization: process.env.E2E_IAM_ORGANIZATION || 'hanzo',
    application: process.env.E2E_IAM_APPLICATION || 'app-hanzo',
  };
}

/**
 * Create a test account on hanzo.id via the Casdoor signup API.
 * Idempotent — if account already exists, login is attempted to verify.
 */
export async function ensureTestAccount(): Promise<{ created: boolean }> {
  const cfg = getConfig();
  const username = cfg.email.split('@')[0];

  // Try to create the account
  const signupRes = await fetch(`${cfg.serverUrl}/api/signup`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      application: cfg.application,
      organization: cfg.organization,
      username,
      name: `E2E Test User`,
      email: cfg.email,
      password: cfg.password,
      type: 'normal-user',
    }),
  });

  const body = await signupRes.json().catch(() => ({}));

  // "user already exists" is fine — account was already created
  if (signupRes.ok || body?.status === 'ok') {
    console.log(`Account created or already exists: ${cfg.email}`);
    return { created: true };
  }

  const errMsg = body?.msg || body?.message || JSON.stringify(body);
  if (
    errMsg.includes('already exists') ||
    errMsg.includes('duplicate') ||
    errMsg.includes('username is already')
  ) {
    console.log(`Account already exists: ${cfg.email}`);
    return { created: false };
  }

  // Unexpected error — but don't fail, login test will catch it
  console.warn(`Signup response (${signupRes.status}): ${errMsg}`);
  return { created: false };
}

/**
 * Perform full PKCE login flow through the browser.
 *
 * 1. Navigate to app → AuthGuard renders
 * 2. Click "Sign in with Hanzo" → redirects to hanzo.id
 * 3. Fill credentials on hanzo.id Casdoor form
 * 4. hanzo.id redirects to /auth/callback?code=&state=
 * 5. App exchanges code for tokens
 * 6. Dashboard becomes visible
 */
export async function performBrowserLogin(page: Page): Promise<void> {
  const cfg = getConfig();
  const baseURL = process.env.E2E_BASE_URL || 'https://app.hanzo.bot';

  // Capture browser console for diagnostics
  page.on('console', (msg) => {
    if (msg.type() === 'error' || msg.type() === 'warn') {
      console.log(`[browser ${msg.type()}] ${msg.text()}`);
    }
  });
  page.on('pageerror', (err) => console.log(`[browser exception] ${err.message}`));

  // Navigate to app — should see AuthGuard
  console.log(`[e2e] Navigating to ${baseURL}`);
  await page.goto(baseURL, { waitUntil: 'networkidle' });
  console.log(`[e2e] Loaded: ${page.url()}`);

  // Click "Sign in with Hanzo" — triggers OIDC redirect to hanzo.id
  const signInButton = page.getByRole('button', { name: /sign in with hanzo/i });
  await expect(signInButton).toBeVisible({ timeout: 15_000 });
  await signInButton.click();
  console.log(`[e2e] Clicked sign-in, waiting for hanzo.id redirect...`);

  // Wait for redirect to hanzo.id login page
  await page.waitForURL(`${cfg.serverUrl}/**`, { timeout: 30_000 });
  console.log(`[e2e] On hanzo.id: ${page.url()}`);

  // Fill the login form
  const emailInput =
    page.locator('input[name="username"]').or(
    page.locator('input[name="email"]')).or(
    page.locator('input[type="email"]')).or(
    page.locator('input[placeholder*="email" i]')).or(
    page.locator('input[placeholder*="username" i]'));

  const passwordInput =
    page.locator('input[name="password"]').or(
    page.locator('input[type="password"]'));

  await emailInput.first().fill(cfg.email, { timeout: 15_000 });
  await passwordInput.first().fill(cfg.password);
  console.log(`[e2e] Filled credentials`);

  // Submit
  const submitButton =
    page.getByRole('button', { name: /sign in|log in|login|submit/i }).or(
    page.locator('button[type="submit"]'));

  await submitButton.first().click();
  console.log(`[e2e] Clicked submit, waiting for redirect back to app...`);

  // Wait for redirect back to app callback
  await page.waitForURL(`${baseURL}/**`, { timeout: 30_000 });
  console.log(`[e2e] Back on app: ${page.url().substring(0, 80)}...`);

  // Wait for callback to process tokens and redirect to dashboard
  await page.waitForURL(/\/(playground|dashboard|bots|nodes|executions|workflows|canvas|spaces)/, {
    timeout: 60_000,
  });

  console.log(`[e2e] Login complete — landed on: ${page.url()}`);
}

/**
 * Get a JWT access token via the Casdoor password grant (for API calls).
 * Used by helpers that need to call Commerce API directly.
 */
export async function getAccessToken(): Promise<string> {
  const cfg = getConfig();

  const tokenRes = await fetch(`${cfg.serverUrl}/api/login/oauth/access_token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      grant_type: 'password',
      client_id: cfg.clientId,
      username: `${cfg.organization}/${cfg.email}`,
      password: cfg.password,
    }),
  });

  if (!tokenRes.ok) {
    // Fallback: try with just username
    const fallbackRes = await fetch(`${cfg.serverUrl}/api/login/oauth/access_token`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        grant_type: 'password',
        client_id: cfg.clientId,
        username: cfg.email,
        password: cfg.password,
      }),
    });

    if (!fallbackRes.ok) {
      const err = await fallbackRes.text();
      throw new Error(`Failed to get access token: ${fallbackRes.status} ${err}`);
    }

    const data = await fallbackRes.json();
    return data.access_token;
  }

  const data = await tokenRes.json();
  return data.access_token;
}

/**
 * Extract access token from browser storage after login.
 * IAM SDK stores tokens in sessionStorage with 'hanzo_iam_' prefix.
 */
export async function extractTokenFromPage(page: Page): Promise<string | null> {
  return page.evaluate(() => {
    return sessionStorage.getItem('hanzo_iam_access_token')
      || localStorage.getItem('af_iam_token');
  });
}
