import { test, expect } from '../../fixtures';
import { LoginPage } from '../../page-objects/login.page';
import { extractTokenFromPage } from '../../helpers/iam-auth.helper';

/**
 * Full authentication E2E flow.
 *
 * Tests the complete journey:
 *   App loads → PKCE redirect → hanzo.id login → callback → dashboard
 *
 * Note: Auth setup already ran and cached state. These tests VERIFY the flow
 * works correctly by using the cached state and checking that the app is
 * properly authenticated.
 */

test.describe('Full Authentication Flow', () => {
  test('user is authenticated and can access dashboard', async ({ page }) => {
    // Using cached auth state from auth.setup.ts
    await page.goto('/dashboard', { waitUntil: 'networkidle' });

    // Should NOT see the auth guard — user is already logged in
    const authGuard = page.getByRole('button', { name: /sign in with hanzo/i });
    await expect(authGuard).not.toBeVisible({ timeout: 10_000 });

    // Should see dashboard content
    await expect(page).toHaveURL(/\/(dashboard)/);
  });

  test('JWT token is present in localStorage after login', async ({ page }) => {
    await page.goto('/dashboard', { waitUntil: 'networkidle' });

    const token = await extractTokenFromPage(page);
    expect(token).toBeTruthy();
    expect(token!.length).toBeGreaterThan(20);

    // JWT should have three parts (header.payload.signature)
    const parts = token!.split('.');
    expect(parts.length).toBe(3);
  });

  test('JWT claims contain user info', async ({ page }) => {
    await page.goto('/dashboard', { waitUntil: 'networkidle' });

    const token = await extractTokenFromPage(page);
    expect(token).toBeTruthy();

    // Decode JWT payload (base64)
    const payload = JSON.parse(Buffer.from(token!.split('.')[1], 'base64').toString());

    // IAM JWT should contain standard claims
    // hanzo.id JWTs use 'name' (username) and 'iss' / 'exp'
    expect(payload).toHaveProperty('iss'); // issuer (hanzo.id)
    expect(payload).toHaveProperty('exp'); // expiration
    expect(payload.name || payload.sub).toBeTruthy(); // user identifier

    // Should contain email if available in claims
    const email = process.env.E2E_IAM_USER_EMAIL;
    if (payload.email) {
      expect(payload.email).toBe(email);
    }
  });

  test('authenticated API calls work with JWT', async ({ page }) => {
    await page.goto('/dashboard', { waitUntil: 'networkidle' });

    // The app makes API calls with the JWT. Verify an API call succeeds.
    const token = await extractTokenFromPage(page);
    expect(token).toBeTruthy();

    // Call the dashboard summary API directly with the token
    const baseURL = process.env.E2E_BASE_URL || 'https://app.hanzo.bot';
    const response = await page.request.get(`${baseURL}/api/v1/dashboard/summary`, {
      headers: { Authorization: `Bearer ${token}` },
    });

    expect(response.ok()).toBeTruthy();
  });

  test('auth persists across page navigation', async ({ page }) => {
    // Navigate to multiple pages — auth should persist
    await page.goto('/dashboard', { waitUntil: 'networkidle' });
    await expect(page).toHaveURL(/\/dashboard/);

    await page.goto('/bots/all', { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(/\/bots\/all/);

    // Still no auth guard
    const authGuard = page.getByRole('button', { name: /sign in with hanzo/i });
    await expect(authGuard).not.toBeVisible({ timeout: 5_000 });

    await page.goto('/executions', { waitUntil: 'domcontentloaded' });
    await expect(page).toHaveURL(/\/executions/);
  });
});
