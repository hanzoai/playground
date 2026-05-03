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
    application: process.env.E2E_IAM_APPLICATION || 'hanzo-bot',
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
  const baseURL = process.env.E2E_BASE_URL || 'https://hanzo.bot';

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

  // Wait for Next.js hydration — the login form is a React app and event
  // handlers aren't attached until hydration completes. Without this,
  // Playwright's fill() updates the DOM but React state stays empty.
  await page.waitForLoadState('networkidle', { timeout: 30_000 }).catch(() => {});

  // The hanzo.id login form uses placeholder-based inputs with no name/id attrs.
  // Use simple, direct selectors that match the actual page structure.
  const emailInput = page.locator('input[placeholder*="email" i]').first();
  const passwordInput = page.locator('input[type="password"]').first();

  await emailInput.waitFor({ state: 'visible', timeout: 15_000 });

  // Click into the field first to ensure React focus handlers fire,
  // then clear and type (more reliable than fill() for React controlled inputs).
  await emailInput.click();
  await emailInput.fill(cfg.email);
  await passwordInput.click();
  await passwordInput.fill(cfg.password);

  // Verify the fill actually took effect in the DOM
  const filledEmail = await emailInput.inputValue();
  const filledPw = await passwordInput.inputValue();
  console.log(`[e2e] Filled credentials (email=${filledEmail ? 'set' : 'EMPTY'}, pw=${filledPw ? 'set' : 'EMPTY'})`);

  if (!filledEmail || !filledPw) {
    // Retry with keyboard typing if programmatic fill failed
    console.log(`[e2e] Retrying credential fill with keyboard input...`);
    await emailInput.click({ clickCount: 3 });
    await page.keyboard.type(cfg.email, { delay: 30 });
    await passwordInput.click({ clickCount: 3 });
    await page.keyboard.type(cfg.password, { delay: 30 });
  }

  // Submit — click Sign In button (do NOT use force:true, let Playwright
  // wait for actionability so we know the button is actually interactive)
  const submitButton = page.getByRole('button', { name: /sign in/i }).first();
  await submitButton.waitFor({ state: 'visible', timeout: 10_000 });

  // Set up navigation listener BEFORE clicking to avoid race
  const navigationPromise = page.waitForURL(
    (url) => !url.href.includes(cfg.serverUrl),
    { timeout: 60_000 },
  ).then(() => 'redirected' as const);

  await submitButton.click({ timeout: 10_000 });
  console.log(`[e2e] Clicked submit, waiting for redirect back to app...`);

  // Wait for redirect away from hanzo.id. If it doesn't happen, check for errors.
  try {
    await navigationPromise;
  } catch {
    // No redirect — check if an error is visible on the login page.
    // hanzo.id uses text-red-400/text-red-500 banners for errors, not class="error".
    const errorBanner = page.locator('.text-red-400, .text-red-500, [role="alert"], .bg-red-500\\/10').first();
    const errText = await errorBanner.textContent({ timeout: 3_000 }).catch(() => null);
    if (errText?.trim()) {
      throw new Error(`[e2e] Login failed on hanzo.id: ${errText.trim()}`);
    }
    // No error banner — page is stuck
    throw new Error(`[e2e] Login stuck — no redirect from hanzo.id after submit. URL: ${page.url()}`);
  }

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
  await page.waitForURL(/\/(launch|playground|dashboard|bots|nodes|executions|workflows|canvas|spaces|settings|metrics|identity|org)/, {
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
  const tokenRes = await fetch(`${cfg.serverUrl}/oauth/token`, {
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
