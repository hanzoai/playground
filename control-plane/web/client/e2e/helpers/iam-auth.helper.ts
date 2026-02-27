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
  application?: string;  // defaults to 'app-hanzobot'
}

function getConfig(): IamConfig {
  return {
    serverUrl: process.env.E2E_IAM_SERVER_URL || 'https://hanzo.id',
    clientId: process.env.E2E_IAM_CLIENT_ID || '',
    email: process.env.E2E_IAM_USER_EMAIL || '',
    password: process.env.E2E_IAM_USER_PASSWORD || '',
    organization: process.env.E2E_IAM_ORGANIZATION || 'hanzo',
    application: process.env.E2E_IAM_APPLICATION || 'app-hanzobot',
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

  // Navigate to app — AuthGuard may show a button or auto-redirect to hanzo.id
  console.log(`[e2e] Navigating to ${baseURL}`);
  await page.goto(baseURL, { waitUntil: 'domcontentloaded', timeout: 60_000 });
  console.log(`[e2e] Loaded: ${page.url()}`);

  // Wait for either: sign-in button appears, or auto-redirect to hanzo.id
  const signInButton = page.getByRole('button', { name: /sign in with hanzo/i });
  const onHanzoId = page.waitForURL(`${cfg.serverUrl}/**`, { timeout: 30_000 });
  const onButton = signInButton.waitFor({ state: 'visible', timeout: 30_000 }).then(() => 'button' as const);

  const landedOn = await Promise.race([
    onHanzoId.then(() => 'hanzo-id' as const),
    onButton,
  ]);

  if (landedOn === 'button') {
    await signInButton.click();
    console.log(`[e2e] Clicked sign-in, waiting for hanzo.id redirect...`);
    await page.waitForURL(`${cfg.serverUrl}/**`, { timeout: 30_000 });
  }
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

  // Submit — click Sign In button
  const submitButton = page.getByRole('button', { name: /sign in/i }).first();
  await submitButton.waitFor({ state: 'visible', timeout: 10_000 });
  await submitButton.click({ force: true, timeout: 10_000 });
  console.log(`[e2e] Clicked submit, waiting for redirect back to app...`);

  // Wait for redirect back to app (login form submits and OIDC redirects)
  await page.waitForURL(`${baseURL}/**`, { timeout: 60_000 });

  console.log(`[e2e] Back on app: ${page.url().substring(0, 80)}...`);

  // Wait for app to load after auth callback
  await page.waitForLoadState('networkidle', { timeout: 30_000 }).catch(() => {});

  // Handle preferences onboarding gate if it appears
  const continueButton = page.getByRole('button', { name: /continue/i });
  try {
    await continueButton.waitFor({ state: 'visible', timeout: 10_000 });
    console.log(`[e2e] Preferences onboarding shown — clicking Continue`);
    await continueButton.click();
  } catch {
    // No onboarding — already completed or not present
  }

  // Wait for callback to process tokens and redirect to app
  await page.waitForURL(/\/(launch|playground|dashboard|bots|nodes|executions|workflows|canvas|spaces|settings|metrics|identity)/, {
    timeout: 60_000,
  });

  console.log(`[e2e] Login complete — landed on: ${page.url()}`);
}

/**
 * Get a JWT access token for API calls.
 * First tries reading from the saved auth state file (populated by auth.setup.ts).
 * Falls back to the IAM client credentials endpoint.
 */
export async function getAccessToken(): Promise<string> {
  // Try reading from saved auth tokens file (written by auth.setup.ts)
  try {
    const fs = await import('fs');
    const path = await import('path');
    const { fileURLToPath } = await import('url');
    const __dirname = path.dirname(fileURLToPath(import.meta.url));
    const tokensPath = path.resolve(__dirname, '..', '.auth', 'iam-tokens.json');
    if (fs.existsSync(tokensPath)) {
      const tokens = JSON.parse(fs.readFileSync(tokensPath, 'utf-8'));
      const token = tokens.hanzo_iam_access_token || tokens.accessToken;
      if (token) {
        return token;
      }
    }
  } catch {
    // File not available — continue to API fallback
  }

  // Fallback: try client_credentials grant
  const cfg = getConfig();
  const tokenRes = await fetch(`${cfg.serverUrl}/api/login/oauth/access_token`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({
      grant_type: 'client_credentials',
      client_id: cfg.clientId,
      client_secret: '',
    }).toString(),
  });

  if (!tokenRes.ok) {
    const err = await tokenRes.text();
    throw new Error(`Failed to get access token: ${tokenRes.status} ${err}`);
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
